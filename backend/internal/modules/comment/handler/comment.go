package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"cfu-board/internal/models"
	"cfu-board/internal/modules/comment/repository"
	postrepo "cfu-board/internal/modules/post/repository"
	"cfu-board/internal/platform/database"
	"cfu-board/internal/platform/middleware"

)

const (
	minCommentLength = 1    // минимальная длина комментария (после trim)
	maxCommentLength = 2000 // максимальная длина комментария в символах

	defaultCommentsLimit = 20  // по умолчанию отдаём 20 комментариев
	maxCommentsLimit     = 100 // жёсткий потолок limit на запрос
)

// CreateCommentInput — входные данные создания комментария (JSON).
// ParentID == nil — корневой комментарий; задан — ответ на корневой комментарий
// того же поста (см. openapi-схему CommentInput, minimum: 1).
type CreateCommentInput struct {
	Content  string `json:"content"`
	ParentID *int   `json:"parent_id,omitempty"`
}

// parsePathID извлекает и валидирует положительный числовой ID из именованного
// значения пути запроса. Возвращает 0 и false, если значение отсутствует или
// невалидно (не число или <= 0).
func parsePathID(r *http.Request, name string) (int, bool) {
	id, err := strconv.Atoi(strings.TrimSpace(r.PathValue(name)))
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// ParsePostIDPathValue извлекает и валидирует числовой ID поста из пути запроса.
// Возвращает 0 и false, если значение отсутствует или невалидно.
func ParsePostIDPathValue(r *http.Request) (int, bool) {
	return parsePathID(r, "id")
}

// ParseQueryInt разбирает целочисленный query-параметр name. Если параметр
// отсутствует или пуст — возвращает значение по умолчанию def без ошибки;
// иначе — результат strconv.Atoi (ошибка для нечисловых значений).
func ParseQueryInt(r *http.Request, name string, def int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return def, nil
	}
	return strconv.Atoi(raw)
}

// CreateCommentHandler обрабатывает создание комментария (POST /posts/{id}/comments).
// Требует авторизацию: автор берётся из JWT-контекста. Перед вставкой проверяется
// существование поста (404), после — отдаётся созданный комментарий с данными автора.
func CreateCommentHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			middleware.RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		postID, ok := ParsePostIDPathValue(r)
		if !ok {
			middleware.RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор публикации")
			return
		}

		authorID, ok := r.Context().Value(middleware.UserIDKey).(int)
		if !ok {
			middleware.RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
			return
		}

		// Ограничиваем размер тела: комментарий — лёгкий JSON, мегабайты не нужны.
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10) // 64 KB

		var input CreateCommentInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			middleware.RespondWithError(w, http.StatusBadRequest, "Некорректный формат JSON или превышен лимит 64KB")
			return
		}

		content := strings.TrimSpace(input.Content)
		length := utf8.RuneCountInString(content)
		if length < minCommentLength {
			middleware.RespondWithError(w, http.StatusBadRequest, "Комментарий не может быть пустым")
			return
		}
		if length > maxCommentLength {
			middleware.RespondWithError(w, http.StatusBadRequest, "Комментарий слишком длинный (максимум 2000 символов)")
			return
		}
		// Ответ на комментарий: parent_id обязан быть положительным ID.
		// Проверка чисто входная — отклоняем до любого обращения к БД.
		// (Нечисловые значения parent_id отсекаются ещё на этапе декодирования JSON.)
		if input.ParentID != nil && *input.ParentID <= 0 {
			middleware.RespondWithError(w, http.StatusBadRequest, "Параметр parent_id: целое число от 1")
			return
		}

		userRepo := repository.NewRepository(database.GlobalPool)
		dbCtx, cancel := context.WithTimeout(r.Context(), middleware.DbOpTimeout)
		defer cancel()

		if _, ok := middleware.LoadVisiblePost(w, postrepo.NewRepository(database.GlobalPool), dbCtx, postID, authorID); !ok {
			return
		}

		// Ответ создаётся только на существующий корневой комментарий этого же
		// поста (дерево глубины 2). Проверяем родителя до вставки, чтобы отдать
		// понятный 400 вместо «молчаливой» ошибки FK/CTE; финальную проверку
		// атомарно выполняет сам SQL (защита от гонки «родителя удалили»).
		if input.ParentID != nil {
			parent, err := userRepo.GetCommentByID(dbCtx, *input.ParentID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					middleware.RespondWithError(w, http.StatusBadRequest, "Параметр parent_id: комментарий-родитель не найден")
					return
				}
				slog.Error("не удалось проверить комментарий-родитель", "parent_id", *input.ParentID, "error", err)
				middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при создании комментария")
				return
			}
			if parent.PostID != postID || parent.ParentID != nil {
				middleware.RespondWithError(w, http.StatusBadRequest, "Параметр parent_id: ответить можно только на корневой комментарий этого поста")
				return
			}
		}

		comment, err := userRepo.CreateCommentWithAuthor(dbCtx, postID, authorID, content, input.ParentID)
		if err != nil {
			// Гонка «пост/автор/родитель исчезли между проверкой и вставкой»:
			// INSERT ... SELECT ... WHERE даёт ErrNoRows, а прямое нарушение
			// FK — pgcode 23503. По спеке оба случая — 404. Для ответа уточняем,
			// что именно пропало: перепроверяем пост, затем родителя.
			if errors.Is(err, pgx.ErrNoRows) || middleware.IsForeignKeyViolation(err) {
				if _, perr := postrepo.NewRepository(database.GlobalPool).GetPostByID(dbCtx, postID); perr != nil {
					if errors.Is(perr, pgx.ErrNoRows) {
						middleware.RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
						return
					}
					slog.Error("не удалось перепроверить пост при создании комментария", "post_id", postID, "error", perr)
					middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при создании комментария")
					return
				}
				if input.ParentID != nil {
					middleware.RespondWithError(w, http.StatusNotFound, "Комментарий-родитель не найден")
					return
				}
				middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при создании комментария")
				slog.Error("создание корневого комментария не удалось при существующем посте", "post_id", postID, "author_id", authorID, "error", err)
				return
			}
			slog.Error("не удалось создать комментарий", "post_id", postID, "author_id", authorID, "error", err)
			middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при создании комментария")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(comment)
	}
}

