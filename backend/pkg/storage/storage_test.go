package storage

import (
	"testing"

	"cfu-board/pkg/config"
)

// TestNew_DriverSelection проверяет, что фабрика создаёт minio-драйвер по
// корректному конфигу и отклоняет неизвестный драйвер.
func TestNew_DriverSelection(t *testing.T) {
	m, err := New(config.StorageConfig{
		Driver:    "minio",
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "cfu-board",
	})
	if err != nil {
		t.Fatalf("New(minio) не должен падать: %v", err)
	}
	if m.Driver() != "minio" {
		t.Fatalf("ожидался драйвер 'minio', получено %q", m.Driver())
	}

	// Minio без endpoint — ошибка.
	if _, err := New(config.StorageConfig{Driver: "minio"}); err == nil {
		t.Fatal("ожидалась ошибка для minio без endpoint")
	}

	// Неизвестный/устаревший драйвер (в т.ч. прежний "local") — ошибка.
	if _, err := New(config.StorageConfig{Driver: "local"}); err == nil {
		t.Fatal("ожидалась ошибка для неизвестного драйвера хранилища")
	}
}

// TestMinioDriver_KeyFromURL проверяет извлечение ключа объекта из публичного
// URL бакета (в т.ч. при нестандартном базовом адресе/CDN).
func TestMinioDriver_KeyFromURL(t *testing.T) {
	key, ok := keyFromMinioURL("http://localhost:9000/cfu-board/posts/1_42.png", "cfu-board")
	if !ok || key != "posts/1_42.png" {
		t.Fatalf("ожидался ключ 'posts/1_42.png', получено %q (ok=%v)", key, ok)
	}
	// URL чужого бакета не должен распознаваться.
	if _, ok := keyFromMinioURL("http://localhost:9000/other-bucket/x.png", "cfu-board"); ok {
		t.Fatal("URL чужого бакета не должен распознаваться как свой")
	}
}

// TestMinioDriver_SaveURL проверяет формирование публичного URL при сохранении
// без обращения к сети: используем заглушку клиента через публичный базовый URL.
func TestMinioDriver_PublicBase(t *testing.T) {
	d, err := NewMinio(config.StorageConfig{
		Driver:        "minio",
		Endpoint:      "minio:9000",
		AccessKey:     "k",
		SecretKey:     "s",
		Bucket:        "cfu-board",
		PublicBaseURL: "http://localhost:9000",
	})
	if err != nil {
		t.Fatalf("NewMinio не должен падать: %v", err)
	}
	if d.publicBase != "http://localhost:9000" {
		t.Fatalf("publicBase должен обрезать завершающий слэш, получено %q", d.publicBase)
	}
	// Если PublicBaseURL не задан — собираем из endpoint и схемы.
	d2, err := NewMinio(config.StorageConfig{
		Driver:    "minio",
		Endpoint:  "minio:9000",
		AccessKey: "k",
		SecretKey: "s",
		Bucket:    "cfu-board",
	})
	if err != nil {
		t.Fatalf("NewMinio не должен падать: %v", err)
	}
	if d2.publicBase != "http://minio:9000" {
		t.Fatalf("publicBase по умолчанию должен быть http://<endpoint>, получено %q", d2.publicBase)
	}
}
