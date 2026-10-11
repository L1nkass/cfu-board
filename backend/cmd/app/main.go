// Command cfu-board — точка входа сервера соцсети CFU-Board.
//
// Отвечает за оркестрацию запуска: загрузку и валидацию конфигурации,
// инициализацию логгера, подключение к БД с миграциями, сборку зависимостей
// (Dependency Injection) и вежливое завершение работы (graceful shutdown).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cfu-board/pkg/api"
	"cfu-board/pkg/config"
	"cfu-board/pkg/database"
	"cfu-board/pkg/logger"
	"cfu-board/pkg/repository"
	"cfu-board/pkg/service"
	"cfu-board/pkg/storage"

	"github.com/joho/godotenv"
)

func main() {
	// Загружаем .env, если он присутствует (в Docker переменные приходят извне).
	if err := godotenv.Load(); err != nil {
		// До инициализации логгера выводим через slog по умолчанию.
		slog.Info("файл .env не найден, используются переменные окружения системы")
	}

	// 1. Конфигурация. Валидация здесь обеспечивает fail-fast: при неверном
	// окружении приложение не запустится и явно укажет причину.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("ошибка конфигурации", "error", err)
		os.Exit(1)
	}

	// 2. Логгер (структурный; JSON в prod, текст в dev).
	log := logger.New(cfg.Env)

	// 3. Инициализация пула БД и накат миграций.
	closeDB := database.InitDB(cfg)
	defer closeDB()

	// 4. Сборка зависимостей (Dependency Injection).
	userRepo := repository.NewUserRepository(database.GlobalPool)
	refreshRepo := repository.NewRefreshTokenRepository(database.GlobalPool)
	verifRepo := repository.NewEmailVerificationRepository(database.GlobalPool)
	mailService := service.NewMailService(cfg.SMTP)
	authService := service.NewAuthService(userRepo, refreshRepo, verifRepo, mailService, cfg)

	// 4.1 Инициализация хранилища медиафайлов (локальный диск или MinIO) и
	// его подготовка (создание каталога/бакета + политика публичного чтения).
	store, err := storage.New(cfg.Storage)
	if err != nil {
		log.Error("ошибка инициализации хранилища", "error", err)
		os.Exit(1)
	}
	// Подготовку хранилища оборачиваем в повторные попытки: при драйвере
	// minio контейнер приложения и контейнер MinIO стартуют одновременно, и
	// на момент первого обращения S3-API может быть ещё не готов. Ждём его
	// с мягким бэкоффом, чтобы не падать из-за состояния гонки запуска.
	if err := ensureStorageWithRetry(context.Background(), store, log); err != nil {
		log.Error("не удалось подготовить хранилище медиафайлов", "driver", store.Driver(), "error", err)
		os.Exit(1)
	}
	log.Info("хранилище медиафайлов готово", "driver", store.Driver())

	// 5. Сборка HTTP-обработчика со всеми middleware и роутами.
	handler := api.NewRouter(authService, []byte(cfg.JWT.Secret), database.GlobalPool, cfg, store)

	// 6. HTTP-сервер с корректными таймаутами.
	// ReadHeaderTimeout защищает от Slowloris, IdleTimeout — от простой.
	// Жёстких Read/Write таймаутов нет: пользователи загружают тяжёлые видео.
	srv := &http.Server{
		Addr:              cfg.HTTP.Host + ":" + cfg.HTTP.Port,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	// Запускаем сервер в отдельной горутине, чтобы главный поток мог
	// дождаться сигнала завершения.
	serverErr := make(chan error, 1)
	go func() {
		log.Info("сервер cfu-board запущен",
			"env", cfg.Env,
			"addr", srv.Addr,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// 7. Ожидаем сигнал прерывания (Ctrl+C / SIGTERM) для graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		log.Error("критический сбой сервера", "error", err)
		os.Exit(1)

	case <-ctx.Done():
		log.Info("получен сигнал завершения, гасим сервер...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("ошибка при graceful shutdown", "error", err)
		}
		log.Info("сервер корректно остановлен")
	}
}

// ensureStorageWithRetry вызывает store.Ensure с повторными попытками и мягким
// бэкоффом. Нужно для драйвера minio: его контейнер стартует одновременно с
// приложением, поэтому S3-API может быть временно недоступен. Общее окно
// ожидания ограничено (60с), чтобы не зависать навсегда при реальной ошибке.
func ensureStorageWithRetry(ctx context.Context, store storage.Storage, log *slog.Logger) error {
	const (
		attempts = 10               // число попыток
		backoff  = 2 * time.Second  // пауза между попытками
		perCall  = 10 * time.Second // таймаут одной попытки
	)
	var lastErr error
	for i := 1; i <= attempts; i++ {
		callCtx, cancel := context.WithTimeout(ctx, perCall)
		lastErr = store.Ensure(callCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		log.Warn("хранилище ещё не готово, повторная попытка",
			"driver", store.Driver(), "attempt", i, "error", lastErr)
		if i < attempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return lastErr
}
