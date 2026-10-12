package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestCORS проверяет матрицу: allowAll, точечный allowlist, preflight.
func TestCORS(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("allowAll ставит *", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "https://any.example.com")
		rec := httptest.NewRecorder()
		CORS([]string{"*"})(next).ServeHTTP(rec, req)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("ожидался *, получено %q", got)
		}
	})

	t.Run("allowlist: свой проходит, чужой нет", func(t *testing.T) {
		h := CORS([]string{"https://app.example.com"})(next)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
			t.Fatalf("свой origin должен отражаться, получено %q", got)
		}
		if got := rec.Header().Get("Vary"); got != "Origin" {
			t.Fatalf("ожидался Vary: Origin, получено %q", got)
		}

		req2 := httptest.NewRequest(http.MethodGet, "/", nil)
		req2.Header.Set("Origin", "https://evil.example.com")
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)
		if got := rec2.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("чужой origin не должен отражаться, получено %q", got)
		}
	})

	t.Run("preflight завершается 204", func(t *testing.T) {
		called := false
		blocker := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		})
		req := httptest.NewRequest(http.MethodOptions, "/posts", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := httptest.NewRecorder()
		CORS([]string{"https://app.example.com"})(blocker).ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent || called {
			t.Fatalf("preflight должен завершиться 204 без next: code=%d called=%v", rec.Code, called)
		}
		if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "DELETE") {
			t.Fatalf("методы должны включать DELETE, получено %q", got)
		}
	})
}

// TestTimeout_Expiry проверяет, что висящий хендлер обрывается по дедлайну.
func TestTimeout_Expiry(t *testing.T) {
	hang := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			w.WriteHeader(http.StatusServiceUnavailable)
		case <-time.After(5 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	Timeout(50*time.Millisecond)(hang).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ожидался обрыв по дедлайну (503 от хендлера), получено %d", rec.Code)
	}
}

// TestRecover_Panic проверяет преобразование паники в 500 с JSON-ошибкой.
func TestRecover_Panic(t *testing.T) {
	panicker := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	rec := httptest.NewRecorder()
	Recover(panicker).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("ожидался 500, получено %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "error") {
		t.Fatalf("тело должно быть JSON-ошибкой, получено %q", rec.Body.String())
	}
}

// TestRateLimiter проверяет всплеск и восстановление токенов.
func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(2)
	if !rl.Allow() || !rl.Allow() {
		t.Fatal("первые 2 запроса в пределах burst должны проходить")
	}
	if rl.Allow() {
		t.Fatal("третий запрос сверх burst должен отклоняться")
	}

	rl2 := NewRateLimiter(1)
	h := RateLimitMiddleware(rl2, 20*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("первый запрос должен пройти, получено %d", rec.Code)
	}
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("ожидался 429, получено %d", rec2.Code)
	}
	time.Sleep(60 * time.Millisecond)
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec3.Code != http.StatusOK {
		t.Fatalf("после refill запрос должен пройти, получено %d", rec3.Code)
	}
}

// TestGenerateRequestID_Unique проверяет случайность идентификаторов.
func TestGenerateRequestID_Unique(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		id := generateRequestID()
		if id == "" {
			t.Fatal("request id не должен быть пустым")
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("дубликат request id %q за 100 генераций", id)
		}
		seen[id] = struct{}{}
	}
}
