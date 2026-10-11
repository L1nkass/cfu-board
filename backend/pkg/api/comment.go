package api

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

	"cfu-board/pkg/database"
	"cfu-board/pkg/repository"
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

// parsePostIDPathValue извлекает и валидирует числовой ID поста из пути запроса.
// Возвращает 0 и false, если значение отсутствует или невалидно.
func parsePostIDPathValue(r *http.Request) (int, bool) {
	return parsePathID(r, "id")
}

// parseQueryInt разбирает целочисленный query-параметр name. Если параметр
// отсутствует или пуст — возвращает значение по умолчанию def без ошибки;
// иначе — результат strconv.Atoi (ошибка для нечисловых значений).
func parseQueryInt(r *http.Request, name string, def int) (int, error) {
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
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		postID, ok := parsePostIDPathValue(r)
		if !ok {
			RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор публикации")
			return
		}

		authorID, ok := r.Context().Value(UserIDKey).(int)
		if !ok {
			RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
			return
		}

		// Ограничиваем размер тела: комментарий — лёгкий JSON, мегабайты не нужны.
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10) // 64 KB

		var input CreateCommentInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Некорректный формат JSON или превышен лимит 64KB")
			return
		}

		content := strings.TrimSpace(input.Content)
		length := utf8.RuneCountInString(content)
		if length < minCommentLength {
			RespondWithError(w, http.StatusBadRequest, "Комментарий не может быть пустым")
			return
		}
		if length > maxCommentLength {
			RespondWithError(w, http.StatusBadRequest, "Комментарий слишком длинный (максимум 2000 символов)")
			return
		}
		// Ответ на комментарий: parent_id обязан быть положительным ID.
		// Проверка чисто входная — отклоняем до любого обращения к БД.
		// (Нечисловые значения parent_id отсекаются ещё на этапе декодирования JSON.)
		if input.ParentID != nil && *input.ParentID <= 0 {
			RespondWithError(w, http.StatusBadRequest, "Параметр parent_id: целое число от 1")
			return
		}

		userRepo := repository.NewUserRepository(database.GlobalPool)
		dbCtx, cancel := context.WithTimeout(r.Context(), dbOpTimeout)
		defer cancel()

		// Пост должен существовать — так сразу отдаём понятный 404, а не
		// нарушение FK внутри INSERT.
		if _, err := userRepo.GetPostByID(dbCtx, postID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
				return
			}
			slog.Error("не удалось проверить существование поста для комментария", "post_id", postID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при создании комментария")
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
					RespondWithError(w, http.StatusBadRequest, "Параметр parent_id: комментарий-родитель не найден")
					return
				}
				slog.Error("не удалось проверить комментарий-родитель", "parent_id", *input.ParentID, "error", err)
				RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при создании комментария")
				return
			}
			if parent.PostID != postID || parent.ParentID != nil {
				RespondWithError(w, http.StatusBadRequest, "Параметр parent_id: ответить можно только на корневой комментарий этого поста")
				return
			}
		}

		comment, err := userRepo.CreateCommentWithAuthor(dbCtx, postID, authorID, content, input.ParentID)
		if err != nil {
			// Гонка «пост/автор/родитель исчезли между проверкой и вставкой»:
			// INSERT ... SELECT ... WHERE даёт ErrNoRows, а прямое нарушение
			// FK — pgcode 23503. По спеке оба случая — 404. Для ответа уточняем,
			// что именно пропало: перепроверяем пост, затем родителя.
			if errors.Is(err, pgx.ErrNoRows) || isForeignKeyViolation(err) {
				if _, perr := userRepo.GetPostByID(dbCtx, postID); perr != nil {
					if errors.Is(perr, pgx.ErrNoRows) {
						RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
						return
					}
					slog.Error("не удалось перепроверить пост при создании комментария", "post_id", postID, "error", perr)
					RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при создании комментария")
					return
				}
				if input.ParentID != nil {
					RespondWithError(w, http.StatusNotFound, "Комментарий-родитель не найден")
					return
				}
				RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при создании комментария")
				slog.Error("создание корневого комментария не удалось при существующем посте", "post_id", postID, "author_id", authorID, "error", err)
				return
			}
			slog.Error("не удалось создать комментарий", "post_id", postID, "author_id", authorID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при создании комментария")
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
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		postID, ok := parsePostIDPathValue(r)
		if !ok {
			RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор публикации")
			return
		}

		limit, err := parseQueryInt(r, "limit", defaultCommentsLimit)
		if err != nil || limit < 1 || limit > maxCommentsLimit {
			RespondWithError(w, http.StatusBadRequest, "Параметр limit: целое число от 1 до 100")
			return
		}
		offset, err := parseQueryInt(r, "offset", 0)
		if err != nil || offset < 0 {
			RespondWithError(w, http.StatusBadRequest, "Параметр offset: целое неотрицательное число")
			return
		}

		// Для анонимов UserID в контексте нет — GetCommentsByPostID трактует 0
		// как «никого не смотрим», liked_by_me=false.
		viewerID, _ := r.Context().Value(UserIDKey).(int)

		userRepo := repository.NewUserRepository(database.GlobalPool)
		dbCtx, cancel := context.WithTimeout(r.Context(), dbOpTimeout)
		defer cancel()

		if _, err := userRepo.GetPostByID(dbCtx, postID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
				return
			}
			slog.Error("не удалось проверить существование поста для комментариев", "post_id", postID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке комментариев")
			return
		}

		comments, err := userRepo.GetCommentsByPostID(dbCtx, postID, limit, offset, viewerID)
		if err != nil {
			slog.Error("не удалось загрузить комментарии", "post_id", postID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке комментариев")
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
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		postID, ok := parsePostIDPathValue(r)
		if !ok {
			RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор публикации")
			return
		}

		commentID, ok := parsePathID(r, "commentID")
		if !ok {
			RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор комментария")
			return
		}

		userID, ok := r.Context().Value(UserIDKey).(int)
		if !ok {
			RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
			return
		}

		userRepo := repository.NewUserRepository(database.GlobalPool)
		dbCtx, cancel := context.WithTimeout(r.Context(), dbOpTimeout)
		defer cancel()

		comment, err := userRepo.GetCommentByID(dbCtx, commentID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
				return
			}
			slog.Error("не удалось проверить комментарий перед лайком", "comment_id", commentID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при обработке лайка")
			return
		}
		if comment.PostID != postID {
			RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
			return
		}

		count, err := userRepo.AddCommentLike(dbCtx, commentID, userID)
		if err != nil {
			// Комментарий могли удалить между проверкой и вставкой — это 404.
			if isForeignKeyViolation(err) {
				RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
				return
			}
			slog.Error("не удалось поставить лайк комментарию", "comment_id", commentID, "user_id", userID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при обработке лайка")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(likeResponse{LikesCount: count})
	}
}

// UnlikeCommentHandler обрабатывает снятие лайка с комментария
// (DELETE /posts/{id}/comments/{commentID}/likes). Требует авторизацию.
// Снятие идемпотентно. Ответ — {likes_count}.
func UnlikeCommentHandler() http.HandlerFunc {
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

		commentID, ok := parsePathID(r, "commentID")
		if !ok {
			RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор комментария")
			return
		}

		userID, ok := r.Context().Value(UserIDKey).(int)
		if !ok {
			RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
			return
		}

		userRepo := repository.NewUserRepository(database.GlobalPool)
		dbCtx, cancel := context.WithTimeout(r.Context(), dbOpTimeout)
		defer cancel()

		comment, err := userRepo.GetCommentByID(dbCtx, commentID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
				return
			}
			slog.Error("не удалось проверить комментарий перед снятием лайка", "comment_id", commentID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при обработке лайка")
			return
		}
		if comment.PostID != postID {
			RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
			return
		}

		count, err := userRepo.RemoveCommentLike(dbCtx, commentID, userID)
		if err != nil {
			slog.Error("не удалось снять лайк комментария", "comment_id", commentID, "user_id", userID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при обработке лайка")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(likeResponse{LikesCount: count})
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
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		postID, ok := parsePostIDPathValue(r)
		if !ok {
			RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор публикации")
			return
		}

		commentID, ok := parsePathID(r, "commentID")
		if !ok {
			RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор комментария")
			return
		}

		actorID, ok := r.Context().Value(UserIDKey).(int)
		if !ok {
			RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
			return
		}

		userRepo := repository.NewUserRepository(database.GlobalPool)
		dbCtx, cancel := context.WithTimeout(r.Context(), dbOpTimeout)
		defer cancel()

		// Грузим пост: чтобы отличить «не существует» (404) от «чужой» (403)
		// — автор поста тоже имеет право удалять комментарии под ним.
		post, err := userRepo.GetPostByID(dbCtx, postID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
				return
			}
			slog.Error("не удалось загрузить пост для удаления комментария", "post_id", postID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при удалении комментария")
			return
		}

		comment, err := userRepo.GetCommentByID(dbCtx, commentID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
				return
			}
			slog.Error("не удалось загрузить комментарий для удаления", "comment_id", commentID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при удалении комментария")
			return
		}
		// Комментарий другого поста в контексте этого пути не существует.
		if comment.PostID != postID {
			RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
			return
		}

		if comment.AuthorID != actorID && post.AuthorID != actorID {
			RespondWithError(w, http.StatusForbidden, "Удалить комментарий может только его автор или автор публикации")
			return
		}

		// Права проверяются ещё раз прямо в SQL — защита от гонок.
		deleted, err := userRepo.DeleteCommentByID(dbCtx, commentID, actorID)
		if err != nil {
			slog.Error("не удалось удалить комментарий из БД", "comment_id", commentID, "actor_id", actorID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при удалении комментария")
			return
		}
		if !deleted {
			// Между чтением и удалением комментарий исчез (удалили конкуренты).
			RespondWithError(w, http.StatusNotFound, "Комментарий не найден")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"success","message":"Комментарий успешно удалён"}`)
	}
}
