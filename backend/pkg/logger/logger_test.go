package logger

import (
	"log/slog"
	"testing"

	"cfu-board/pkg/config"
)

// TestNew_Levels проверяет уровни и формат по окружению: dev — debug+text,
// prod — info+json. Логгер также становится slog.Default.
func TestNew_Levels(t *testing.T) {
	dev := New(config.EnvDevelopment)
	if !dev.Enabled(nil, slog.LevelDebug) {
		t.Fatal("dev-логгер должен писать debug")
	}

	prod := New(config.EnvProduction)
	if prod.Enabled(nil, slog.LevelDebug) {
		t.Fatal("prod-логгер не должен писать debug")
	}
	if !prod.Enabled(nil, slog.LevelInfo) {
		t.Fatal("prod-логгер должен писать info")
	}
	if slog.Default() == nil {
		t.Fatal("slog.Default должен быть установлен")
	}
}
