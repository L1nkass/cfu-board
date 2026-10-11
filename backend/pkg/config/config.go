// Package config централизует загрузку и валидацию конфигурации приложения
// из переменных окружения. Позволяет "упасть" ещё на старте (fail-fast),
// если окружение настроено некорректно, а не ловить баги в рантайме.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment — режим работы приложения.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"
)

// Config — корневая структура конфигурации приложения.
type Config struct {
	Env          Environment
	HTTP         HTTPConfig
	DB           DBConfig
	JWT          JWTConfig
	Verification VerificationConfig
	SMTP         SMTPConfig
	Storage      StorageConfig
	// PublicBaseURL — внешний адрес сервиса (схема+хост), используется для
	// генерации абсолютных ссылок (например, в письмах верификации).
	// За реверс-прокси отличается от слушающего адреса HTTP.
	PublicBaseURL string
}

// HTTPConfig — настройки HTTP-сервера.
type HTTPConfig struct {
	Host              string
	Port              string
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	RequestTimeout    time.Duration
	AllowedOrigins    []string
}

// DBConfig — настройки пула соединений с PostgreSQL.
type DBConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnIdleTime time.Duration
	AutoMigrate     bool
}

// JWTConfig — настройки подписи токенов.
type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration // короткоживущий access-токен (JWT)
	RefreshTTL time.Duration // долгоживущий refresh-токен (хранится в БД)
}

// VerificationConfig — настройки подтверждения email.
type VerificationConfig struct {
	TokenTTL       time.Duration // срок жизни токена подтверждения
	ResendCooldown time.Duration // минимальный интервал между отправками писем
	MaxPerHour     int           // максимум писем на пользователя в час
}

// SMTPConfig — настройки почтового сервиса.
type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
}

// StorageConfig — настройки хранилища медиафайлов публикаций.
// Хранилище — S3-совместимый MinIO.
type StorageConfig struct {
	Driver        string // "minio"
	Endpoint      string // хост:порт S3-API, напр. "minio:9000"
	AccessKey     string // логин доступа
	SecretKey     string // секретный ключ
	Bucket        string // имя бакета
	Region        string // регион (по умолчанию us-east-1)
	UseSSL        bool   // использовать TLS
	PublicBaseURL string // публичный базовый URL для ссылок на медиа
}

