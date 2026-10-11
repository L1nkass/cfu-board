# ============================================================
#  CFU-Board — multi-stage сборка (малый финальный образ)
# ============================================================

# ---------- Этап 1: сборка ----------
FROM golang:1.27-alpine AS builder

# git и ca-certificates нужны для загрузки зависимостей goose/pgx
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Кэшируем зависимости: сначала копируем манифесты, качаем модули.
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники и собираем статический бинарник.
COPY . .

# CGO отключён → чистый статический бинарник, совместимый с alpine/distroless.
# -trimpath убирает абсолютные пути, -ldflags урезает размер и версионирует сборку.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=$(date +%Y%m%d%H%M%S)" \
    -o /out/cfu-board ./cmd/app

# ---------- Этап 2: раннер ----------
FROM alpine:3.21 AS runner

# ca-certificates — для TLS (SMTP), tzdata — для корректного времени.
RUN apk add --no-cache ca-certificates tzdata

# Непривилегированный пользователь (безопасность: не root).
RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

# Забираем бинарник из builder.
COPY --from=builder /out/cfu-board /app/cfu-board

# Миграции и спецификация API для интерактивной документации на /docs.
COPY --from=builder /app/pkg/migrations /app/pkg/migrations
COPY --from=builder /app/docs /app/docs
RUN chown -R app:app /app

USER app

EXPOSE 8080

# Healthcheck через встроенный эндпоинт /healthz.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/cfu-board"]
