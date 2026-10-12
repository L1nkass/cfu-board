package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	yaml "go.yaml.in/yaml/v3"
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

// TestOpenAPISpecParses строго парсит docs/openapi.yaml и проверяет,
// что админ-эндпоинты модерации присутствуют, а все $ref на components/responses
// разрешаются (нет битых ссылок).
func TestOpenAPISpecParses(t *testing.T) {
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	if err := os.Chdir("../.."); err != nil {
		t.Fatalf("не удалось перейти в корень проекта: %v", err)
	}

	data, err := os.ReadFile("docs/openapi.yaml")
	if err != nil {
		t.Fatalf("не удалось прочитать спеку: %v", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("openapi.yaml не парсится как YAML: %v", err)
	}

	paths, _ := doc["paths"].(map[string]any)
	for _, want := range []string{
		"/admin/posts/pending",
		"/admin/posts/{id}/approve",
		"/admin/posts/{id}/reject",
	} {
		if _, ok := paths[want]; !ok {
			t.Errorf("в спеке нет пути %s", want)
		}
	}

	components, _ := doc["components"].(map[string]any)
	responses, _ := components["responses"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)

	// Собираем все $ref и проверяем, что каждый разрешается.
	var refs []string
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for k, val := range t {
				if k == "$ref" {
					if s, ok := val.(string); ok {
						refs = append(refs, s)
					}
				}
				walk(val)
			}
		case []any:
			for _, val := range t {
				walk(val)
			}
		}
	}
	walk(doc)

	if len(refs) == 0 {
		t.Fatal("в спеке нет ни одного $ref — что-то пошло не так")
	}
	for _, ref := range refs {
		const prefix = "#/components/"
		if !strings.HasPrefix(ref, prefix) {
			t.Errorf("неожиданный формат $ref: %q", ref)
			continue
		}
		rest := strings.TrimPrefix(ref, prefix)
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 {
			t.Errorf("некорректный $ref: %q", ref)
			continue
		}
		group, name := parts[0], parts[1]
		switch group {
		case "responses":
			if _, ok := responses[name]; !ok {
				t.Errorf("битая ссылка %q: components.responses.%s не существует", ref, name)
			}
		case "schemas":
			if _, ok := schemas[name]; !ok {
				t.Errorf("битая ссылка %q: components.schemas.%s не существует", ref, name)
			}
		default:
			t.Errorf("неожиданная группа компонентов в $ref: %q", ref)
		}
	}
}
