package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cfu-board/pkg/database"
	"cfu-board/pkg/models"
	"cfu-board/pkg/repository"
)

// adminOpTimeout — таймаут операций модерации.
const adminOpTimeout = 5 * time.Second

// adminModerationLimit/pendingLimit ограничивают пагинацию очереди модерации.
const (
	defaultAdminLimit = 20
	maxAdminLimit     = 100
)

// requireAdmin проверяет, что текущий пользователь — админ. Идентификатор берётся
// из JWT-контекста (middleware), а статус админа — АКТУАЛЬНЫЙ из БД (а не из
// токена): права могли быть отозваны после выдачи access-токена. При отсутствии
// прав/ошибке пишет стандартный ответ и возвращает (userID, false).
func requireAdmin(w http.ResponseWriter, r *http.Request) (int, bool) {
	userID, ok := r.Context().Value(UserIDKey).(int)
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
		return 0, false
	}

	userRepo := repository.NewUserRepository(database.GlobalPool)
	ctx, cancel := context.WithTimeout(r.Context(), adminOpTimeout)
	defer cancel()

	user, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			RespondWithError(w, http.StatusUnauthorized, "Пользователь не найден")
			return 0, false
		}
		slog.Error("не удалось проверить права админа", "user_id", userID, "error", err)
		RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при проверке прав")
		return 0, false
	}
	if !user.IsAdmin {
		RespondWithError(w, http.StatusForbidden, "Доступ разрешён только администраторам")
		return 0, false
	}
	return userID, true
}

// AdminListPendingHandler обрабатывает GET /admin/posts/pending — очередь
// модерации: посты со статусом pending (старые сверху, FIFO). Доступен только
// админам. Поддерживает пагинацию limit/offset (как лента постов).
func AdminListPendingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}
		if _, ok := requireAdmin(w, r); !ok {
			return
		}

		limit, err := parseQueryInt(r, "limit", defaultAdminLimit)
		if err != nil || limit < 1 || limit > maxAdminLimit {
			RespondWithError(w, http.StatusBadRequest, "Параметр limit: целое число от 1 до 100")
			return
		}
		offset, err := parseQueryInt(r, "offset", 0)
		if err != nil || offset < 0 {
			RespondWithError(w, http.StatusBadRequest, "Параметр offset: целое неотрицательное число")
			return
		}

		userRepo := repository.NewUserRepository(database.GlobalPool)
		ctx, cancel := context.WithTimeout(r.Context(), adminOpTimeout)
		defer cancel()

		posts, err := userRepo.ListPendingPosts(ctx, limit, offset)
		if err != nil {
			slog.Error("не удалось загрузить очередь модерации", "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке очереди модерации")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(posts)
	}
}

// moderationInput — тело запроса отклонения поста.
type moderationInput struct {
	Reason string `json:"reason"` // причина отказа (опционально)
}

// AdminModeratePostHandler обрабатывает POST /admin/posts/{id}/approve и
// POST /admin/posts/{id}/reject — решение админа по посту на модерации.
// Оба действия меняют статус поста на approved/rejected, фиксируют, кто и когда
// принял решение; при отказе можно указать причину. Доступно только админам.
// Повторное решение по уже отмодерированному посту тоже возможно (пере-одобрение
// или смена причины отказа).
func AdminModeratePostHandler(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}
		adminID, ok := requireAdmin(w, r)
		if !ok {
			return
		}

		postID, ok := parsePostIDPathValue(r)
		if !ok {
			RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор публикации")
			return
		}

		// Причина отказа — только для reject, читаем тело (оно может отсутствовать).
		var reason string
		if action == models.PostStatusRejected {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB — с запасом
			var input moderationInput
			// Пустое тело — не ошибка: причина опциональна.
			_ = json.NewDecoder(r.Body).Decode(&input)
			reason = strings.TrimSpace(input.Reason)
			if len(reason) > 1000 {
				RespondWithError(w, http.StatusBadRequest, "Причина отказа слишком длинная (максимум 1000 символов)")
				return
			}
		}

		userRepo := repository.NewUserRepository(database.GlobalPool)
		ctx, cancel := context.WithTimeout(r.Context(), adminOpTimeout)
		defer cancel()

		updated, err := userRepo.SetPostModeration(ctx, postID, action, adminID, reason)
		if err != nil {
			slog.Error("не удалось сохранить решение модерации", "post_id", postID, "action", action, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при сохранении решения")
			return
		}
		if !updated {
			RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
			return
		}

		msg := "Публикация одобрена и теперь видна в общей ленте"
		if action == models.PostStatusRejected {
			msg = "Публикация отклонена"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "success",
			"message": msg,
			"post_id": postID,
			"action":  action,
		})
	}
}
