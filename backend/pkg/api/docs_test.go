package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestDocsHandler проверяет, что /docs отдаёт HTML с Redoc и CSP для CDN.
func TestDocsHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rec := httptest.NewRecorder()

	DocsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался код 200, получено %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("неверный Content-Type: %q", ct)
	}
	if csp := rec.Header().Get("Content-Security-Policy"); csp == "" {
		t.Fatal("CSP должен быть установлен для Redoc")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "redoc") || !strings.Contains(body, "/docs/openapi.yaml") {
		t.Fatal("HTML должен ссылаться на redoc и /docs/openapi.yaml")
	}
}

// TestOpenAPIYAMLHandler проверяет, что /docs/openapi.yaml отдаёт валидную спеку.
func TestOpenAPIYAMLHandler(t *testing.T) {
	// Хендлер читает файл относительно рабочего каталога; в тесте cwd = pkg/api,
	// поэтому временно переходим в корень проекта и возвращаемся обратно.
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	if err := os.Chdir("../.."); err != nil {
		t.Fatalf("не удалось перейти в корень проекта: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
	rec := httptest.NewRecorder()

	OpenAPIYAMLHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался код 200, получено %d, тело: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/yaml; charset=utf-8" {
		t.Fatalf("неверный Content-Type: %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "openapi: 3.0.3") {
		t.Fatal("ответ должен содержать спецификацию OpenAPI")
	}
}
