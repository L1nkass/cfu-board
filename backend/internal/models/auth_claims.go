package models

import (
	"github.com/golang-jwt/jwt/v5"
)

// UserClaims описывает структуру полезной нагрузки внутри JWT access-токена.
// Вынесена в пакет models, чтобы platform/middleware не зависел
// от modules/auth/service (слой инфраструктуры не должен знать о бизнес-логике).
type UserClaims struct {
	UserID     int    `json:"user_id"`
	Username   string `json:"username"`
	IsAdmin    bool   `json:"is_admin"`
	IsVerified bool   `json:"is_verified"`
	jwt.RegisteredClaims
}
