package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestRefreshToken_Lifecycle проверяет создание, чтение по хешу и идемпотентный
// revoke токена обновления.
func TestRefreshToken_Lifecycle(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	userRepo := NewUserRepository(pool)
	u := seedUser(t, userRepo, "rt", "rt@example.com")
	repo := NewRefreshTokenRepository(pool)

	hash := "a" + repeat("0", 63) // 64 hex-символа
	if err := repo.Create(context.Background(), u.ID, hash, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if got.UserID != u.ID || got.IsRevoked() {
		t.Fatalf("GetByHash: %+v", got)
	}

	if err := repo.Revoke(context.Background(), got.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if err := repo.Revoke(context.Background(), got.ID); err != nil {
		t.Fatalf("повторный Revoke: %v", err)
	}
	got2, err := repo.GetByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetByHash после revoke: %v", err)
	}
	if !got2.IsRevoked() {
		t.Fatal("токен должен быть отозван")
	}

	if _, err := repo.GetByHash(context.Background(), repeat("b", 64)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("неизвестный хеш: ожидался ErrNoRows, получено %v", err)
	}
}

// TestRefreshToken_RevokeAllForUser проверяет отзыв всех токенов пользователя.
func TestRefreshToken_RevokeAllForUser(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	userRepo := NewUserRepository(pool)
	u := seedUser(t, userRepo, "rtall", "rtall@example.com")
	repo := NewRefreshTokenRepository(pool)

	if err := repo.Create(context.Background(), u.ID, repeat("1", 64), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Create 1: %v", err)
	}
	if err := repo.Create(context.Background(), u.ID, repeat("2", 64), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Create 2: %v", err)
	}
	if err := repo.RevokeAllForUser(context.Background(), u.ID); err != nil {
		t.Fatalf("RevokeAllForUser: %v", err)
	}

	t1, _ := repo.GetByHash(context.Background(), repeat("1", 64))
	t2, _ := repo.GetByHash(context.Background(), repeat("2", 64))
	if !t1.IsRevoked() || !t2.IsRevoked() {
		t.Fatal("все токены пользователя должны быть отозваны")
	}
}

// TestEmailVerification_Lifecycle проверяет создание, чтение по хешу и
// идемпотентное MarkUsed токена верификации.
func TestEmailVerification_Lifecycle(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	userRepo := NewUserRepository(pool)
	u := seedUser(t, userRepo, "ev", "ev@example.com")
	repo := NewEmailVerificationRepository(pool)

	hash := repeat("3", 64)
	if err := repo.Create(context.Background(), u.ID, hash, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if got.UserID != u.ID || got.IsUsed() {
		t.Fatalf("GetByHash: %+v", got)
	}

	if err := repo.MarkUsed(context.Background(), got.ID); err != nil {
		t.Fatalf("MarkUsed: %v", err)
	}
	if err := repo.MarkUsed(context.Background(), got.ID); err != nil {
		t.Fatalf("повторный MarkUsed: %v", err)
	}
	got2, err := repo.GetByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetByHash после MarkUsed: %v", err)
	}
	if !got2.IsUsed() {
		t.Fatal("токен должен быть помечен использованным")
	}
}

// TestEmailVerification_InvalidateAndRateLimit проверяет инвалидацию активных
// токенов и helper'ы лимитирования частоты (LastCreatedAt, CountSince).
func TestEmailVerification_InvalidateAndRateLimit(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	userRepo := NewUserRepository(pool)
	u := seedUser(t, userRepo, "evrl", "evrl@example.com")
	repo := NewEmailVerificationRepository(pool)

	// Пустое состояние: LastCreatedAt — zero, CountSince == 0.
	last, err := repo.LastCreatedAt(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("LastCreatedAt (пусто): %v", err)
	}
	if !last.IsZero() {
		t.Fatalf("LastCreatedAt при отсутствии токенов должен быть zero, получено %v", last)
	}
	if n, err := repo.CountSince(context.Background(), u.ID, time.Now().Add(-time.Hour)); err != nil || n != 0 {
		t.Fatalf("CountSince (пусто): n=%d err=%v", n, err)
	}

	if err := repo.Create(context.Background(), u.ID, repeat("4", 64), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Create 1: %v", err)
	}
	if err := repo.Create(context.Background(), u.ID, repeat("5", 64), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	if n, err := repo.CountSince(context.Background(), u.ID, time.Now().Add(-time.Hour)); err != nil || n != 2 {
		t.Fatalf("CountSince после 2 созданий: n=%d err=%v", n, err)
	}
	last, err = repo.LastCreatedAt(context.Background(), u.ID)
	if err != nil || last.IsZero() {
		t.Fatalf("LastCreatedAt после создания: %v err=%v", last, err)
	}

	if err := repo.InvalidateActiveForUser(context.Background(), u.ID); err != nil {
		t.Fatalf("InvalidateActiveForUser: %v", err)
	}
	tok, err := repo.GetByHash(context.Background(), repeat("4", 64))
	if err != nil {
		t.Fatalf("GetByHash после инвалидации: %v", err)
	}
	if !tok.IsUsed() {
		t.Fatal("токен должен стать недействительным после инвалидации")
	}
}

// repeat возвращает строку из n копий s (для валидных 64-символьных хешей).
func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
