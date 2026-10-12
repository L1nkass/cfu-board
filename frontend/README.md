# frontend — веб-клиент CFU-Board («Микроволнивка»)

## Стек

- **Next.js 16** (App Router) + **React 19** + **TypeScript**
- **Tailwind CSS 4** (тема из исходного макета: тёмная клетка `#141722`,
  панели `#222633`, акцент `#ffe066`, переключатель луна/солнце)
- **TanStack Query** — кэш и мутации API
- **React Hook Form + Zod** — формы (вход, регистрация, пост, комментарии,
  причина отказа в модерации)
- **shadcn/ui** (Radix UI) — базовые компоненты (`src/components/ui`)

## Возможности

- Лента постов (сетка 3 колонки, бесконечная подгрузка, лайки, комментарии)
- Регистрация + подтверждение email (`/register`, `/verify`)
- Вход/выход, автопродление JWT-токенов (refresh-ротация)
- Создание поста с изображением/видео (отправка на модерацию)
- Профиль: свой (`/profile`) и чужой (`/user/{id}`) со статистикой
- Панель модерации для админов (`/admin`): очередь pending-постов,
  одобрение/отклонение с причиной

## API

Все запросы идут на `/api/*` того же origin — Next.js переписывает их
на бэкенд (rewrites в `next.config.ts`), CORS не нужен.

- Локально: `API_PROXY_TARGET` по умолчанию `http://localhost:8080`
- В Docker compose: сборочный аргумент `http://app:8080`

Спека бэкенда: `../backend/docs/openapi.yaml` (интерактивно — http://localhost:8080/docs).

## Запуск

```bash
# разработка (нужен запущенный бэкенд на :8080)
npm install
npm run dev        # http://localhost:3000

# прод-сборка
npm run build && npm start
```

## Docker

Сервис `frontend` описан в `../docker-compose.yml`:

```bash
# из корня репозитория
docker compose up --build frontend
# http://localhost:3000
```

