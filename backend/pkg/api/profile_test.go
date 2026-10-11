package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetUserProfileHandler_BadID проверяет, что некорректный id в пути
// возвращает 400 ещё до обращения к БД.
func TestGetUserProfileHandler_BadID(t *testing.T) {
	cases := []struct {
		name string
		id   string
	}{
		{"не число", "abc"},
		{"ноль", "0"},
		{"отрицательный", "-5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/users/"+c.id, nil)
			req.SetPathValue("id", c.id)
			rec := httptest.NewRecorder()

			GetUserProfileHandler()(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("ожидался код 400 для id=%q, получено %d", c.id, rec.Code)
			}
		})
	}
}

// TestGetUserProfileHandler_MethodNotAllowed проверяет, что не-GET метод
// отклоняется до любой работы с БД.
func TestGetUserProfileHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/users/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	GetUserProfileHandler()(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("ожидался код 405, получено %d", rec.Code)
	}
}

// TestGetMyProfileHandler_NoAuthContext проверяет, что без идентификатора
// пользователя в контексте (middleware не отработал) хендлер отвечает 401
// и не пытается лезть в БД.
func TestGetMyProfileHandler_NoAuthContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	rec := httptest.NewRecorder()

	GetMyProfileHandler()(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ожидался код 401 без контекста авторизации, получено %d", rec.Code)
	}
}
