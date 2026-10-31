package models

import (
	"testing"
	"time"
)

// TestRefreshToken_State проверяет предикаты IsExpired/IsRevoked.
func TestRefreshToken_State(t *testing.T) {
	expired := &RefreshToken{ExpiresAt: time.Now().Add(-time.Hour)}
	if !expired.IsExpired() {
		t.Fatal("протухший токен должен быть IsExpired")
	}
	if expired.IsRevoked() {
		t.Fatal("неотозванный токен не должен быть IsRevoked")
	}

	revokedAt := time.Now()
	revoked := &RefreshToken{ExpiresAt: time.Now().Add(time.Hour), RevokedAt: &revokedAt}
	if revoked.IsExpired() {
		t.Fatal("свежий токен не должен быть IsExpired")
	}
	if !revoked.IsRevoked() {
		t.Fatal("отозванный токен должен быть IsRevoked")
	}

	fresh := &RefreshToken{ExpiresAt: time.Now().Add(time.Hour)}
	if fresh.IsExpired() || fresh.IsRevoked() {
		t.Fatal("свежий активный токен: оба предиката false")
	}
}

// TestEmailVerification_State проверяет предикаты IsExpired/IsUsed.
func TestEmailVerification_State(t *testing.T) {
	expired := &EmailVerification{ExpiresAt: time.Now().Add(-time.Minute)}
	if !expired.IsExpired() {
		t.Fatal("протухший токен должен быть IsExpired")
	}

	usedAt := time.Now()
	used := &EmailVerification{ExpiresAt: time.Now().Add(time.Hour), UsedAt: &usedAt}
	if !used.IsUsed() {
		t.Fatal("использованный токен должен быть IsUsed")
	}

	fresh := &EmailVerification{ExpiresAt: time.Now().Add(time.Hour)}
	if fresh.IsExpired() || fresh.IsUsed() {
		t.Fatal("свежий токен: оба предиката false")
	}
}
