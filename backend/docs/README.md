# CFU-Board API — документация для фронтенда

## 🚀 Быстрый старт

Интерактивная документация (Redoc) поднимается вместе с бэкендом:

- **Локально:** откройте http://localhost:8080/docs в браузере
- **Сырой файл спеки:** http://localhost:8080/docs/openapi.yaml

> Redoc подгружает бандл и шрифты с CDN (`cdn.redoc.ly`, `fonts.googleapis.com`),
> поэтому для отображения страницы нужен доступ в интернет.

## 🔐 Аутентификация (кратко)

1. `POST /login` → ответ `{ access_token, refresh_token, user }`.
2. `access_token` (JWT, 60 мин) — в заголовке защищённых запросов:
   `Authorization: Bearer <access_token>`.
3. Когда `access_token` истёк (401) — вызовите `POST /refresh`
   с `{ "refresh_token": "..." }`. Получите **новую пару** токенов.
   ⚠️ `refresh_token` одноразовый — всегда сохраняйте новый из ответа `/refresh`.
4. `POST /logout` с `{ "refresh_token": "..." }` — выход (отзыв токена).

## 🧰 Генерация типизированного клиента (TypeScript)

Спека — стандартный OpenAPI 3.0. Рекомендуемые инструменты:

**openapi-typescript** (только типы):
```bash
npx openapi-typescript http://localhost:8080/docs/openapi.yaml -o ./src/api/types.ts
```

**orval** (типы + готовые функции/хуки):
```bash
# orval.config.ts
export default {
  cfu: {
    input: 'http://localhost:8080/docs/openapi.yaml',
    output: { target: './src/api/client.ts', client: 'react-query' },
  },
}
# npx orval
```

**openapi-generator** (генерация SDK):
```bash
npx @openapitools/openapi-generator-cli generate \
  -i http://localhost:8080/docs/openapi.yaml \
  -g typescript-fetch -o ./src/api
```

## 📤 Загрузка постов (`POST /posts/create`)

Формат `multipart/form-data`:
- `title` (string, обязательно, ≤255)
- `content` (string, обязательно)
- `image` (файл, необязательно) — JPG/PNG/WEBP
- `video` (файл, необязательно) — MP4/MOV/AVI

Общий лимит запроса — 500 МБ. Нужен заголовок `Authorization: Bearer <access_token>`.

Пример (fetch):
```ts
const fd = new FormData();
fd.append('title', 'Продам велосипед');
fd.append('content', 'Отличное состояние');
fd.append('image', imageFile);

await fetch('/posts/create', {
  method: 'POST',
  headers: { Authorization: `Bearer ${accessToken}` },
  body: fd,
});
```

## 🩺 Health-checks

- `GET /healthz` — liveness (процесс жив).
- `GET /readyz` — readiness (проверяет доступность БД; 503, если БД недоступна).

## ❗ Формат ошибок

Все ошибки: `{ "error": "описание" }`.
Коды: 400 · 401 · 404 · 409 · 413 · 429 · 500.
