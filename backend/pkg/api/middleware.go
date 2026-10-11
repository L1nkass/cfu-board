package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"cfu-board/pkg/service"

	"github.com/golang-jwt/jwt/v5"
)

// Кастомный тип для ключа контекста, чтобы избежать конфликтов в памяти Go
type contextKey string

const UserIDKey contextKey = "userID"

// userIDFromBearerToken извлекает и валидирует access-токен из заголовка
// Authorization (формат "Bearer <токен>") и возвращает userID из claims.
// Любая ошибка означает, что токен отсутствует, неверно оформлен или недействителен.
func userIDFromBearerToken(authHeader string, jwtSecret []byte) (int, error) {
	// 1. Заголовок должен быть в формате "Bearer <токен>"
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
		return 0, fmt.Errorf("неверный формат заголовка Authorization")
	}
	tokenString := parts[1]

	// 2. Расшифровываем JWT и проверяем цифровую подпись.
	var claims service.UserClaims
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		// Защита от подмены алгоритма подписи: принимаем только HMAC.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("неожиданный алгоритм подписи JWT: %v", t.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return 0, fmt.Errorf("токен недействителен или его срок истёк")
	}
	return claims.UserID, nil
}

// AuthMiddleware принимает секретный ключ JWT и возвращает промежуточный слой защиты
func AuthMiddleware(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				RespondWithError(w, http.StatusUnauthorized, "Доступ запрещен: отсутствует токен авторизации")
				return
			}

			userID, err := userIDFromBearerToken(authHeader, jwtSecret)
			if err != nil {
				RespondWithError(w, http.StatusUnauthorized, "Доступ запрещен: токен недействителен или его срок истек")
				return
			}

			// Зашиваем UserID внутрь контекста запроса (ctx), чтобы защищенные хендлеры
			// могли мгновенно узнать, какой именно пользователь сейчас делает запрос!
			ctx := context.WithValue(r.Context(), UserIDKey, userID)

			// Пропускаем запрос дальше по конвейеру к реальному хендлеру!
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuthMiddleware — «мягкая» авторизация для публичных роутов с
// персонализацией: без заголовка Authorization запрос проходит анонимно, а при
// валидном токене в контекст подкладывается UserID (например, для поля
// liked_by_me в GET /posts/{id}). Невалидный/протухший токен при наличии
// заголовка считается ошибкой (401): клиент должен узнать, что его сессия
// недействительна, а не получить «анонимный» ответ посреди авторизованной сессии.
func OptionalAuthMiddleware(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// Анонимный запрос — пропускаем без UserID в контексте.
				next.ServeHTTP(w, r)
				return
			}

			userID, err := userIDFromBearerToken(authHeader, jwtSecret)
			if err != nil {
				RespondWithError(w, http.StatusUnauthorized, "Доступ запрещен: токен недействителен или его срок истек")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
