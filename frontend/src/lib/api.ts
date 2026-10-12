import type { RefreshResponse } from "./types";

// HTTP-клиент API. Все запросы идут на /api/* того же origin —
// Next.js переписывает их на бэкенд (см. next.config.ts), поэтому
// CORS-ограничения не действуют.

const ACCESS_KEY = "cfu_access";
const REFRESH_KEY = "cfu_refresh";

export class ApiError extends Error {
  readonly status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

export function getAccessToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(ACCESS_KEY);
}

export function getRefreshToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(REFRESH_KEY);
}

export function setTokens(access: string, refresh: string): void {
  localStorage.setItem(ACCESS_KEY, access);
  localStorage.setItem(REFRESH_KEY, refresh);
}

export function clearTokens(): void {
  localStorage.removeItem(ACCESS_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

// Одноразовый refresh на все параллельные 401: ждущие запросы
// переиспользуют один и тот же промис.
let refreshPromise: Promise<boolean> | null = null;

async function refreshOnce(): Promise<boolean> {
  refreshPromise ??= (async () => {
    const refresh_token = getRefreshToken();
    if (!refresh_token) return false;
    try {
      const res = await fetch("/api/refresh", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token }),
      });
      if (!res.ok) return false;
      const data = (await res.json()) as RefreshResponse;
      setTokens(data.access_token, data.refresh_token);
      return true;
    } catch {
      return false;
    } finally {
      // Сбрасываем мьютекс сразу: новый refresh нужен только на новый 401.
      setTimeout(() => {
        refreshPromise = null;
      }, 0);
    }
  })();
  return refreshPromise;
}

export interface ApiOptions extends Omit<RequestInit, "body"> {
  /** JSON-тело (не использовать вместе с body/formData). */
  json?: unknown;
  /** Тело запроса как есть (FormData и т.п.). */
  body?: BodyInit | null;
  /** Таймаут по умолчанию — 30 с (для аплоадов передают больше). */
  timeoutMs?: number;
  /** Не продлевать токены при 401 (для /login, /refresh). */
  noRefresh?: boolean;
}

async function parseError(res: Response): Promise<ApiError> {
  let message = `Ошибка запроса (HTTP ${res.status})`;
  try {
    const data = (await res.json()) as { error?: string };
    if (data?.error) message = data.error;
  } catch {
    /* тело не JSON — оставляем общее сообщение */
  }
  return new ApiError(res.status, message);
}

/** Выполняет запрос к API и возвращает разобранный JSON. */
export async function api<T>(path: string, opts: ApiOptions = {}): Promise<T> {
  const { json, noRefresh, timeoutMs = 30_000, headers, ...rest } = opts;

  const doFetch = (): Promise<Response> => {
    const h = new Headers(headers);
    if (json !== undefined) h.set("Content-Type", "application/json");
    const access = getAccessToken();
    if (access) h.set("Authorization", `Bearer ${access}`);
    return fetch(`/api${path}`, {
      ...rest,
      headers: h,
      body: json !== undefined ? JSON.stringify(json) : rest.body,
      signal: AbortSignal.timeout(timeoutMs),
    });
  };

  let res = await doFetch();

  // Продлеваем токены один раз и повторяем запрос.
  if (res.status === 401 && !noRefresh && getRefreshToken()) {
    const refreshed = await refreshOnce();
    if (refreshed) res = await doFetch();
  }

  if (!res.ok) throw await parseError(res);
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

/** Multipart-загрузка (создание поста с медиафайлами). */
export async function apiUpload<T>(
  path: string,
  formData: FormData,
  timeoutMs = 15 * 60_000,
): Promise<T> {
  const doFetch = (): Promise<Response> => {
    const h = new Headers();
    const access = getAccessToken();
    if (access) h.set("Authorization", `Bearer ${access}`);
    return fetch(`/api${path}`, {
      method: "POST",
      headers: h,
      body: formData,
      signal: AbortSignal.timeout(timeoutMs),
    });
  };

  let res = await doFetch();
  if (res.status === 401 && getRefreshToken()) {
    const refreshed = await refreshOnce();
    if (refreshed) res = await doFetch();
  }

  if (!res.ok) throw await parseError(res);
  return (await res.json()) as T;
}
