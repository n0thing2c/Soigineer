import { authApi, configureAuthSession } from "@/api/client";
import type { LoginResult, User } from "@/api/types";
import {
  createContext,
  ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

const STORAGE_KEY = "soigineer.auth";

interface AuthState {
  token: string;
  refreshToken: string;
  user: User;
}

interface AuthContextValue {
  token: string | null;
  refreshToken: string | null;
  user: User | null;
  isAuthenticated: boolean;
  isAdmin: boolean;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refreshSession: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

function readStoredAuth(): AuthState | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? (JSON.parse(raw) as AuthState) : null;
  } catch {
    return null;
  }
}

function persist(result: LoginResult) {
  const state: AuthState = {
    token: result.token,
    refreshToken: result.refreshToken,
    user: result.user,
  };
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  return state;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState | null>(() => readStoredAuth());
  const [loading, setLoading] = useState(Boolean(state?.token));
  const stateRef = useRef<AuthState | null>(state);

  const clearLocalSession = useCallback(() => {
    localStorage.removeItem(STORAGE_KEY);
    setState(null);
    stateRef.current = null;
  }, []);

  const storeResult = useCallback((result: LoginResult) => {
    const next = persist(result);
    stateRef.current = next;
    setState(next);
  }, []);

  useEffect(() => {
    stateRef.current = state;
  }, [state]);

  useEffect(() => {
    configureAuthSession({
      getRefreshToken: () => stateRef.current?.refreshToken ?? null,
      onRefresh: storeResult,
      onLogout: clearLocalSession,
    });

    return () => configureAuthSession(null);
  }, [clearLocalSession, storeResult]);

  useEffect(() => {
    let cancelled = false;

    async function loadUser() {
      if (!state?.token) {
        setLoading(false);
        return;
      }

      try {
        const user = await authApi.me(state.token);
        if (!cancelled) {
          const next = { ...state, user };
          localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
          setState(next);
        }
      } catch {
        if (!cancelled) {
          clearLocalSession();
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    loadUser();
    return () => {
      cancelled = true;
    };
  }, [clearLocalSession, state?.token]);

  const login = useCallback(async (username: string, password: string) => {
    const result = await authApi.login(username, password);
    storeResult(result);
  }, [storeResult]);

  const refreshSession = useCallback(async () => {
    const refreshToken = stateRef.current?.refreshToken;
    if (!refreshToken) {
      clearLocalSession();
      return;
    }
    const result = await authApi.refresh(refreshToken);
    storeResult(result);
  }, [clearLocalSession, storeResult]);

  const logout = useCallback(async () => {
    const refreshToken = stateRef.current?.refreshToken;
    const token = stateRef.current?.token;
    clearLocalSession();

    if (refreshToken) {
      try {
        await authApi.logout(refreshToken, token);
      } catch {
        // Local logout should still succeed if the auth service is offline.
      }
    }
  }, [clearLocalSession]);

  const value = useMemo<AuthContextValue>(
    () => ({
      token: state?.token ?? null,
      refreshToken: state?.refreshToken ?? null,
      user: state?.user ?? null,
      isAuthenticated: Boolean(state?.token),
      isAdmin: state?.user?.role === "admin",
      loading,
      login,
      logout,
      refreshSession,
    }),
    [loading, login, logout, refreshSession, state],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return value;
}
