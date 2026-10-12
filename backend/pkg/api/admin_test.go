package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"cfu-board/pkg/database"
	"cfu-board/pkg/models"
	"cfu-board/pkg/repository"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestAdminHandlers_NoAuth проверяет, что админ-эндпоинты без пользователя в
// контексте (middleware не отработал) сразу отдают 401 — ДО любого обращения
// к БД. Поэтому тест не требует поднятого окружения. Каждому хендлеру шлём его
// валидный HTTP-метод, чтобы дойти именно до проверки авторизации.
func TestAdminHandlers_NoAuth(t *testing.T) {
	handlers := map[string]struct {
		h      http.HandlerFunc
		method string
	}{
		"pending": {AdminListPendingHandler(), http.MethodGet},
		"approve": {AdminModeratePostHandler("approved"), http.MethodPost},
		"reject":  {AdminModeratePostHandler("rejected"), http.MethodPost},
	}
	for name, hc := range handlers {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(hc.method, "/admin/posts/1/approve", nil)
			// В контексте нет UserIDKey — эмулируем «middleware не сработал».
			hc.h(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("ожидался 401 без пользователя в контексте, получено %d", rec.Code)
			}
		})
	}
}

// TestAdminListPending_MethodNotAllowed — не-GET метод к очереди модерации
// отклоняется. Пользователь в контексте подставлен вручную (мимо БД), чтобы
// дойти до проверки метода.
func TestAdminListPending_MethodNotAllowed(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/posts/pending", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, 1))
	AdminListPendingHandler()(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("ожидался 405, получено %d", rec.Code)
	}
}

// --- Интеграционные тесты админ-панели (testcontainers) ---
// Хендлеры модерации обращаются к database.GlobalPool (глобальный пул, как в
// проде). Здесь на время теста подменяем его на пул к одноразовому контейнеру
// PostgreSQL с боевыми миграциями и создаём реальных пользователей (админ и
// обычного), проверяя контроль доступа и сам акт одобрения/отклонения.

var (
	adminPoolOnce sync.Once
	adminPool     *pgxpool.Pool
	adminSetupErr error
)

func adminTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	adminPoolOnce.Do(func() {
		ctx := context.Background()
		pg, err := tcpostgres.RunContainer(ctx,
			tcpostgres.WithDatabase("admin_test"),
			tcpostgres.WithUsername("tester"),
			tcpostgres.WithPassword("tester"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).WithStartupTimeout(2*time.Minute),
			),
		)
		if err != nil {
			adminSetupErr = err
			return
		}
		connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			adminSetupErr = err
			return
		}
		sdb, err := sql.Open("pgx", connStr)
		if err != nil {
			adminSetupErr = err
			return
		}
		defer sdb.Close()
		_ = goose.SetDialect("postgres")
		if err := goose.Up(sdb, "../migrations"); err != nil {
			adminSetupErr = err
			return
		}
		adminPool, err = pgxpool.New(ctx, connStr)
		if err != nil {
			adminSetupErr = err
		}
	})
	if adminSetupErr != nil {
		t.Skipf("интеграционное окружение админки недоступно, тест пропущен: %v", adminSetupErr)
	}
	return adminPool
}

// seedAdminUser создаёт пользователя с заданным флагом админа и возвращает ID.
func seedAdminUser(t *testing.T, pool *pgxpool.Pool, username string, admin bool) int {
	t.Helper()
	repo := repository.NewUserRepository(pool)
	u := &models.User{Username: username, Email: username + "@example.com", PasswordHash: "x", IsAdmin: admin}
	if err := repo.Create(context.Background(), u); err != nil {
		t.Fatalf("seedAdminUser(%q): %v", username, err)
	}
	// is_admin не выставляется через Create (там дефолт false) — правим напрямую.
	if admin {
		if _, err := pool.Exec(context.Background(), `UPDATE users SET is_admin = TRUE WHERE id = $1`, u.ID); err != nil {
			t.Fatalf("seedAdminUser set admin: %v", err)
		}
	}
	return u.ID
}

// TestAdminModeration_AccessControl проверяет сквозной флоу модерации через
// HTTP-хендлеры: не-админ получает 403, админ одобряет пост (200), и тот
// становится видимым в ленте; модерация несуществующего поста — 404.
func TestAdminModeration_AccessControl(t *testing.T) {
	pool := adminTestPool(t)

	// Изолируем БД и подменяем глобальный пул на время теста.
	if _, err := pool.Exec(context.Background(),
		`TRUNCATE users, posts, comments, post_likes, comment_likes,
		 refresh_tokens, email_verifications RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	prevPool := database.GlobalPool
	database.GlobalPool = pool
	t.Cleanup(func() { database.GlobalPool = prevPool })

	repo := repository.NewUserRepository(pool)
	authorID := seedAdminUser(t, pool, "postAuthor", false)
	adminID := seedAdminUser(t, pool, "theAdmin", true)
	regularID := seedAdminUser(t, pool, "regularJoe", false)

	// Автор создаёт пост — он уходит на модерацию (pending).
	postID, err := repo.CreatePostWithMedia(context.Background(), authorID, "Ждёт одобрения", "текст", "", "")
	if err != nil {
		t.Fatalf("CreatePostWithMedia: %v", err)
	}

	// Обычный пользователь НЕ может модерировать → 403.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/posts/1/approve", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, regularID))
	AdminModeratePostHandler(models.PostStatusApproved)(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("не-админ: ожидался 403, получено %d", rec.Code)
	}

	// Пост всё ещё pending и скрыт из ленты.
	if feed, _ := repo.ListPostsWithLikes(context.Background(), 0, 20, 0); len(feed) != 0 {
		t.Fatalf("после отказа не-админа пост должен оставаться скрытым, найдено %d", len(feed))
	}

	// Модерация несуществующего поста админом → 404.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/admin/posts/999999/approve", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, adminID))
	req.SetPathValue("id", "999999")
	AdminModeratePostHandler(models.PostStatusApproved)(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("несуществующий пост: ожидался 404, получено %d", rec.Code)
	}

	// Админ одобряет реальный пост → 200.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/admin/posts/approve", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, adminID))
	req.SetPathValue("id", strconv.Itoa(postID))
	AdminModeratePostHandler(models.PostStatusApproved)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("админ одобряет: ожидался 200, получено %d", rec.Code)
	}

	// Пост стал видимым в общей ленте со статусом approved.
	feed, err := repo.ListPostsWithLikes(context.Background(), 0, 20, 0)
	if err != nil {
		t.Fatalf("ListPostsWithLikes: %v", err)
	}
	if len(feed) != 1 || feed[0].ID != postID || feed[0].Status != models.PostStatusApproved {
		t.Fatalf("после одобрения пост должен быть в ленте: %+v", feed)
	}
}
