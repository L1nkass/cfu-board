package api

import (
	"log/slog"
	"net/http"
	"os"
)

// openAPISpecPath — путь к файлу спецификации относительно рабочего каталога.
const openAPISpecPath = "docs/openapi.yaml"

// redocCSP — политика безопасности, необходимая для отображения Redoc,
// который подгружает бандл и шрифты со внешнего CDN. Переопределяет жёсткий
// глобальный CSP (SecurityHeaders) только для страницы /docs.
const redocCSP = "default-src 'self'; " +
	"script-src 'self' https://cdn.redoc.ly https://unpkg.com; " +
	"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
	"font-src 'self' https://fonts.gstatic.com; " +
	"img-src 'self' data: https:; " +
	"connect-src 'self'"

// docsPageHTML — страница с интерактивной документацией Redoc.
const docsPageHTML = `<!DOCTYPE html>
<html lang="ru">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>CFU-Board API — документация</title>
    <link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet" />
    <style>body{margin:0;padding:0;}</style>
  </head>
  <body>
    <redoc spec-url="/docs/openapi.yaml"></redoc>
    <script src="https://cdn.redoc.ly/redoc/latest/bundles/redoc.standalone.js"></script>
  </body>
</html>`

// DocsHandler отдаёт HTML-страницу с интерактивной документацией (Redoc).
// Требует интернета для загрузки бандла Redoc и шрифтов с CDN.
func DocsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", redocCSP)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(docsPageHTML))
}

// OpenAPIYAMLHandler отдаёт файл спецификации OpenAPI (openapi.yaml).
// Фронтендер может скачать его для генерации типизированного клиента.
func OpenAPIYAMLHandler(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(openAPISpecPath)
	if err != nil {
		slog.Error("не удалось прочитать openapi.yaml", "path", openAPISpecPath, "error", err)
		RespondWithError(w, http.StatusNotFound, "Спецификация API не найдена")
		return
	}

	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", "inline; filename=\"openapi.yaml\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
