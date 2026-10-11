# ============================================================
#  CFU-Board — Makefile для типовых операций
# ============================================================

BINARY_NAME := cfu-board
DOCKER_IMAGE := cfu-board:latest

.PHONY: help build run test vet fmt tidy docker-up docker-down clean lint

## help: показать список доступных команд
help:
	@echo "Доступные команды:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## build: собрать бинарник в ./bin
build:
	@echo "==> Сборка $(BINARY_NAME)..."
	@mkdir -p bin
	@CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/$(BINARY_NAME) ./cmd/app

## run: запустить локально (нужен .env и поднятая БД)
run:
	@go run ./cmd/app

## test: прогнать тесты
test:
	@go test ./... -race -cover

## vet: статический анализ
vet:
	@go vet ./...

## fmt: применить gofmt
fmt:
	@gofmt -w .

## tidy: подтянуть/почистить зависимости
tidy:
	@go mod tidy

## lint: форматирование + vet + тесты (быстрый контроль качества)
lint: fmt vet test

## docker-up: собрать образ и поднять прод-окружение
docker-up:
	@docker compose up --build -d
	@docker compose ps

## docker-down: остановить и удалить контейнеры
docker-down:
	@docker compose down

## docker-logs: смотреть логи приложения
docker-logs:
	@docker compose logs -f app

## clean: удалить артефакты сборки
clean:
	@rm -rf bin
	@echo "Готово."
