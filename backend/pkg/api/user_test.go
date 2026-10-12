package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// authHandlers — все хендлеры, принимающие *service.AuthService. Передаём nil:
// до вызова сервиса дело не доходит, т.к. валидация метода/JSON идёт первой.
func TestAuthHandlers_MethodAndJSON(t *testing.T) {
	cases := []struct {
		name    string
		handler func() http.HandlerFunc
		method  string
		path    string
	}{
		{"register", func() http.HandlerFunc { return RegisterHandler(nil) }, http.MethodPost, "/register"},
		{"login", func() http.HandlerFunc { return LoginHandler(nil) }, http.MethodPost, "/login"},
		{"refresh", func() http.HandlerFunc { return RefreshHandler(nil) }, http.MethodPost, "/refresh"},
		{"logout", func() http.HandlerFunc { return LogoutHandler(nil) }, http.MethodPost, "/logout"},
		{"resend", func() http.HandlerFunc { return ResendVerificationHandler(nil) }, http.MethodPost, "/resend-verification"},
	}

	for _, c := range cases {
		t.Run(c.name+"/неверный метод → 405", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, c.path, nil)
			rec := httptest.NewRecorder()
			c.handler()(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("ожидался 405, получено %d", rec.Code)
			}
		})

		t.Run(c.name+"/битый JSON → 400", func(t *testing.T) {
			req := httptest.NewRequest(c.method, c.path, strings.NewReader(`{`))
			rec := httptest.NewRecorder()
			c.handler()(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("ожидался 400, получено %d", rec.Code)
			}
		})
	}
}

// TestVerifyHandler_Validation — GET /verify: метод и обязательный token.
func TestVerifyHandler_Validation(t *testing.T) {
	t.Run("неверный метод → 405", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/verify?token=abc", nil)
		rec := httptest.NewRecorder()
		VerifyHandler(nil)(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("ожидался 405, получено %d", rec.Code)
		}
	})

	t.Run("без token → 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/verify", nil)
		rec := httptest.NewRecorder()
		VerifyHandler(nil)(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("ожидался 400, получено %d", rec.Code)
		}
	})
}

// TestRespondWithError_Format — единый формат {error} и Content-Type.
func TestRespondWithError_Format(t *testing.T) {
	rec := httptest.NewRecorder()
	RespondWithError(rec, http.StatusTeapot, "чайник")
	if rec.Code != http.StatusTeapot {
		t.Fatalf("ожидался 418, получено %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("ожидался application/json, получено %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "чайник") {
		t.Fatalf("тело должно содержать сообщение, получено %q", rec.Body.String())
	}
}
