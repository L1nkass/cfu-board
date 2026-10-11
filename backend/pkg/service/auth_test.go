package service

import "testing"

// TestGenerateSecureToken проверяет, что токены случайны, достаточно длинны
// и не содержат padding (base64url без '=').
func TestGenerateSecureToken(t *testing.T) {
	a, err := generateSecureToken()
	if err != nil {
		t.Fatalf("не ожидалась ошибка генерации: %v", err)
	}
	b, err := generateSecureToken()
	if err != nil {
		t.Fatalf("не ожидалась ошибка генерации: %v", err)
	}
	if a == b {
		t.Fatal("два токена не должны совпадать")
	}
	// 32 байта в base64url без padding = 43 символа.
	if len(a) != 43 {
		t.Fatalf("ожидалась длина 43, получено %d (%q)", len(a), a)
	}
	for _, c := range a {
		if c == '=' || c == '+' || c == '/' {
			t.Fatalf("токен должен быть в base64url без padding и спецсимволов: %q", a)
		}
	}
}

// TestHashToken проверяет детерминированность и формат SHA-256 (64 hex),
// а также различие хешей для разных токенов.
func TestHashToken(t *testing.T) {
	h1 := hashToken("token-one")
	h2 := hashToken("token-one")
	h3 := hashToken("token-two")

	if h1 != h2 {
		t.Fatal("хеш должен быть детерминированным")
	}
	if h1 == h3 {
		t.Fatal("разные токены должны давать разные хеши")
	}
	if len(h1) != 64 {
		t.Fatalf("SHA-256 в hex должен быть длиной 64, получено %d", len(h1))
	}
}
