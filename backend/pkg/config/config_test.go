package config

import "testing"

// TestLoad_Validation проверяет, что fail-fast валидация конфигурации
// корректно отклоняет невалидное окружение.
func TestLoad_Validation(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")

	// Слишком короткий секрет для production.
	t.Setenv("JWT_SECRET", "short")

	if _, err := Load(); err == nil {
		t.Fatal("ожидалась ошибка валидации для короткого JWT_SECRET в production")
	}

	// Достаточно длинный секрет — валидация проходит.
	t.Setenv("JWT_SECRET", "this-is-a-sufficiently-long-secret-key-0123456789")
	// Минимально необходимая конфигурация хранилища MinIO.
	t.Setenv("STORAGE_DRIVER", "minio")
	t.Setenv("MINIO_ENDPOINT", "localhost:9000")
	t.Setenv("MINIO_ACCESS_KEY", "minioadmin")
	t.Setenv("MINIO_SECRET_KEY", "minioadmin")
	t.Setenv("MINIO_BUCKET", "cfu-board")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("не ожидалась ошибка валидации: %v", err)
	}
	if !cfg.IsProduction() {
		t.Fatal("ожидался режим production")
	}
	if cfg.PublicBaseURL == "" {
		t.Fatal("PublicBaseURL должен быть заполнен")
	}
}

// TestLoad_StorageValidation проверяет обязательность и полноту конфигурации
// MinIO (единственный поддерживаемый драйвер хранилища).
func TestLoad_StorageValidation(t *testing.T) {
	// Базовое валидное окружение.
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("JWT_SECRET", "dev-secret")
	t.Setenv("MINIO_ENDPOINT", "localhost:9000")
	t.Setenv("MINIO_ACCESS_KEY", "minioadmin")
	t.Setenv("MINIO_SECRET_KEY", "minioadmin")
	t.Setenv("MINIO_BUCKET", "cfu-board")

	// Полная конфигурация minio — проходит.
	if _, err := Load(); err != nil {
		t.Fatalf("не ожидалась ошибка при полной конфигурации MinIO: %v", err)
	}

	// Пустой endpoint minio — ошибка.
	t.Setenv("MINIO_ENDPOINT", "")
	if _, err := Load(); err == nil {
		t.Fatal("ожидалась ошибка при пустом MINIO_ENDPOINT")
	}

	// Неизвестный драйвер (в т.ч. прежний "local") — ошибка.
	t.Setenv("MINIO_ENDPOINT", "localhost:9000")
	t.Setenv("STORAGE_DRIVER", "local")
	if _, err := Load(); err == nil {
		t.Fatal("ожидалась ошибка для неизвестного/устаревшего драйвера хранилища")
	}
}

// TestLoad_MissingRequired проверяет реакцию на отсутствие обязательных полей.
func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")

	if _, err := Load(); err == nil {
		t.Fatal("ожидалась ошибка при пустом DATABASE_URL/JWT_SECRET")
	}
}
