# ============================================================
#  CFU-Board frontend — Next.js multi-stage сборка (standalone)
# ============================================================

# ---------- Этап 1: зависимости ----------
FROM node:24-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

# ---------- Этап 2: сборка ----------
FROM node:24-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .

# Адрес бэкенда запекается в rewrites next.config.ts при сборке.
ARG API_PROXY_TARGET=http://app:8080
ENV API_PROXY_TARGET=${API_PROXY_TARGET}
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

# ---------- Этап 3: раннер (standalone) ----------
FROM node:24-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0

RUN addgroup -S app && adduser -S app -G app

# standalone-соборка уже содержит сервер и node_modules приложения.
COPY --from=builder --chown=app:app /app/.next/standalone ./
COPY --from=builder --chown=app:app /app/.next/static ./.next/static
COPY --from=builder --chown=app:app /app/public ./public

USER app
EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://127.0.0.1:3000/ || exit 1

CMD ["node", "server.js"]
