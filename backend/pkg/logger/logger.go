// Package logger настраивает структурное логирование приложения на базе
// стандартного пакета log/slog. В production используется JSON (удобно для
// сбора логов), в development — человекочитаемый текстовый формат.
package logger

import (
	"log/slog"
	"os"

	"cfu-board/pkg/config"
)

// New создаёт и возвращает настроенный логгер в зависимости от окружения.
func New(env config.Environment) *slog.Logger {
	level := slog.LevelDebug
	if env == config.EnvProduction {
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if env == config.EnvProduction {
		// JSON для машинного парсинга в проде.
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		// Читаемый текст для локальной разработки.
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	// Делаем логгер логгером по умолчанию, чтобы slog.Info/... во всех
	// пакетах писали через нашу единую конфигурацию.
	slog.SetDefault(logger)
	return logger
}
