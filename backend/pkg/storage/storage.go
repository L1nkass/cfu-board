// Package storage предоставляет абстракцию объектного хранилища для медиафайлов
// публикаций. Реализация — S3-совместимый MinIO; HTTP-слой работает через
// единый интерфейс Storage и не зависит от деталей хранилища.
package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"cfu-board/pkg/config"
)

// Storage описывает контракт хранилища медиафайлов.
type Storage interface {
	// Driver возвращает имя активного драйвера ("minio") — для логов.
	Driver() string
	// Ensure подготавливает хранилище к работе: создаёт бакет и политику
	// публичного чтения (minio). Вызывается один раз на старте.
	Ensure(ctx context.Context) error
	// Save загружает содержимое reader под ключом key и возвращает публичный URL.
	// size — полный размер объекта, contentType — MIME-тип для корректной отдачи.
	Save(ctx context.Context, key string, r io.Reader, size int64, contentType string) (url string, err error)
	// Delete удаляет объект по ранее выданному Save публичному URL. Повторный
	// вызов для отсутствующего объекта не является ошибкой.
	Delete(ctx context.Context, url string) error
}

// New создаёт реализацию Storage на основе конфигурации.
func New(cfg config.StorageConfig) (Storage, error) {
	if strings.ToLower(cfg.Driver) != "minio" {
		return nil, fmt.Errorf("неизвестный драйвер хранилища: %q (ожидается 'minio')", cfg.Driver)
	}
	return NewMinio(cfg)
}