// ListCommentsHandler обрабатывает выдачу комментариев поста
// (GET /posts/{id}/comments?limit=&offset=). Публичный эндпоинт: комментарии
// видны всем, liked_by_me персонализируется при валидном токене
// (OptionalAuthMiddleware). Пост обязан существовать (404), иначе — пустой
// список не отдаём.
func ListCommentsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			middleware.RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		postID, ok := ParsePostIDPathValue(r)
		if !ok {
			middleware.RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор публикации")
			return
		}

		limit, err := ParseQueryInt(r, "limit", defaultCommentsLimit)
		if err != nil || limit < 1 || limit > maxCommentsLimit {
			middleware.RespondWithError(w, http.StatusBadRequest, "Параметр limit: целое число от 1 до 100")
			return
		}
		offset, err := ParseQueryInt(r, "offset", 0)
		if err != nil || offset < 0 {
			middleware.RespondWithError(w, http.StatusBadRequest, "Параметр offset: целое неотрицательное число")
			return
		}

		// Для анонимов UserID в контексте нет — GetCommentsByPostID трактует 0
		// как «никого не смотрим», liked_by_me=false.
		viewerID, _ := r.Context().Value(middleware.UserIDKey).(int)

		userRepo := repository.NewRepository(database.GlobalPool)
		dbCtx, cancel := context.WithTimeout(r.Context(), middleware.DbOpTimeout)
		defer cancel()

		if _, ok := middleware.LoadVisiblePost(w, postrepo.NewRepository(database.GlobalPool), dbCtx, postID, viewerID); !ok {
			return
		}

		comments, err := userRepo.GetCommentsByPostID(dbCtx, postID, limit, offset, viewerID)
		if err != nil {
			slog.Error("не удалось загрузить комментарии", "post_id", postID, "error", err)
			middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке комментариев")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(comments)
	}
}

// LikeCommentHandler обрабатывает постановку лайка на комментарий
// (POST /posts/{id}/comments/{commentID}/likes). Требует авторизацию.
// Комментарий обязан принадлежать указанному посту (404), иначе — чужой ресурс
// в контексте этого пути. Лайк идемпотентен. Ответ — {likes_count}.
func LikeCommentHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			middleware.RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		postID, ok := ParsePostIDPathValue(r)
		if !ok {
			middleware.RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор публикации")
			return
		}

		commentID, ok := parsePathID(r, "commentID")
		if !ok {
			middleware.RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор комментария")
			return
		}

		userID, ok := r.Context().Value(middleware.UserIDKey).(int)
		if !ok {
			middleware.RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
			return
		}

		userRepo := repository.NewRepository(database.GlobalPool)
		dbCtx, cancel := context.WithTimeout(r.Context(), middleware.DbOpTimeout)
		defer cancel()

		if _, ok := middleware.LoadVisiblePost(w, postrepo.NewRepository(database.GlobalPool), dbCtx, postID, userID); !ok {
			return
		}

		comment, err := userRepo.GetCommentByID(dbCtx, commentID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				middleware.RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
				return
			}
			slog.Error("не удалось проверить комментарий перед лайком", "comment_id", commentID, "error", err)
			middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при обработке лайка")
			return
		}
		if comment.PostID != postID {
			middleware.RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
			return
		}

		count, err := userRepo.AddCommentLike(dbCtx, commentID, userID)
		if err != nil {
			// Комментарий могли удалить между проверкой и вставкой — это 404.
			if middleware.IsForeignKeyViolation(err) {
				middleware.RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
				return
			}
			slog.Error("не удалось поставить лайк комментарию", "comment_id", commentID, "user_id", userID, "error", err)
			middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при обработке лайка")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(models.LikeResponse{LikesCount: count})
	}
}

