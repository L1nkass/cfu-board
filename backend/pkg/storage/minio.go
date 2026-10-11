package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"cfu-board/pkg/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// publicReadPolicy — политика публичного чтения бакета. Позволяет любому
// анонимному пользователю скачивать объекты (s3:GetObject), чтобы медиа
// корректно отображалось в <img>/<video> на клиенте без подписи URL.
// Запись (PutObject) при этом по-прежнему требует авторизации по ключам.
const publicReadPolicy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": ["*"]},
      "Action": ["s3:GetObject"],
      "Resource": ["arn:aws:s3:::%s/*"]
    }
  ]
}`

// MinioDriver хранит файлы в S3-совместимом объектном хранилище MinIO.
type MinioDriver struct {
	client     *minio.Client
	bucket     string
	publicBase string // базовый публичный адрес, напр. "http://localhost:9000"
}

// NewMinio создаёт клиент MinIO по параметрам конфигурации.
func NewMinio(cfg config.StorageConfig) (*MinioDriver, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("MINIO_ENDPOINT обязателен для драйвера minio")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("не удалось создать клиент MinIO: %w", err)
	}
	// Публичный базовый URL: если не задан — собираем из endpoint и схемы.
	publicBase := strings.TrimRight(cfg.PublicBaseURL, "/")
	if publicBase == "" {
		scheme := "http"
		if cfg.UseSSL {
			scheme = "https"
		}
		publicBase = scheme + "://" + cfg.Endpoint
	}
	return &MinioDriver{
		client:     client,
		bucket:     cfg.Bucket,
		publicBase: publicBase,
	}, nil
}

func (d *MinioDriver) Driver() string { return "minio" }

// Ensure создаёт бакет при отсутствии и делает его публично читаемым,
// чтобы медиа было доступно по прямой ссылке. Идемпотентна.
func (d *MinioDriver) Ensure(ctx context.Context) error {
	exists, err := d.client.BucketExists(ctx, d.bucket)
	if err != nil {
		return fmt.Errorf("не удалось проверить существование бакета %q: %w", d.bucket, err)
	}
	if !exists {
		if err := d.client.MakeBucket(ctx, d.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("не удалось создать бакет %q: %w", d.bucket, err)
		}
	}
	policy := fmt.Sprintf(publicReadPolicy, d.bucket)
	if err := d.client.SetBucketPolicy(ctx, d.bucket, policy); err != nil {
		return fmt.Errorf("не удалось выставить политику публичного чтения бакета %q: %w", d.bucket, err)
	}
	return nil
}

// Save загружает объект в бакет под ключом key и возвращает публичный URL.
func (d *MinioDriver) Save(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}
	// size == -1 означает неизвестную длину — minio-go использует multipart.
	if _, err := d.client.PutObject(ctx, d.bucket, key, r, size, opts); err != nil {
		return "", fmt.Errorf("не удалось загрузить объект %q в MinIO: %w", key, err)
	}
	return d.publicBase + "/" + d.bucket + "/" + key, nil
}

// Delete удаляет объект по публичному URL, извлекая ключ из пути бакета.
func (d *MinioDriver) Delete(ctx context.Context, url string) error {
	key, ok := keyFromMinioURL(url, d.bucket)
	if !ok {
		return nil // URL не принадлежит нашему бакету — нечего удалять.
	}
	if err := d.client.RemoveObject(ctx, d.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("не удалось удалить объект %q из MinIO: %w", key, err)
	}
	return nil
}

// keyFromMinioURL извлекает ключ объекта из URL, находя маркер "/<bucket>/".
// Устойчиво к смене публичного базового адреса (CDN/прокси).
func keyFromMinioURL(url, bucket string) (string, bool) {
	marker := "/" + bucket + "/"
	_, key, found := strings.Cut(url, marker)
	if !found || key == "" {
		return "", false
	}
	return key, true
}
