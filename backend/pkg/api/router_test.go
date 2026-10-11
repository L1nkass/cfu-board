package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cfu-board/pkg/config"
)

// testRouter собирает NewRouter с минимальными зависимостями: authService=nil
// (хендлеры auth не вызываются), pool=nil (readiness не вызывается), cfg с
// коротким таймаутом, store=nil (аплоад не вызывается).
func testRouter() http.Handler {
	cfg := &config.Config{}
	cfg.HTTP.RequestTimeout = 5 * time.Second
	cfg.HTTP.UploadTimeout = 5 * time.Second
	cfg.HTTP.AllowedOrigins = []string{"*"}
	return NewRouter(nil, []byte("test-secret"), nil, cfg, nil)
}

// TestRouter_Smoke проверяет, что все зарегистрированные пути отвечают
// ожидаемым кодом, а не 404 от mux. Тела не проверяются (нужна БД), важен
// факт маршрутизации: путь найден, метод распознан.
func TestRouter_Smoke(t *testing.T) {
	router := testRouter()

	// method, path → ожидаемый код. 4xx здесь означает «роут найден, хендлер
	// отработал валидацию» — это успех маршрутизации.
	cases := []struct {
		method string
		path   string
		want   int
	}{
		// Auth (без тела → 400 от хендлера, не 404 от mux).
		{http.MethodPost, "/register", http.StatusBadRequest},
		{http.MethodPost, "/login", http.StatusBadRequest},
		{http.MethodPost, "/refresh", http.StatusBadRequest},
		{http.MethodPost, "/logout", http.StatusBadRequest},
		{http.MethodPost, "/resend-verification", http.StatusBadRequest},
		// Posts: лента vs один пост — конфликт шаблонов ловится здесь.
		{http.MethodGet, "/posts", http.StatusInternalServerError}, // роут есть, дальше нужна БД
		{http.MethodGet, "/posts/abc", http.StatusBadRequest},      // {id} не число → 400
		{http.MethodGet, "/posts/1/comments?limit=999", http.StatusBadRequest},
		{http.MethodPost, "/posts/1/comments", http.StatusUnauthorized}, // auth без токена
		{http.MethodDelete, "/posts/1", http.StatusUnauthorized},
		// Users.
		{http.MethodGet, "/users/abc", http.StatusBadRequest},
		{http.MethodGet, "/users/abc/posts", http.StatusBadRequest},
		{http.MethodGet, "/users/me", http.StatusUnauthorized},
		// Health и docs.
		{http.MethodGet, "/healthz", http.StatusOK},
		{http.MethodGet, "/docs", http.StatusOK},
		// Несуществующий путь — единственный легитимный 404 от mux.
		{http.MethodGet, "/no-such-route", http.StatusNotFound},
	}

	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s %s: ожидался %d, получено %d (тело: %s)",
				c.method, c.path, c.want, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("X-Request-ID"); got == "" {
			t.Errorf("%s %s: нет заголовка X-Request-ID", c.method, c.path)
		}
	}
}

// TestRouter_UploadBypass проверяет, что POST /posts/create идёт отдельным
// хендлером (мимо глобального Timeout): без токена отвечает 401 от auth,
// а не обрывом по таймауту.
func TestRouter_UploadBypass(t *testing.T) {
	router := testRouter()
	req := httptest.NewRequest(http.MethodPost, "/posts/create", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ожидался 401 без токена, получено %d", rec.Code)
	}
}
