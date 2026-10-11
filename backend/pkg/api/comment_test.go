package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCreateCommentHandler_BadPostID — некорректный ID поста в пути возвращает 400
// до обращения к БД и без токена.
func TestCreateCommentHandler_BadPostID(t *testing.T) {
	for _, id := range []string{"abc", "0", "-5"} {
		req := httptest.NewRequest(http.MethodPost, "/posts/"+id+"/comments", nil)
		req.SetPathValue("id", id)
		rec := httptest.NewRecorder()

		CreateCommentHandler()(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("ожидался 400 для id=%q, получено %d", id, rec.Code)
		}
	}
}

// TestCreateCommentHandler_NoAuth — без контекста авторизации (middleware не
// отработал) валидный ID поста даёт 401 до обращения к БД.
func TestCreateCommentHandler_NoAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/posts/1/comments", strings.NewReader(`{"content":"hi"}`))
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	CreateCommentHandler()(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ожидался 401 без контекста авторизации, получено %d", rec.Code)
	}
}

// TestCreateCommentHandler_MethodNotAllowed — не-POST метод отклоняется.
func TestCreateCommentHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/posts/1/comments", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	CreateCommentHandler()(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("ожидался 405, получено %d", rec.Code)
	}
}

// TestListCommentsHandler_BadPostID — публичный список: невалидный ID -> 400,
// невалидный limit -> 400 (оба без БД).
func TestListCommentsHandler_BadPostID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/posts/abc/comments", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	ListCommentsHandler()(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидался 400 для некорректного id, получено %d", rec.Code)
	}
}

func TestListCommentsHandler_BadLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/posts/1/comments?limit=999", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	ListCommentsHandler()(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидался 400 для limit=999, получено %d", rec.Code)
	}
}

// TestDeleteCommentHandler_Validation — невалидные ID и отсутствие авторизации
// отклоняются до обращения к БД.
func TestDeleteCommentHandler_Validation(t *testing.T) {
	t.Run("некорректный id поста", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/posts/0/comments/1", nil)
		req.SetPathValue("id", "0")
		req.SetPathValue("commentID", "1")
		rec := httptest.NewRecorder()

		DeleteCommentHandler()(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("ожидался 400, получено %d", rec.Code)
		}
	})

	t.Run("некорректный id комментария", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/posts/1/comments/xyz", nil)
		req.SetPathValue("id", "1")
		req.SetPathValue("commentID", "xyz")
		rec := httptest.NewRecorder()

		DeleteCommentHandler()(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("ожидался 400, получено %d", rec.Code)
		}
	})

	t.Run("без авторизации", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/posts/1/comments/1", nil)
		req.SetPathValue("id", "1")
		req.SetPathValue("commentID", "1")
		rec := httptest.NewRecorder()

		DeleteCommentHandler()(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("ожидался 401, получено %d", rec.Code)
		}
	})
}

// TestCreateCommentHandler_EmptyBody — пустое/невалидное JSON тело -> 400.
// Авторизация подставляется в контекст вручную, минуя middleware.
func TestCreateCommentHandler_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/posts/1/comments", strings.NewReader(`{`))
	req.SetPathValue("id", "1")
	req = req.WithContext(withUserID(req, 1))
	rec := httptest.NewRecorder()

	CreateCommentHandler()(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидался 400 для невалидного JSON, получено %d", rec.Code)
	}
}

// TestCreateCommentHandler_BadParentID — невалидные значения parent_id
// отклоняются до обращения к БД.
func TestCreateCommentHandler_BadParentID(t *testing.T) {
	for _, body := range []string{
		`{"content":"hi","parent_id":0}`,
		`{"content":"hi","parent_id":-1}`,
		`{"content":"hi","parent_id":"abc"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/posts/1/comments", strings.NewReader(body))
		req.SetPathValue("id", "1")
		req = req.WithContext(withUserID(req, 1))
		rec := httptest.NewRecorder()

		CreateCommentHandler()(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("ожидался 400 для тела %q, получено %d (тело: %s)", body, rec.Code, rec.Body.String())
		}
	}
}

// TestLikeCommentHandler_Validation — валидация лайка комментария до обращения к БД.
func TestLikeCommentHandler_Validation(t *testing.T) {
	t.Run("некорректный id поста", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/posts/abc/comments/1/likes", nil)
		req.SetPathValue("id", "abc")
		req.SetPathValue("commentID", "1")
		req = req.WithContext(withUserID(req, 1))
		rec := httptest.NewRecorder()

		LikeCommentHandler()(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("ожидался 400 для некорректного id поста, получено %d", rec.Code)
		}
	})

	t.Run("некорректный id комментария", func(t *testing.T) {
		for _, id := range []string{"xyz", "0", "-3"} {
			req := httptest.NewRequest(http.MethodPost, "/posts/1/comments/"+id+"/likes", nil)
			req.SetPathValue("id", "1")
			req.SetPathValue("commentID", id)
			req = req.WithContext(withUserID(req, 1))
			rec := httptest.NewRecorder()

			LikeCommentHandler()(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("ожидался 400 для commentID=%q, получено %d", id, rec.Code)
			}
		}
	})

	t.Run("без авторизации", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/posts/1/comments/2/likes", nil)
		req.SetPathValue("id", "1")
		req.SetPathValue("commentID", "2")
		rec := httptest.NewRecorder()

		LikeCommentHandler()(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("ожидался 401 без контекста авторизации, получено %d", rec.Code)
		}
	})

	t.Run("неверный метод", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/posts/1/comments/2/likes", nil)
		req.SetPathValue("id", "1")
		req.SetPathValue("commentID", "2")
		req = req.WithContext(withUserID(req, 1))
		rec := httptest.NewRecorder()

		LikeCommentHandler()(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("ожидался 405, получено %d", rec.Code)
		}
	})
}

// TestUnlikeCommentHandler_Validation — валидация снятия лайка комментария.
func TestUnlikeCommentHandler_Validation(t *testing.T) {
	t.Run("некорректный id комментария", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/posts/1/comments/xyz/likes", nil)
		req.SetPathValue("id", "1")
		req.SetPathValue("commentID", "xyz")
		req = req.WithContext(withUserID(req, 1))
		rec := httptest.NewRecorder()

		UnlikeCommentHandler()(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("ожидался 400 для некорректного id, получено %d", rec.Code)
		}
	})

	t.Run("без авторизации", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/posts/1/comments/2/likes", nil)
		req.SetPathValue("id", "1")
		req.SetPathValue("commentID", "2")
		rec := httptest.NewRecorder()

		UnlikeCommentHandler()(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("ожидался 401 без контекста авторизации, получено %d", rec.Code)
		}
	})

	t.Run("неверный метод", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/posts/1/comments/2/likes", nil)
		req.SetPathValue("id", "1")
		req.SetPathValue("commentID", "2")
		req = req.WithContext(withUserID(req, 1))
		rec := httptest.NewRecorder()

		UnlikeCommentHandler()(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("ожидался 405, получено %d", rec.Code)
		}
	})
}

// withUserID подставляет userID в контекст запроса тем же ключом, что и middleware.
func withUserID(req *http.Request, id int) context.Context {
	return context.WithValue(req.Context(), UserIDKey, id)
}
