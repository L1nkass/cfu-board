package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"cfu-board/pkg/service"
)

var testJWTSecret = []byte("test-secret-for-middleware-tests-0123456789")

// signTestToken подписывает валидный access-токен тестовым секретом.
func signTestToken(t *testing.T, userID int, ttl time.Duration) string {
	t.Helper()
	claims := service.UserClaims{
		UserID:   userID,
		Username: "tester",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(testJWTSecret)
	if err != nil {
		t.Fatalf("не удалось подписать токен: %v", err)
	}
	return signed
}

func signWith(t *testing.T, userID int, secret []byte) string {
	t.Helper()
	claims := service.UserClaims{
		UserID:           userID,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// unsignedToken собирает токен с alg=none (атака подмены алгоритма).
func unsignedToken(t *testing.T, userID int) string {
	t.Helper()
	claims := service.UserClaims{
		UserID:           userID,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestUserIDFromBearerToken — разбор и валидация заголовка Authorization.
func TestUserIDFromBearerToken(t *testing.T) {
	valid := signTestToken(t, 42, time.Hour)

	id, err := userIDFromBearerToken("Bearer "+valid, testJWTSecret)
	if err != nil || id != 42 {
		t.Fatalf("ожидался userID=42, получено id=%d err=%v", id, err)
	}

	for name, header := range map[string]string{
		"пустой":          "",
		"без Bearer":      valid,
		"только Bearer":   "Bearer",
		"пустой токен":    "Bearer ",
		"три части":       "Bearer a b",
		"мусор":           "Bearer not.a.jwt",
		"чужой секрет":    "Bearer " + signWith(t, 1, testJWTSecret[:5]),
		"протухший токен": "Bearer " + signTestToken(t, 1, -time.Hour),
		"none-алгоритм":   "Bearer " + unsignedToken(t, 7),
	} {
		if _, err := userIDFromBearerToken(header, testJWTSecret); err == nil {
			t.Errorf("%s: ожидалась ошибка, получен nil", name)
		}
	}
}

// TestAuthMiddleware — строгая защита: без/битый токен → 401, валидный → дальше.
func TestAuthMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := r.Context().Value(UserIDKey).(int)
		if !ok || id != 7 {
			t.Errorf("в контекст должен попасть userID=7, получено %v", r.Context().Value(UserIDKey))
		}
		w.WriteHeader(http.StatusTeapot)
	})
	h := AuthMiddleware(testJWTSecret)(next)

	t.Run("без заголовка → 401", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("ожидался 401, получено %d", rec.Code)
		}
	})

	t.Run("битый токен → 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer мусор")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("ожидался 401, получено %d", rec.Code)
		}
	})

	t.Run("валидный токен → дальше", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+signTestToken(t, 7, time.Hour))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusTeapot {
			t.Fatalf("ожидался проход к next (418), получено %d", rec.Code)
		}
	})
}

// TestOptionalAuthMiddleware — мягкая защита: без заголовка — анонимно,
// с валидным — userID в контексте, с битым — 401.
func TestOptionalAuthMiddleware(t *testing.T) {
	var gotID any
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = r.Context().Value(UserIDKey)
		w.WriteHeader(http.StatusOK)
	})
	h := OptionalAuthMiddleware(testJWTSecret)(next)

	t.Run("аноним проходит без userID", func(t *testing.T) {
		gotID = "sentinel"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusOK || gotID != nil {
			t.Fatalf("аноним должен пройти без userID: code=%d id=%v", rec.Code, gotID)
		}
	})

	t.Run("валидный токен кладёт userID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+signTestToken(t, 9, time.Hour))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		id, ok := gotID.(int)
		if rec.Code != http.StatusOK || !ok || id != 9 {
			t.Fatalf("ожидался userID=9: code=%d id=%v", rec.Code, gotID)
		}
	})

	t.Run("битый токен → 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer мусор")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("ожидался 401, получено %d", rec.Code)
		}
	})
}

// TestContentTypeForExt — MIME-типы медиа и фолбэк.
func TestContentTypeForExt(t *testing.T) {
	cases := map[string]string{
		".jpg":  "image/jpeg",
		".JPG":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".webp": "image/webp",
		".mp4":  "video/mp4",
		".mov":  "video/quicktime",
		".avi":  "video/x-msvideo",
		".gif":  "application/octet-stream",
		"":      "application/octet-stream",
	}
	for ext, want := range cases {
		if got := contentTypeForExt(ext); got != want {
			t.Errorf("contentTypeForExt(%q) = %q, ожидалось %q", ext, got, want)
		}
	}
}
