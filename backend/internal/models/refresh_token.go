package models

import "time"

// RefreshToken — долгоживущий токен обновления. В БД хранится только его
// SHA-256 хеш (TokenHash); сам токен отдаётся клиенту один раз при выдаче.
type RefreshToken struct {
	ID        int
	UserID    int
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time // nil — активен; не-nil — отозван (logout или ротация)
}

// IsExpired — true, если срок действия токена истёк.
func (t *RefreshToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsRevoked — true, если токен был отозван.
func (t *RefreshToken) IsRevoked() bool {
	return t.RevokedAt != nil
}