// Load читает переменные окружения, применяет безопасные значения по умолчанию
// и валидирует результат. Возвращает ошибку, если конфигурация невалидна.
func Load() (*Config, error) {
	cfg := &Config{
		Env: Environment(strings.ToLower(getEnv("APP_ENV", string(EnvDevelopment)))),
		HTTP: HTTPConfig{
			Host:              getEnv("HTTP_HOST", "0.0.0.0"),
			Port:              getEnv("HTTP_PORT", "8080"),
			ReadHeaderTimeout: getEnvDuration("HTTP_READ_HEADER_TIMEOUT", 10*time.Second),
			IdleTimeout:       getEnvDuration("HTTP_IDLE_TIMEOUT", 120*time.Second),
			ShutdownTimeout:   getEnvDuration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
			RequestTimeout:    getEnvDuration("HTTP_REQUEST_TIMEOUT", 60*time.Second),
			AllowedOrigins:    getEnvSlice("HTTP_ALLOWED_ORIGINS", []string{"*"}),
		},
		DB: DBConfig{
			URL:             getEnv("DATABASE_URL", ""),
			MaxConns:        int32(getEnvInt("DB_MAX_CONNS", 25)),
			MinConns:        int32(getEnvInt("DB_MIN_CONNS", 5)),
			MaxConnIdleTime: getEnvDuration("DB_MAX_CONN_IDLE_TIME", 15*time.Minute),
			AutoMigrate:     getEnvBool("DB_AUTO_MIGRATE", true),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", ""),
			AccessTTL:  getEnvDuration("JWT_ACCESS_TTL", 60*time.Minute),
			RefreshTTL: getEnvDuration("JWT_REFRESH_TTL", 30*24*time.Hour),
		},
		Verification: VerificationConfig{
			TokenTTL:       getEnvDuration("VERIFY_TOKEN_TTL", 15*time.Minute),
			ResendCooldown: getEnvDuration("VERIFY_RESEND_COOLDOWN", 60*time.Second),
			MaxPerHour:     getEnvInt("VERIFY_MAX_PER_HOUR", 5),
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", ""),
			Port:     getEnvInt("SMTP_PORT", 465),
			User:     getEnv("SMTP_USER", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
		},
		Storage: StorageConfig{
			Driver:        strings.ToLower(getEnv("STORAGE_DRIVER", "minio")),
			Endpoint:      getEnv("MINIO_ENDPOINT", ""),
			AccessKey:     getEnv("MINIO_ACCESS_KEY", ""),
			SecretKey:     getEnv("MINIO_SECRET_KEY", ""),
			Bucket:        getEnv("MINIO_BUCKET", "cfu-board"),
			Region:        getEnv("MINIO_REGION", "us-east-1"),
			UseSSL:        getEnvBool("MINIO_USE_SSL", false),
			PublicBaseURL: strings.TrimRight(getEnv("MINIO_PUBLIC_URL", ""), "/"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Публичный базовый URL: если не задан явно — выводим из HTTP-адреса.
	cfg.PublicBaseURL = strings.TrimRight(
		getEnv("PUBLIC_BASE_URL", "http://"+cfg.HTTP.Host+":"+cfg.HTTP.Port),
		"/",
	)

	return cfg, nil
}

// Validate проверяет обязательные поля и здравые границы значений.
func (c *Config) Validate() error {
	if c.Env != EnvDevelopment && c.Env != EnvProduction {
		return fmt.Errorf("APP_ENV должен быть 'development' или 'production', получено: %q", c.Env)
	}
	if c.DB.URL == "" {
		return fmt.Errorf("DATABASE_URL обязателен")
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET обязателен")
	}
	// В продакшене короткий/предсказуемый секрет недопустим.
	if c.Env == EnvProduction && len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET слишком короткий для production (минимум 32 символа)")
	}
	if c.JWT.AccessTTL <= 0 {
		return fmt.Errorf("JWT_ACCESS_TTL должен быть положительным")
	}
	if c.JWT.RefreshTTL <= 0 {
		return fmt.Errorf("JWT_REFRESH_TTL должен быть положительным")
	}
	if c.JWT.RefreshTTL <= c.JWT.AccessTTL {
		return fmt.Errorf("JWT_REFRESH_TTL должен быть больше JWT_ACCESS_TTL")
	}
	if c.Verification.TokenTTL <= 0 {
		return fmt.Errorf("VERIFY_TOKEN_TTL должен быть положительным")
	}
	if c.Verification.ResendCooldown <= 0 {
		return fmt.Errorf("VERIFY_RESEND_COOLDOWN должен быть положительным")
	}
	if c.Verification.MaxPerHour <= 0 {
		return fmt.Errorf("VERIFY_MAX_PER_HOUR должен быть больше 0")
	}
	if c.DB.MaxConns <= 0 || c.DB.MinConns < 0 {
		return fmt.Errorf("DB_MAX_CONNS должен быть > 0, DB_MIN_CONNS >= 0")
	}
	if c.DB.MinConns > c.DB.MaxConns {
		return fmt.Errorf("DB_MIN_CONNS не может превышать DB_MAX_CONNS")
	}
	// Валидация хранилища: единственный поддерживаемый драйвер — minio.
	if c.Storage.Driver != "minio" {
		return fmt.Errorf("STORAGE_DRIVER должен быть 'minio', получено: %q", c.Storage.Driver)
	}
	if c.Storage.Endpoint == "" {
		return fmt.Errorf("MINIO_ENDPOINT обязателен для драйвера 'minio'")
	}
	if c.Storage.AccessKey == "" || c.Storage.SecretKey == "" {
		return fmt.Errorf("MINIO_ACCESS_KEY и MINIO_SECRET_KEY обязательны для драйвера 'minio'")
	}
	if c.Storage.Bucket == "" {
		return fmt.Errorf("MINIO_BUCKET обязателен для драйвера 'minio'")
	}
	return nil
}

// IsProduction — true, если приложение работает в боевом режиме.
func (c *Config) IsProduction() bool { return c.Env == EnvProduction }

// --- Вспомогательные функции чтения окружения ---

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvSlice(key string, fallback []string) []string {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
