# ============================================================
#  CFU-Board — корневой Makefile монорепо
#
#  Локальные Go-команды делегируются в backend/ (там go.mod).
#  Docker-команды работают с корневым docker-compose.yml.
# ============================================================

.PHONY: help build run test vet fmt tidy lint docker-up docker-down docker-logs docker-build clean

## help: показать список доступных команд
help:
	@echo "Доступные команды:"
	@echo "  build        собрать бэкенд в backend/bin"
	@echo "  run          запустить бэкенд локально (нужен backend/.env и поднятая БД)"
	@echo "  test         прогнать тесты бэкенда"
	@echo "  vet          статический анализ бэкенда"
	@echo "  fmt          применить gofmt к бэкенду"
	@echo "  tidy         подтянуть/почистить зависимости бэкенда"
	@echo "  lint         форматирование + vet + тесты"
	@echo "  docker-up    собрать образ и поднять прод-окружение"
	@echo "  docker-down  остановить и удалить контейнеры"
	@echo "  docker-logs  смотреть логи приложения"
	@echo "  docker-build пересобрать образ приложения без кеша"
	@echo "  clean        удалить артефакты сборки"

## build: собрать бэкенд в backend/bin
build:
	@cd backend && $(MAKE) build

## run: запустить бэкенд локально (нужен backend/.env и поднятая БД)
run:
	@cd backend && $(MAKE) run

## test: прогнать тесты бэкенда
test:
	@cd backend && $(MAKE) test

## vet: статический анализ бэкенда
vet:
	@cd backend && $(MAKE) vet

## fmt: применить gofmt к бэкенду
fmt:
	@cd backend && $(MAKE) fmt

## tidy: подтянуть/почистить зависимости бэкенда
tidy:
	@cd backend && $(MAKE) tidy

## lint: форматирование + vet + тесты (быстрый контроль качества)
lint:
	@cd backend && $(MAKE) lint

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

## docker-build: пересобрать образ приложения без кеша
docker-build:
	@docker compose build --no-cache app

## clean: удалить артефакты сборки
clean:
	@cd backend && $(MAKE) clean
	@echo "Готово."
