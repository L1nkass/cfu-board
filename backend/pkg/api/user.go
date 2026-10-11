package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"cfu-board/pkg/service"
)

// Входные структуры (DTO) для работы с фронтендом
type RegisterInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

// ResendVerificationInput — входные данные для повторной отправки письма.
type ResendVerificationInput struct {
	Email string `json:"email"`
}

type LoginInput struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// RefreshInput — входные данные для обновления токенов.
type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

// LogoutInput — входные данные для выхода из системы.
type LogoutInput struct {
	RefreshToken string `json:"refresh_token"`
}

type JSONSuccess struct {
	Message string `json:"message"`
}

type JSONError struct {
	Error string `json:"error"`
}

// RespondWithError отправляет стандартизированную JSON ошибку
func RespondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(JSONError{Error: message})
}

// RegisterHandler обрабатывает POST-запрос на создание аккаунта
func RegisterHandler(authService *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1048576)

		var input RegisterInput
		err := json.NewDecoder(r.Body).Decode(&input)
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, "Некорректный формат JSON или превышен лимит 1MB")
			return
		}

		err = authService.Register(r.Context(), input.Username, input.Email, input.Password)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidInput),
				errors.Is(err, service.ErrUsernameLength),
				errors.Is(err, service.ErrUsernameChars),
				errors.Is(err, service.ErrEmailTooLong),
				errors.Is(err, service.ErrEmailInvalid),
				errors.Is(err, service.ErrPasswordLength):
				RespondWithError(w, http.StatusBadRequest, err.Error())

			case errors.Is(err, service.ErrUsernameTaken),
				errors.Is(err, service.ErrEmailRegistered):
				RespondWithError(w, http.StatusConflict, err.Error())

			default:
				RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при регистрации")
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(JSONSuccess{Message: "Пользователь успешно зарегистрирован"})
	}
}

// LoginHandler обрабатывает POST-запрос на аутентификацию и выдает токен
func LoginHandler(authService *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1048576)

		var input LoginInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Некорректный формат JSON")
			return
		}

		result, err := authService.Login(r.Context(), input.Login, input.Password)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrAuthFailed):
				RespondWithError(w, http.StatusUnauthorized, err.Error())
			case errors.Is(err, service.ErrInvalidInput):
				RespondWithError(w, http.StatusBadRequest, err.Error())
			default:
				RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при авторизации")
			}
			return
		}

		slog.Info("пользователь вошел в систему", "username", result.Username)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(map[string]any{
			"message":       "Авторизация прошла успешно",
			"access_token":  result.AccessToken,
			"refresh_token": result.RefreshToken,
			"user": map[string]any{
				"id":          result.UserID,
				"username":    result.Username,
				"email":       result.Email,
				"is_verified": result.IsVerified,
			},
		})
	}
}

// RefreshHandler обрабатывает POST /refresh: по действующему refresh-токену
// выдаёт новую пару (access + refresh) с ротацией старого токена.
func RefreshHandler(authService *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1048576)

		var input RefreshInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Некорректный формат JSON")
			return
		}

		result, err := authService.Refresh(r.Context(), input.RefreshToken)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrTokenInvalid):
				RespondWithError(w, http.StatusUnauthorized, "Refresh-токен недействителен")
			case errors.Is(err, service.ErrTokenExpired):
				RespondWithError(w, http.StatusUnauthorized, "Срок действия refresh-токена истёк, войдите заново")
			case errors.Is(err, service.ErrInvalidInput):
				RespondWithError(w, http.StatusBadRequest, err.Error())
			default:
				RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при обновлении токенов")
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  result.AccessToken,
			"refresh_token": result.RefreshToken,
		})
	}
}

// LogoutHandler обрабатывает POST /logout: отзывает предъявленный refresh-токен.
func LogoutHandler(authService *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1048576)

		var input LogoutInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Некорректный формат JSON")
			return
		}

		if err := authService.Logout(r.Context(), input.RefreshToken); err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidInput):
				RespondWithError(w, http.StatusBadRequest, err.Error())
			default:
				RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при выходе")
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(JSONSuccess{Message: "Выход выполнен успешно"})
	}
}

// VerifyHandler обрабатывает GET-запрос подтверждения почты из ссылки
func VerifyHandler(authService *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		tokenParam := r.URL.Query().Get("token")
		if tokenParam == "" {
			RespondWithError(w, http.StatusBadRequest, "Токен верификации отсутствует")
			return
		}

		err := authService.VerifyEmail(r.Context(), tokenParam)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrTokenInvalid):
				RespondWithError(w, http.StatusBadRequest, "Ссылка недействительна")
			case errors.Is(err, service.ErrTokenAlreadyUsed):
				RespondWithError(w, http.StatusBadRequest, "Ссылка уже была использована")
			case errors.Is(err, service.ErrTokenExpired):
				RespondWithError(w, http.StatusBadRequest, "Срок действия ссылки истёк, запросите новую")
			case errors.Is(err, service.ErrUserNotFound):
				RespondWithError(w, http.StatusNotFound, err.Error())
			case errors.Is(err, service.ErrAlreadyVerified):
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(JSONSuccess{Message: err.Error()})
			default:
				RespondWithError(w, http.StatusInternalServerError, "Не удалось подтвердить аккаунт")
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(JSONSuccess{Message: "Ваш аккаунт успешно подтвержден! Теперь вы можете войти."})
	}
}

// ResendVerificationHandler обрабатывает POST /resend-verification: повторно
// отправляет письмо подтверждения по email. Отвечает нейтрально (не раскрывает,
// существует ли адрес), защищён cooldown'ом и лимитом на стороне сервиса.
func ResendVerificationHandler(authService *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1048576)

		var input ResendVerificationInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Некорректный формат JSON")
			return
		}

		if err := authService.ResendVerification(r.Context(), input.Email); err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidInput):
				RespondWithError(w, http.StatusBadRequest, err.Error())
			default:
				RespondWithError(w, http.StatusInternalServerError, "Не удалось отправить письмо, попробуйте позже")
			}
			return
		}

		// Нейтральный ответ, чтобы не раскрывать статус адреса.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(JSONSuccess{
			Message: "Если аккаунт с таким email существует и не подтверждён, письмо отправлено",
		})
	}
}
