# ============================================================
#  CFU-Board — корневой Makefile монорепо
#
#  Локальные Go-команды делегируются в backend/ (там go.mod).
#  Docker-команды работают с корневым docker-compose.yml.
# ============================================================

.PHONY: help build run test vet fmt tidy lint docker-up docker-down docker-logs docker-build clean web-install web-dev web-build web-typecheck

## help: показать список доступных команд
help:
@echo "Доступные команды:"
@echo "  build        собрать бэкенд в backend/bin"
@echo "  run          запустить бэкенд локально (нужен backend/.env и поднятая БД)"
@echo "  test         прогнать тесты бэкенда"
@echo "  vet          статический анализ бэкенда"
@echo "  fmt          применить gofmt к бэкенду"
@echo "  tidy         подтянуть/почистить зависимости бэкенда"
@echo "  lint         форматирование + vet + тесты бэкенда"
@echo "  docker-up    собрать образ и поднять прод-окружение"
@echo "  docker-down  остановить и удалить контейнеры"
@echo "  docker-logs  смотреть логи приложения"
@echo "  docker-build пересобрать образ приложения без кеша"
@echo "  clean        удалить артефакты сборки"
@echo "  web-install  установить зависимости фронтенда"
@echo "  web-dev      запустить фронтенд в dev-режиме"
@echo "  web-build    собрать фронтенд (Next.js)"
@echo "  web-typecheck проверить типы фронтенда (tsc)"

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

## fmt: применить gofmt
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
@rm -rf web/.next web/out
@echo "Готово."

## web-install: установить зависимости фронтенда
web-install:
@cd web && npm ci

## web-dev: запустить фронтенд в dev-режиме
web-dev:
@cd web && npm run dev

## web-build: собрать фронтенд (Next.js)
web-build:
@cd web && npm run build

## web-typecheck: проверить типы фронтенда (tsc)
web-typecheck:
@cd web && npm run typecheck
