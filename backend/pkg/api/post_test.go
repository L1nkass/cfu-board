package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestListPostsHandler_Validation — валидация ленты до обращения к БД:
// limit/offset проверяются первыми, затем метод. Без токена тоже работает
// (публичный роут), авторизация не требуется.
func TestListPostsHandler_Validation(t *testing.T) {
	t.Run("невалидный limit", func(t *testing.T) {
		for _, q := range []string{"?limit=0", "?limit=101", "?limit=999", "?limit=abc"} {
			req := httptest.NewRequest(http.MethodGet, "/posts"+q, nil)
			rec := httptest.NewRecorder()

			ListPostsHandler()(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("ожидался 400 для %q, получено %d", q, rec.Code)
			}
		}
	})

	t.Run("невалидный offset", func(t *testing.T) {
		for _, q := range []string{"?offset=-1", "?offset=abc"} {
			req := httptest.NewRequest(http.MethodGet, "/posts"+q, nil)
			rec := httptest.NewRecorder()

			ListPostsHandler()(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("ожидался 400 для %q, получено %d", q, rec.Code)
			}
		}
	})

	t.Run("неверный метод", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/posts", nil)
		rec := httptest.NewRecorder()

		ListPostsHandler()(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("ожидался 405, получено %d", rec.Code)
		}
	})
}
