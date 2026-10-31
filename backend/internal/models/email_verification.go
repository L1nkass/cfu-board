package models

import "time"

// EmailVerification — токен подтверждения email. В БД хранится только его
// SHA-256 хеш (TokenHash); сам токен отдаётся в письме один раз.
// UsedAt != nil означает, что токен более недействителен: либо уже применён,
// либо заменён при повторной отправке письма.
type EmailVerification struct {
	ID        int
	UserID    int
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
	UsedAt    *time.Time
}

// IsExpired — true, если срок действия токена истёк.
func (e *EmailVerification) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// IsUsed — true, если токен уже был использован или заменён новым.
func (e *EmailVerification) IsUsed() bool {
	return e.UsedAt != nil
}
