package api

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

	"cfu-board/pkg/database"
	"cfu-board/pkg/repository"
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
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		userID, ok := r.Context().Value(UserIDKey).(int)
		if !ok {
			RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
			return
		}

		userRepo := repository.NewUserRepository(database.GlobalPool)

		ctx, cancel := context.WithTimeout(r.Context(), profileOpTimeout)
		defer cancel()

		user, err := userRepo.GetByID(ctx, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RespondWithError(w, http.StatusNotFound, "Пользователь не найден")
				return
			}
			slog.Error("не удалось загрузить профиль", "user_id", userID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке профиля")
			return
		}

		postsCount, err := userRepo.CountPostsByAuthor(ctx, userID)
		if err != nil {
			slog.Error("не удалось подсчитать публикации пользователя", "user_id", userID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке профиля")
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
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		idParam := strings.TrimSpace(r.PathValue("id"))
		userID, err := strconv.Atoi(idParam)
		if err != nil || userID <= 0 {
			RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор пользователя")
			return
		}

		userRepo := repository.NewUserRepository(database.GlobalPool)

		ctx, cancel := context.WithTimeout(r.Context(), profileOpTimeout)
		defer cancel()

		user, err := userRepo.GetByID(ctx, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RespondWithError(w, http.StatusNotFound, "Пользователь не найден")
				return
			}
			slog.Error("не удалось загрузить пользователя", "user_id", userID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке профиля")
			return
		}

		postsCount, err := userRepo.CountPostsByAuthor(ctx, userID)
		if err != nil {
			slog.Error("не удалось подсчитать публикации пользователя", "user_id", userID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке профиля")
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
