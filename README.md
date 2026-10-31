# CFU-Board — соцсеть объявлений

Монорепо:

| Каталог | Что внутри |
|---|---|
| `backend/` | Go API (net/http, pgx, MinIO) — см. `backend/docs/README.md` |
| `web/` | Веб-клиент (Next.js 16, shadcn/ui, React Query) |
| `docs/` | Архитектура и проектная документация |
| `docker-compose.yml` | Прод-окружение: app + PostgreSQL + MinIO |

## Быстрый старт (Docker)

```bash
cp .env.example .env        # заполнить JWT_SECRET и пароли
docker compose up --build   # из корня репозитория
```

- API: http://localhost:8080
- Документация (Redoc): http://localhost:8080/docs
- Сырая OpenAPI-спека: http://localhost:8080/docs/openapi.yaml
- MinIO-консоль: http://localhost:9001

## Локальная разработка бэкенда

```bash
cd backend
cp .env.example .env        # DATABASE_URL там указывает на localhost
make docker-up              # или поднимите postgres+minio любым способом
make run                    # запуск API
make lint                   # gofmt + vet + тесты
```

Корневой `Makefile` дублирует команды (`make test`, `make docker-up` …) с делегированием в `backend/`,
а также содержит команды для фронтенда (`make web-install`, `make web-dev`, `make web-build`, `make web-typecheck`).

## Переменные окружения

- Корень `.env` — для `docker compose` (образец: `.env.example`).
- `backend/.env` — для локального `go run` без Docker (образец: `backend/.env.example`).
- Оба файла в `.gitignore`, в репозиторий не попадают.

## Архитектура

См. `docs/architecture.md`. Бэкенд организован по слоям (transport/service/repository/domain),
внутренние пакеты находятся в `backend/internal/`, миграции — в `backend/migrations/`.
