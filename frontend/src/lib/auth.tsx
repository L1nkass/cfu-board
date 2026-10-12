"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import {
  api,
  clearTokens,
  getRefreshToken,
  setTokens,
} from "./api";
import type { LoginResponse, OwnProfile, SuccessResponse } from "./types";

interface AuthContextValue {
  user: OwnProfile | null;
  /** Пока true — токены не проверены, UI не должен решать по user. */
  loading: boolean;
  login: (login: string, password: string) => Promise<void>;
  register: (
    username: string,
    email: string,
    password: string,
  ) => Promise<SuccessResponse>;
  logout: () => Promise<void>;
  /** Перечитать /users/me (например, после верификации email). */
  reloadUser: () => Promise<OwnProfile | null>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<OwnProfile | null>(null);
  const [loading, setLoading] = useState(true);

  const reloadUser = useCallback(async (): Promise<OwnProfile | null> => {
    try {
      const me = await api<OwnProfile>("/users/me");
      setUser(me);
      return me;
    } catch {
      clearTokens();
      setUser(null);
      return null;
    }
  }, []);

  // Восстановление сессии из localStorage.
  useEffect(() => {
    let alive = true;
    (async () => {
      if (getRefreshToken() || localStorage.getItem("cfu_access")) {
        const me = await reloadUser();
        if (!alive) return;
        void me;
      }
      if (alive) setLoading(false);
    })();
    return () => {
      alive = false;
    };
  }, [reloadUser]);

  const login = useCallback(
    async (loginValue: string, password: string) => {
      const res = await api<LoginResponse>("/login", {
        method: "POST",
        json: { login: loginValue, password },
        noRefresh: true,
      });
      setTokens(res.access_token, res.refresh_token);
      await reloadUser();
    },
    [reloadUser],
  );

  const register = useCallback(
    async (username: string, email: string, password: string) =>
      api<SuccessResponse>("/register", {
        method: "POST",
        json: { username, email, password },
        noRefresh: true,
      }),
    [],
  );

  const logout = useCallback(async () => {
    const refresh_token = getRefreshToken();
    try {
      if (refresh_token) {
        await api<SuccessResponse>("/logout", {
          method: "POST",
          json: { refresh_token },
          noRefresh: true,
        });
      }
    } catch {
      /* локальный выход всё равно выполняем */
    }
    clearTokens();
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider
      value={{ user, loading, login, register, logout, reloadUser }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth должен использоваться внутри AuthProvider");
  return ctx;
}
