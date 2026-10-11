// Package api — главный роутер CFU-Board.
//
// NewRouter связывает все хендлеры (auth, users, posts, comments, likes,
// health, docs) с путями из docs/openapi.yaml и навешивает глобальные
// middleware (Recover, RequestID, AccessLogger, SecurityHeaders, CORS,
// Timeout) + точечные Auth/OptionalAuth и RateLimit для auth-эндпоинтов.
package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"cfu-board/pkg/config"
	"cfu-board/pkg/service"
	"cfu-board/pkg/storage"
)

// chain оборачивает h middleware в порядке перечисления:
// chain(h, A, B, C) => A(B(C(h))).
func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// NewRouter собирает http.ServeMux со всеми роутами и возвращает готовый
// http.Handler с глобальными middleware.
//
// Сигнатура зафиксирована вызовом из cmd/app/main.go:
//
//	handler := api.NewRouter(authService, []byte(cfg.JWT.Secret), database.GlobalPool, cfg, store)
func NewRouter(
	authService *service.AuthService,
	jwtSecret []byte,
	pool *pgxpool.Pool,
	cfg *config.Config,
	store storage.Storage,
) http.Handler {
	mux := http.NewServeMux()

	auth := AuthMiddleware(jwtSecret)
	optionalAuth := OptionalAuthMiddleware(jwtSecret)

	// Rate-limiter для auth-эндпоинтов: всплеск 20, восстановление
	// 1 токен/сек. Отдельный инстанс, чтобы лимит логина/регистрации
	// не съедал лимит остальных роутов.
	authLimiter := NewRateLimiter(20)
	authRateLimit := RateLimitMiddleware(authLimiter, time.Second)

	// --- Auth: регистрация, вход, токены, подтверждение email ---
	mux.Handle("POST /register", chain(
		RegisterHandler(authService),
		authRateLimit,
	))
	mux.Handle("POST /login", chain(
		LoginHandler(authService),
		authRateLimit,
	))
	mux.Handle("POST /refresh", chain(
		RefreshHandler(authService),
		authRateLimit,
	))
	mux.Handle("POST /logout", LogoutHandler(authService))
	mux.Handle("GET /verify", chain(
		VerifyHandler(authService),
		authRateLimit,
	))
	mux.Handle("POST /resend-verification", chain(
		ResendVerificationHandler(authService),
		authRateLimit,
	))

	// --- Users: профили ---
	// Важно: "GET /users/me" регистрируем раньше/наряду с "GET /users/{id}".
	// ServeMux Go 1.22+ отдаёт предпочтение более специфичному шаблону,
	// поэтому /users/me не будет проглочен wildcard-ом {id}.
	mux.Handle("GET /users/me", chain(
		GetMyProfileHandler(),
		auth,
	))
	mux.Handle("GET /users/{id}", GetUserProfileHandler())

	// --- Posts: лента, создание, просмотр, удаление ---
	// Лента публичная, с опциональной персонализацией liked_by_me.
	// Важно: "GET /posts" регистрируем до "GET /posts/{id}" — иначе mux
	// попытается распарсить "posts" как {id} (получим 400 вместо ленты).
	mux.Handle("GET /posts", chain(
		ListPostsHandler(),
		optionalAuth,
	))
	mux.Handle("POST /posts/create", chain(
		CreatePostMultipartHandler(store),
		auth,
	))
	// Публичный просмотр с опциональной персонализацией liked_by_me.
	mux.Handle("GET /posts/{id}", chain(
		GetPostHandler(),
		optionalAuth,
	))
	mux.Handle("DELETE /posts/{id}", chain(
		DeletePostHandler(store),
		auth,
	))

	// --- Likes ---
	mux.Handle("POST /posts/{id}/likes", chain(
		LikePostHandler(),
		auth,
	))
	mux.Handle("DELETE /posts/{id}/likes", chain(
		UnlikePostHandler(),
		auth,
	))

	// --- Comments ---
	// Публичный список + защищённое создание на одном пути,
	// различаются только методом. Список с опциональной персонализацией
	// liked_by_me (как у GET /posts/{id}).
	mux.Handle("GET /posts/{id}/comments", chain(
		ListCommentsHandler(),
		optionalAuth,
	))
	mux.Handle("POST /posts/{id}/comments", chain(
		CreateCommentHandler(),
		auth,
	))
	mux.Handle("DELETE /posts/{id}/comments/{commentID}", chain(
		DeleteCommentHandler(),
		auth,
	))
	mux.Handle("POST /posts/{id}/comments/{commentID}/likes", chain(
		LikeCommentHandler(),
		auth,
	))
	mux.Handle("DELETE /posts/{id}/comments/{commentID}/likes", chain(
		UnlikeCommentHandler(),
		auth,
	))

	// --- Health ---
	mux.HandleFunc("GET /healthz", HealthHandler)
	mux.Handle("GET /readyz", ReadinessHandler(pool))

	// --- Docs (Redoc + сырая спека) ---
	mux.HandleFunc("GET /docs", DocsHandler)
	mux.HandleFunc("GET /docs/openapi.yaml", OpenAPIYAMLHandler)

	// --- Глобальные middleware ---
	// Порядок (снаружи внутрь):
	// Recover -> RequestID -> AccessLogger -> SecurityHeaders -> CORS -> Timeout -> mux.
	// Recover первым, чтобы ловить паники всех нижележащих слоёв.
	requestTimeout := 60 * time.Second
	allowedOrigins := []string{"*"}
	if cfg != nil {
		if cfg.HTTP.RequestTimeout > 0 {
			requestTimeout = cfg.HTTP.RequestTimeout
		}
		if len(cfg.HTTP.AllowedOrigins) > 0 {
			allowedOrigins = cfg.HTTP.AllowedOrigins
		}
	}

	return chain(
		mux,
		Recover,
		RequestID,
		AccessLogger,
		SecurityHeaders,
		CORS(allowedOrigins),
		Timeout(requestTimeout),
	)
}
