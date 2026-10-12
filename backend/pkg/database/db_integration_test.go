package database

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestRunMigrations_AppliesAndIdempotent поднимает чистый контейнер PostgreSQL,
// применяет боевые миграции через runMigrations (тот же путь, что и в проде) и
// проверяет, что схема создана, а повторный запуск идемпотентен.
//
// runMigrations использует относительный путь "pkg/migrations", поэтому тест
// переключает рабочий каталог на корень модуля (backend) через t.Chdir — он
// автоматически восстановит каталог после теста.
func TestRunMigrations_AppliesAndIdempotent(t *testing.T) {
	t.Chdir("../..") // backend/pkg/database -> backend, чтобы "pkg/migrations" резолвился

	ctx := context.Background()
	pg, err := tcpostgres.RunContainer(ctx,
		tcpostgres.WithDatabase("mig_test"),
		tcpostgres.WithUsername("tester"),
		tcpostgres.WithPassword("tester"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(2*time.Minute),
		),
	)
	if err != nil {
		t.Skipf("контейнер postgres недоступен, тест пропущен: %v", err)
	}
	defer func() { _ = pg.Terminate(ctx) }()

	connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("строка подключения: %v", err)
	}

	// Первичный накат миграций.
	if err := runMigrations(connStr); err != nil {
		t.Fatalf("runMigrations (первичный): %v", err)
	}

	// Повторный запуск должен быть идемпотентен (goose отслежит версии).
	if err := runMigrations(connStr); err != nil {
		t.Fatalf("runMigrations (повторный) должен быть идемпотентен: %v", err)
	}
}

// TestRunMigrations_BadConnStr проверяет, что на битую строку подключения
// runMigrations возвращает ошибку, а не падает.
func TestRunMigrations_BadConnStr(t *testing.T) {
	t.Chdir("../..")
	// Заведомо невалидная строка подключения.
	if err := runMigrations("postgres://bad:bad@127.0.0.1:1/none?sslmode=disable&connect_timeout=1"); err == nil {
		t.Fatal("ожидалась ошибка при некорректном подключении")
	}
}
