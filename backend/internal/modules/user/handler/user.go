package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	postrepo "cfu-board/internal/modules/post/repository"
	"cfu-board/internal/modules/user/repository"
	"cfu-board/internal/platform/database"
	"cfu-board/internal/platform/middleware"
)

// profileOpTimeout — таймаут операций профиля (загрузка пользователя + счётчик постов).
const profileOpTimeout = 5 * time.Second

// PublicProfile — публичное представление профиля пользователя. Сознательно НЕ
// содержит email и хеш пароля: приватные поля не должны утекать при просмотре
// чужого профиля. Используется для GET /users/{id}.
type PublicProfile struct {
	ID         int       `json:"id"`
	Username   string    `json:"username"`
	AvatarURL  string    `json:"avatar_url"`
	IsVerified bool      `json:"is_verified"`
	IsAdmin    bool      `json:"is_admin"`
	CreatedAt  time.Time `json:"created_at"`
	PostsCount int       `json:"posts_count"`
}

// OwnProfile — профиль текущего пользователя (GET /users/me). В отличие от
// публичного, включает собственный email. Хеш пароля не отдаётся никогда.
type OwnProfile struct {
	ID         int       `json:"id"`
	Username   string    `json:"username"`
	AvatarURL  string    `json:"avatar_url"`
	Email      string    `json:"email"`
	IsVerified bool      `json:"is_verified"`
	IsAdmin    bool      `json:"is_admin"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	PostsCount int       `json:"posts_count"`
}

// GetMyProfileHandler обрабатывает GET /users/me — профиль текущего пользователя.
// Идентификатор берётся из JWT-контекста (middleware), а пользователь и счётчик
// постов — из БД, чтобы отдавать актуальные данные (а не устаревшие из токена).
func GetMyProfileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			middleware.RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		userID, ok := r.Context().Value(middleware.UserIDKey).(int)
		if !ok {
			middleware.RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
			return
		}

		userRepo := repository.NewUserRepository(database.GlobalPool)

		ctx, cancel := context.WithTimeout(r.Context(), profileOpTimeout)
		defer cancel()

		user, err := userRepo.GetByID(ctx, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				middleware.RespondWithError(w, http.StatusNotFound, "Пользователь не найден")
				return
			}
			slog.Error("не удалось загрузить профиль", "user_id", userID, "error", err)
			middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке профиля")
			return
		}

		postsCount, err := postrepo.NewRepository(database.GlobalPool).CountPostsByAuthor(ctx, userID, userID)
		if err != nil {
			slog.Error("не удалось подсчитать публикации пользователя", "user_id", userID, "error", err)
			middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке профиля")
			return
		}

		profile := OwnProfile{
			ID:         user.ID,
			Username:   user.Username,
			AvatarURL:  user.AvatarURL,
			Email:      user.Email,
			IsVerified: user.IsVerified,
			IsAdmin:    user.IsAdmin,
			CreatedAt:  user.CreatedAt,
			UpdatedAt:  user.UpdatedAt,
			PostsCount: postsCount,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(profile)
	}
}

// GetUserProfileHandler обрабатывает GET /users/{id} — публичный профиль
// пользователя по его идентификатору. Приватные поля (email) не возвращаются.
func GetUserProfileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			middleware.RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		idParam := strings.TrimSpace(r.PathValue("id"))
		userID, err := strconv.Atoi(idParam)
		if err != nil || userID <= 0 {
			middleware.RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор пользователя")
			return
		}

		userRepo := repository.NewUserRepository(database.GlobalPool)

		ctx, cancel := context.WithTimeout(r.Context(), profileOpTimeout)
		defer cancel()

		user, err := userRepo.GetByID(ctx, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				middleware.RespondWithError(w, http.StatusNotFound, "Пользователь не найден")
				return
			}
			slog.Error("не удалось загрузить пользователя", "user_id", userID, "error", err)
			middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке профиля")
			return
		}

		viewerID, _ := r.Context().Value(middleware.UserIDKey).(int)
		postsCount, err := postrepo.NewRepository(database.GlobalPool).CountPostsByAuthor(ctx, userID, viewerID)
		if err != nil {
			slog.Error("не удалось подсчитать публикации пользователя", "user_id", userID, "error", err)
			middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке профиля")
			return
		}

		profile := PublicProfile{
			ID:         user.ID,
			Username:   user.Username,
			AvatarURL:  user.AvatarURL,
			IsVerified: user.IsVerified,
			IsAdmin:    user.IsAdmin,
			CreatedAt:  user.CreatedAt,
			PostsCount: postsCount,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(profile)
	}
}

// ListUserPostsHandler обрабатывает GET /users/{id}/posts — публичную ленту
// публикаций одного автора (новые сверху, limit/offset как в GET /posts).
// При валидном токене liked_by_me персонализируется (OptionalAuthMiddleware),
// анонимы получают false. Несуществующий автор — 404, а не пустой список.
func ListUserPostsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			middleware.RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		authorID, err := strconv.Atoi(strings.TrimSpace(r.PathValue("id")))
		if err != nil || authorID <= 0 {
			middleware.RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор пользователя")
			return
		}

		limit, err := middleware.ParseQueryInt(r, "limit", middleware.DefaultPostsLimit)
		if err != nil || limit < 1 || limit > middleware.MaxPostsLimit {
			middleware.RespondWithError(w, http.StatusBadRequest, "Параметр limit: целое число от 1 до 100")
			return
		}
		offset, err := middleware.ParseQueryInt(r, "offset", 0)
		if err != nil || offset < 0 {
			middleware.RespondWithError(w, http.StatusBadRequest, "Параметр offset: целое неотрицательное число")
			return
		}

		viewerID, _ := r.Context().Value(middleware.UserIDKey).(int)

		userRepo := repository.NewUserRepository(database.GlobalPool)
		ctx, cancel := context.WithTimeout(r.Context(), profileOpTimeout)
		defer cancel()

		if _, err := userRepo.GetByID(ctx, authorID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				middleware.RespondWithError(w, http.StatusNotFound, "Пользователь не найден")
				return
			}
			slog.Error("не удалось проверить автора для ленты", "user_id", authorID, "error", err)
			middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке публикаций")
			return
		}

		posts, err := postrepo.NewRepository(database.GlobalPool).ListPostsByAuthorWithLikes(ctx, viewerID, authorID, limit, offset)
		if err != nil {
			slog.Error("не удалось загрузить публикации автора", "user_id", authorID, "error", err)
			middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке публикаций")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(posts)
	}
}