// UnlikeCommentHandler обрабатывает снятие лайка с комментария
// (DELETE /posts/{id}/comments/{commentID}/likes). Требует авторизацию.
// Снятие идемпотентно. Ответ — {likes_count}.
func UnlikeCommentHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			middleware.RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		postID, ok := ParsePostIDPathValue(r)
		if !ok {
			middleware.RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор публикации")
			return
		}

		commentID, ok := parsePathID(r, "commentID")
		if !ok {
			middleware.RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор комментария")
			return
		}

		userID, ok := r.Context().Value(middleware.UserIDKey).(int)
		if !ok {
			middleware.RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
			return
		}

		userRepo := repository.NewRepository(database.GlobalPool)
		dbCtx, cancel := context.WithTimeout(r.Context(), middleware.DbOpTimeout)
		defer cancel()

		if _, ok := middleware.LoadVisiblePost(w, postrepo.NewRepository(database.GlobalPool), dbCtx, postID, userID); !ok {
			return
		}

		comment, err := userRepo.GetCommentByID(dbCtx, commentID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				middleware.RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
				return
			}
			slog.Error("не удалось проверить комментарий перед снятием лайка", "comment_id", commentID, "error", err)
			middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при обработке лайка")
			return
		}
		if comment.PostID != postID {
			middleware.RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
			return
		}

		count, err := userRepo.RemoveCommentLike(dbCtx, commentID, userID)
		if err != nil {
			slog.Error("не удалось снять лайк комментария", "comment_id", commentID, "user_id", userID, "error", err)
			middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при обработке лайка")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(models.LikeResponse{LikesCount: count})
	}
}

// DeleteCommentHandler обрабатывает удаление комментария
// (DELETE /posts/{id}/comments/{commentID}). Требует авторизацию: удалить может
// автор комментария либо автор поста, под которым тот оставлен (403 иначе).
// Комментарий обязан принадлежать указанному посту (404), иначе — это чужой
// ресурс в контексте этого пути. Условие прав дублируется в самом SQL (атомарная
// защита от гонок). Ответ — {status, message} по схеме Success.
func DeleteCommentHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			middleware.RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		postID, ok := ParsePostIDPathValue(r)
		if !ok {
			middleware.RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор публикации")
			return
		}

		commentID, ok := parsePathID(r, "commentID")
		if !ok {
			middleware.RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор комментария")
			return
		}

		actorID, ok := r.Context().Value(middleware.UserIDKey).(int)
		if !ok {
			middleware.RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
			return
		}

		userRepo := repository.NewRepository(database.GlobalPool)
		dbCtx, cancel := context.WithTimeout(r.Context(), middleware.DbOpTimeout)
		defer cancel()

		// Грузим пост: чтобы отличить «не существует» (404) от «чужой» (403)
		// — автор поста тоже имеет право удалять комментарии под ним.
		post, ok := middleware.LoadVisiblePost(w, postrepo.NewRepository(database.GlobalPool), dbCtx, postID, actorID)
		if !ok {
			return
		}

		comment, err := userRepo.GetCommentByID(dbCtx, commentID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				middleware.RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
				return
			}
			slog.Error("не удалось загрузить комментарий для удаления", "comment_id", commentID, "error", err)
			middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при удалении комментария")
			return
		}
		// Комментарий другого поста в контексте этого пути не существует.
		if comment.PostID != postID {
			middleware.RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
			return
		}

		if comment.AuthorID != actorID && post.AuthorID != actorID {
			middleware.RespondWithError(w, http.StatusForbidden, "Удалить комментарий может только его автор или автор публикации")
			return
		}

		// Права проверяются ещё раз прямо в SQL — защита от гонок.
		deleted, err := userRepo.DeleteCommentByID(dbCtx, commentID, actorID)
		if err != nil {
			slog.Error("не удалось удалить комментарий из БД", "comment_id", commentID, "actor_id", actorID, "error", err)
			middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при удалении комментария")
			return
		}
		if !deleted {
			// Между чтением и удалением комментарий исчез (удалили конкуренты).
			middleware.RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"success","message":"Комментарий успешно удалён"}`)
	}
}
