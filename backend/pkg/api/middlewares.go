package api

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"
)

// RequestIDKey — ключ контекста для сквозного идентификатора запроса.
const RequestIDKey contextKey = "requestID"

// statusRecorder оборачивает http.ResponseWriter, запоминая код статуса,
// чтобы AccessLogger мог его залогировать после выполнения хендлера.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

// Write нужен, чтобы залогировать неявный 200 OK, когда хендлер пишет тело
// без явного вызова WriteHeader.
func (rec *statusRecorder) Write(b []byte) (int, error) {
	if rec.status == 0 {
		rec.status = http.StatusOK
	}
	return rec.ResponseWriter.Write(b)
}

// Recover защищает сервер от падений: любая паника в хендлере перехватывается,
// логируется со стеком и преобразуется в ответ 500, вместо обвала всего процесса.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				reqID, _ := r.Context().Value(RequestIDKey).(string)
				slog.Error("паника в обработчике запроса",
					"panic", rec,
					"method", r.Method,
					"path", r.URL.Path,
					"request_id", reqID,
					"stack", string(debug.Stack()),
				)
				RespondWithError(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RequestID присваивает каждому запросу уникальный идентификатор. Если клиент
// прислал X-Request-ID — переиспользуем его, иначе генерируем новый. Значение
// добавляется в контекст и в ответный заголовок для трассировки.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateRequestID()
		}
		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AccessLogger логирует метод, путь, статус и длительность каждого запроса.
func AccessLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		reqID, _ := r.Context().Value(RequestIDKey).(string)
		slog.Info("http запрос",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", reqID,
			"remote_addr", r.RemoteAddr,
		)
	})
}

// SecurityHeaders выставляет базовые заголовки безопасности на каждый ответ.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// CORS реализует простую и предсказуемую поддержку кросс-доменных запросов.
// allowedOrigins — список разрешённых источников; "*" разрешает всех.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowAll := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" {
				if allowAll {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
			}

			// Preflight-запрос завершаем сразу. Список методов должен покрывать
			// все реально используемые API (включая DELETE для постов,
			// комментариев и лайков), иначе браузер заблокирует кросс-доменные
			// запросы ещё на preflight-этапе.
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Timeout ограничивает общее время обработки запроса. Предотвращает
// "зависшие" соединения, исчерпывающие ресурсы сервера.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RateLimiter — простой потокобезопасный ограничитель частоты запросов
// на основе алгоритма token bucket.
type RateLimiter struct {
	tokens chan struct{}
}

// NewRateLimiter создаёт ограничитель с bucket ёмкостью burst.
func NewRateLimiter(burst int) *RateLimiter {
	return &RateLimiter{tokens: make(chan struct{}, burst)}
}

// Allow пытается занять один токен. Возвращает false, если лимит исчерпан.
func (rl *RateLimiter) Allow() bool {
	select {
	case rl.tokens <- struct{}{}:
		return true
	default:
		return false
	}
}

// refill периодически освобождает один токен, задавая среднюю пропускную
// способность "один запрос каждые interval".
func (rl *RateLimiter) refill(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			select {
			case <-rl.tokens:
			default:
			}
		}
	}()
}

// RateLimitMiddleware оборачивает хендлер ограничением частоты. Отдаёт 429 при
// превышении. burst — размер всплеска, interval — период восстановления токена.
func RateLimitMiddleware(rl *RateLimiter, interval time.Duration) func(http.Handler) http.Handler {
	rl.refill(interval)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.Allow() {
				RespondWithError(w, http.StatusTooManyRequests, "Слишком много запросов, попробуйте позже")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// generateRequestID создаёт короткий уникальный идентификатор запроса.
func generateRequestID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
