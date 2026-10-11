package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestLikePostHandler_Validation — валидация постановки лайка до обращения к БД.
func TestLikePostHandler_Validation(t *testing.T) {
	t.Run("некорректный id поста", func(t *testing.T) {
		for _, id := range []string{"abc", "0", "-5"} {
			req := httptest.NewRequest(http.MethodPost, "/posts/"+id+"/likes", nil)
			req.SetPathValue("id", id)
			req = req.WithContext(withUserID(req, 1))
			rec := httptest.NewRecorder()

			LikePostHandler()(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("ожидался 400 для id=%q, получено %d", id, rec.Code)
			}
		}
	})

	t.Run("без авторизации", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/posts/1/likes", nil)
		req.SetPathValue("id", "1")
		rec := httptest.NewRecorder()

		LikePostHandler()(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("ожидался 401 без контекста авторизации, получено %d", rec.Code)
		}
	})

	t.Run("неверный метод", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/posts/1/likes", nil)
		req.SetPathValue("id", "1")
		req = req.WithContext(withUserID(req, 1))
		rec := httptest.NewRecorder()

		LikePostHandler()(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("ожидался 405, получено %d", rec.Code)
		}
	})
}

// TestUnlikePostHandler_Validation — валидация снятия лайка до обращения к БД.
func TestUnlikePostHandler_Validation(t *testing.T) {
	t.Run("некорректный id поста", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/posts/xyz/likes", nil)
		req.SetPathValue("id", "xyz")
		req = req.WithContext(withUserID(req, 1))
		rec := httptest.NewRecorder()

		UnlikePostHandler()(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("ожидался 400 для некорректного id, получено %d", rec.Code)
		}
	})

	t.Run("без авторизации", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/posts/1/likes", nil)
		req.SetPathValue("id", "1")
		rec := httptest.NewRecorder()

		UnlikePostHandler()(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("ожидался 401 без контекста авторизации, получено %d", rec.Code)
		}
	})

	t.Run("неверный метод", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/posts/1/likes", nil)
		req.SetPathValue("id", "1")
		req = req.WithContext(withUserID(req, 1))
		rec := httptest.NewRecorder()

		UnlikePostHandler()(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("ожидался 405, получено %d", rec.Code)
		}
	})
}

// TestGetPostHandler_Validation — валидация просмотра поста до обращения к БД.
func TestGetPostHandler_Validation(t *testing.T) {
	t.Run("некорректный id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/posts/abc", nil)
		req.SetPathValue("id", "abc")
		rec := httptest.NewRecorder()

		GetPostHandler()(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("ожидался 400 для некорректного id, получено %d", rec.Code)
		}
	})

	t.Run("неверный метод", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/posts/1", nil)
		req.SetPathValue("id", "1")
		rec := httptest.NewRecorder()

		GetPostHandler()(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("ожидался 405, получено %d", rec.Code)
		}
	})
}

// TestIsForeignKeyViolation — распознавание pgcode 23503 (пост удалён между
// проверкой и вставкой лайка → 404).
func TestIsForeignKeyViolation(t *testing.T) {
	if isForeignKeyViolation(nil) {
		t.Fatal("nil не должен считаться нарушением FK")
	}
	if isForeignKeyViolation(errors.New("простая ошибка")) {
		t.Fatal("прочая ошибка не должна считаться нарушением FK")
	}
	if isForeignKeyViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("нарушение UNIQUE (23505) не должно считаться нарушением FK")
	}
	if !isForeignKeyViolation(&pgconn.PgError{Code: "23503"}) {
		t.Fatal("pgcode 23503 должен распознаваться как нарушение FK")
	}
}
