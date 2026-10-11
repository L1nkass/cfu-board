package database

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"cfu-board/pkg/config"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// GlobalPool — глобальный пул соединений, к которому обращаются другие пакеты (api).
var GlobalPool *pgxpool.Pool

// InitDB инициализирует пул соединений с PostgreSQL на основе переданной
// конфигурации. При критических ошибках — падает (fail-fast). Возвращает
// функцию-клинер для вежливого закрытия пула при выключении сервера.
func InitDB(cfg *config.Config) func() {
	// Контекст с таймаутом на подключение.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(cfg.DB.URL)
	if err != nil {
		slog.Error("не удалось распарсить строку подключения к БД", "error", err)
		panic(err)
	}

	// Настройки пула берём из конфигурации.
	poolConfig.MaxConns = cfg.DB.MaxConns
	poolConfig.MinConns = cfg.DB.MinConns
	poolConfig.MaxConnIdleTime = cfg.DB.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		slog.Error("невозможно создать пул соединений", "error", err)
		panic(err)
	}

	// Обязательный Ping для проверки доступности СУБД.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		slog.Error("база данных недоступна (Ping провален)", "error", err)
		panic(err)
	}

	slog.Info("пул соединений с PostgreSQL инициализирован",
		"max_conns", cfg.DB.MaxConns,
		"min_conns", cfg.DB.MinConns,
	)

	// Запускаем миграции, если это разрешено конфигурацией.
	if cfg.DB.AutoMigrate {
		if err := runMigrations(cfg.DB.URL); err != nil {
			pool.Close()
			panic(err)
		}
	} else {
		slog.Info("автомиграции отключены (DB_AUTO_MIGRATE=false)")
	}

	GlobalPool = pool

	return func() {
		pool.Close()
		slog.Info("пул соединений с PostgreSQL закрыт")
	}
}

// runMigrations накатывает миграции goose. Открывает отдельное sql-подключение
// через встроенный pgx stdlib-драйвер (goose работает с database/sql).
func runMigrations(connStr string) error {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		slog.Error("не удалось открыть соединение для миграций", "error", err)
		return err
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		slog.Error("не удалось установить диалект goose", "error", err)
		return err
	}

	slog.Info("проверка и накат миграций базы данных...")
	if err := goose.Up(db, "pkg/migrations"); err != nil {
		slog.Error("ошибка при накате миграций goose", "error", err)
		return err
	}
	slog.Info("миграции применены успешно")
	return nil
}
