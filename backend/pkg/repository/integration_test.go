package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"cfu-board/pkg/models"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/jackc/pgx/v5/stdlib" // драйвер "pgx" для database/sql (goose)
)

// Интеграционные тесты репозиториев поднимают одноразовый контейнер PostgreSQL
// (testcontainers) и накатывают на него боевые миграции goose из ../migrations —
// так тесты проверяют реальный SQL против реальной СУБД, а не моки. Если Docker
// недоступен, вся группа тестов аккуратно пропускается (t.Skip), чтобы
// `go test ./...` оставался зелёным в окружениях без демона.
var (
	poolOnce   sync.Once
	sharedPool *pgxpool.Pool
	container  testcontainers.Container
	setupErr   error
)

// TestMain поднимает окружение один раз на весь пакет и корректно освобождает
// ресурсы контейнера после прогона.
func TestMain(m *testing.M) {
	code := m.Run()
	if container != nil {
		_ = container.Terminate(context.Background())
	}
	if sharedPool != nil {
		sharedPool.Close()
	}
	os.Exit(code)
}

// setupIntegration лениво (один раз) создаёт контейнер, миграции и пул.
func setupIntegration(ctx context.Context) error {
	pgContainer, err := tcpostgres.RunContainer(ctx,
		tcpostgres.WithDatabase("cfu_test"),
		tcpostgres.WithUsername("tester"),
		tcpostgres.WithPassword("tester"),
		// Ждём, пока сервер реально начнёт принимать соединения (порт может
		// открыться раньше готовности БД).
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(2*time.Minute),
		),
	)
	if err != nil {
		return fmt.Errorf("не удалось запустить контейнер postgres: %w", err)
	}
	container = pgContainer

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fmt.Errorf("не удалось получить строку подключения: %w", err)
	}

	// Накатываем боевые миграции через goose (как в продакшене).
	if err := runTestMigrations(connStr); err != nil {
		return fmt.Errorf("не удалось применить миграции: %w", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return fmt.Errorf("не удалось создать пул: %w", err)
	}
	sharedPool = pool
	return nil
}

// runTestMigrations применяет миграции goose из каталога ../migrations (относительно
// рабочего каталога теста — пакета pkg/repository).
func runTestMigrations(connStr string) error {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, "../migrations")
}

// requirePool возвращает общий пул либо пропускает тест, если окружение
// поднять не удалось (нет Docker и т.п.).

// truncateAll очищает все таблицы между тестами, чтобы каждый стартовал с
// чистого листа (изоляция данных).
func truncateAll(t *testing.T) {
	t.Helper()
	pool := requirePool(t)
	_, err := pool.Exec(context.Background(),
		`TRUNCATE TABLE users, posts, comments, post_likes, comment_likes,
		 refresh_tokens, email_verifications RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncateAll: %v", err)
	}
}

// seedUser создаёт пользователя-фикстуру и возвращает его с заполненным ID.
func seedUser(t *testing.T, repo *UserRepository, username, email string) *models.User {
	t.Helper()
	u := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: "hash_" + username,
	}
	if err := repo.Create(context.Background(), u); err != nil {
		t.Fatalf("seedUser(%q): %v", username, err)
	}
	return u
}

// seedPost создаёт пост-фикстуру, сразу одобренный (approved), чтобы проверки
// ленты/видимости работали «как в бою» после модерации. Возвращает ID поста.
func seedPost(t *testing.T, pool *pgxpool.Pool, authorID int, title string) int {
	t.Helper()
	repo := NewUserRepository(pool)
	id, err := repo.CreatePostWithMedia(context.Background(), authorID, title,
		"content-"+title, "", "")
	if err != nil {
		t.Fatalf("seedPost(%q): %v", title, err)
	}
	if _, err := repo.SetPostModeration(context.Background(), id, models.PostStatusApproved, authorID, ""); err != nil {
		t.Fatalf("seedPost(%q) approve: %v", title, err)
	}
	return id
}

// lastPostID возвращает ID последнего созданного поста (для быстрых фикстур).
func lastPostID(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var id int
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM posts ORDER BY id DESC LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("lastPostID: %v", err)
	}
	return id
}

func requirePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	poolOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		setupErr = setupIntegration(ctx)
	})
	if setupErr != nil {
		t.Skipf("интеграционное окружение недоступно, тест пропущен: %v", setupErr)
	}
	return sharedPool
}
