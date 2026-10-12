package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"cfu-board/pkg/database"
	"cfu-board/pkg/repository"
)

// likeResponse — JSON-ответ операций с лайком комментария: актуальное
// количество лайков после выполненного действия.
type likeResponse struct {
	LikesCount int `json:"likes_count"`
}

// postLikeResponse — JSON-ответ операций с лайком поста: актуальное
// количество лайков и просмотров после выполненного действия.
// Новый лайк засчитывает просмотр (дедупликация по IP), повторный — нет.
type postLikeResponse struct {
	LikesCount int `json:"likes_count"`
	ViewsCount int `json:"views_count"`
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
// запрос не создаёт дубликат и не является ошибкой. Первый (новый) лайк с IP
// засчитывает просмотр поста (дедупликация по IP, best-effort); повторный лайк
// счётчик просмотров не трогает. Ответ — {likes_count, views_count}.
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
		post, err := userRepo.GetPostByID(dbCtx, postID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
				return
			}
			slog.Error("не удалось проверить пост перед лайком", "post_id", postID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при обработке лайка")
			return
		}

		count, added, err := userRepo.AddLike(dbCtx, postID, userID)
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

		// Новый лайк = человек видел пост: засчитываем просмотр с его IP
		// (дедупликация по IP — повторный визит уже учтён через GET).
		// Повторный лайк просмотр не накручивает — отдаём текущее значение.
		views := post.ViewsCount
		if added {
			if v, verr := userRepo.IncrementPostViews(dbCtx, postID, clientIP(r)); verr != nil {
				slog.Warn("не удалось засчитать просмотр за лайк", "post_id", postID, "error", verr)
			} else if v >= 0 {
				views = v
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(postLikeResponse{LikesCount: count, ViewsCount: views})
	}
}

// UnlikePostHandler обрабатывает снятие лайка с публикации
// (DELETE /posts/{id}/likes). Требует авторизацию. Снятие идемпотентно: лайка
// и не было — просто возвращается текущий счётчик. Просмотр не засчитывает
// (человек пост уже видел). Ответ — {likes_count, views_count}.
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

		post, err := userRepo.GetPostByID(dbCtx, postID)
		if err != nil {
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
		json.NewEncoder(w).Encode(postLikeResponse{LikesCount: count, ViewsCount: post.ViewsCount})
	}
}

// clientIP извлекает реальный IP клиента: сначала X-Forwarded-For
// (первый адрес — исходный клиент за прокси/CDN), затем X-Real-IP,
// иначе — хост из RemoteAddr. Возвращает "" если определить не удалось.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, _ := strings.Cut(xff, ","); strings.TrimSpace(first) != "" {
			return strings.TrimSpace(first)
		}
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}

// GetPostHandler обрабатывает публичный просмотр публикации (GET /posts/{id}).
// Публичный роут: без токена пост доступен, liked_by_me всегда false; при
// валидном токене (OptionalAuthMiddleware) поле liked_by_me персонализируется.
// Просмотр засчитывается один раз с одного IP (таблица post_views, best-effort:
// ошибка/неизвестный IP не роняет ответ — пост отдаётся со старым счётчиком).
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

		// Инкремент просмотров с дедупликацией по IP — после проверки видимости,
		// чтобы скрытые посты (404 для чужих) счётчик не накручивали.
		// views < 0 — маркер «IP не определён», счётчик не трогаем.
		if views, err := userRepo.IncrementPostViews(dbCtx, postID, clientIP(r)); err != nil {
			slog.Warn("не удалось засчитать просмотр", "post_id", postID, "error", err)
		} else if views >= 0 {
			post.ViewsCount = views
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(post)
	}
}
