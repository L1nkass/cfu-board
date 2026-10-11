package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"cfu-board/pkg/database"
	"cfu-board/pkg/repository"
)

// likeResponse — JSON-ответ операций с лайком: актуальное количество лайков
// поста после выполненного действия.
type likeResponse struct {
	LikesCount int `json:"likes_count"`
}

// isForeignKeyViolation — проверяет, что ошибка БД это нарушение внешнего ключа
// (pgcode 23503). В контексте лайков это означает, что пост был удалён между
// проверкой существования и вставкой лайка — трактуем как 404.
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// likeRepoAndCtx собирает репозиторий и контекст с таймаутом для операций лайка.
func likeRepoAndCtx(r *http.Request) (*repository.UserRepository, context.Context, context.CancelFunc) {
	userRepo := repository.NewUserRepository(database.GlobalPool)
	dbCtx, cancel := context.WithTimeout(r.Context(), dbOpTimeout)
	return userRepo, dbCtx, cancel
}

// LikePostHandler обрабатывает постановку лайка на публикацию
// (POST /posts/{id}/likes). Требует авторизацию. Лайк идемпотентен: повторный
// запрос не создаёт дубликат и не является ошибкой. Ответ — {likes_count}.
func LikePostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		postID, ok := parsePostIDPathValue(r)
		if !ok {
			RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор публикации")
			return
		}

		userID, ok := r.Context().Value(UserIDKey).(int)
		if !ok {
			RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
			return
		}

		userRepo, dbCtx, cancel := likeRepoAndCtx(r)
		defer cancel()

		// Пост должен существовать — так сразу отдаём понятный 404, а не
		// нарушение FK внутри INSERT.
		if _, err := userRepo.GetPostByID(dbCtx, postID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
				return
			}
			slog.Error("не удалось проверить пост перед лайком", "post_id", postID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при обработке лайка")
			return
		}

		count, err := userRepo.AddLike(dbCtx, postID, userID)
		if err != nil {
			// Пост могли удалить между проверкой и вставкой — это тоже 404.
			if isForeignKeyViolation(err) {
				RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
				return
			}
			slog.Error("не удалось поставить лайк", "post_id", postID, "user_id", userID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при обработке лайка")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(likeResponse{LikesCount: count})
	}
}

// UnlikePostHandler обрабатывает снятие лайка с публикации
// (DELETE /posts/{id}/likes). Требует авторизацию. Снятие идемпотентно: лайка
// и не было — просто возвращается текущий счётчик. Ответ — {likes_count}.
func UnlikePostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		postID, ok := parsePostIDPathValue(r)
		if !ok {
			RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор публикации")
			return
		}

		userID, ok := r.Context().Value(UserIDKey).(int)
		if !ok {
			RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
			return
		}

		userRepo, dbCtx, cancel := likeRepoAndCtx(r)
		defer cancel()

		if _, err := userRepo.GetPostByID(dbCtx, postID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
				return
			}
			slog.Error("не удалось проверить пост перед снятием лайка", "post_id", postID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при обработке лайка")
			return
		}

		count, err := userRepo.RemoveLike(dbCtx, postID, userID)
		if err != nil {
			slog.Error("не удалось снять лайк", "post_id", postID, "user_id", userID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при обработке лайка")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(likeResponse{LikesCount: count})
	}
}

// GetPostHandler обрабатывает публичный просмотр публикации (GET /posts/{id}).
// Публичный роут: без токена пост доступен, liked_by_me всегда false; при
// валидном токене (OptionalAuthMiddleware) поле liked_by_me персонализируется.
func GetPostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		postID, ok := parsePostIDPathValue(r)
		if !ok {
			RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор публикации")
			return
		}

		// Для анонимов UserID в контексте нет — GetPostWithLikes трактует 0
		// как «никого не смотрим», liked_by_me=false.
		viewerID, _ := r.Context().Value(UserIDKey).(int)

		userRepo, dbCtx, cancel := likeRepoAndCtx(r)
		defer cancel()

		post, err := userRepo.GetPostWithLikes(dbCtx, postID, viewerID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
				return
			}
			slog.Error("не удалось загрузить пост", "post_id", postID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке публикации")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(post)
	}
}
