import type {
  AlertRuleCreate,
  Application,
  AlertRule,
  HealthRow,
  Incident,
  IncidentStatus,
  LoginResult,
  ProcessedLogEvent,
  User,
} from "./types";

const AUTH_BASE_URL =
  import.meta.env.VITE_AUTH_API_URL ?? "http://localhost:8070/v1";
const MONITORING_BASE_URL =
  import.meta.env.VITE_MONITORING_API_URL ?? "http://localhost:8090/v1";

interface AuthSessionHandlers {
  getRefreshToken: () => string | null;
  onRefresh: (result: LoginResult) => void;
  onLogout: () => void;
}

let authSessionHandlers: AuthSessionHandlers | null = null;
let refreshPromise: Promise<LoginResult> | null = null;

type QueryValue = string | number | boolean | string[] | undefined | null;

export function configureAuthSession(handlers: AuthSessionHandlers | null) {
  authSessionHandlers = handlers;
}

export function getApiConfig() {
  return {
    authBaseUrl: AUTH_BASE_URL,
    monitoringBaseUrl: MONITORING_BASE_URL,
  };
}

function queryString(params: Record<string, QueryValue> = {}) {
  const search = new URLSearchParams();

  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === null || value === "") {
      return;
    }

    if (Array.isArray(value)) {
      if (value.length > 0) {
        search.set(key, value.join(","));
      }
      return;
    }

    search.set(key, String(value));
  });

  const value = search.toString();
  return value ? `?${value}` : "";
}

async function request<T>(
  baseUrl: string,
  path: string,
  options: RequestInit = {},
  token?: string,
  retryOnUnauthorized = true,
): Promise<T> {
  const headers = new Headers(options.headers);

  if (!(options.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(`${baseUrl}${path}`, {
    ...options,
    headers,
  });

  if (response.status === 401 && token && retryOnUnauthorized && authSessionHandlers) {
    const refreshToken = authSessionHandlers.getRefreshToken();
    if (refreshToken) {
      try {
        if (!refreshPromise) {
          refreshPromise = request<LoginResult>(
            AUTH_BASE_URL,
            "/auth/refresh",
            {
              method: "POST",
              body: JSON.stringify({ refreshToken }),
            },
            undefined,
            false,
          ).finally(() => {
            refreshPromise = null;
          });
        }
        const refreshed = await refreshPromise;
        authSessionHandlers.onRefresh(refreshed);
        return request<T>(baseUrl, path, options, refreshed.token, false);
      } catch {
        authSessionHandlers.onLogout();
      }
    } else {
      authSessionHandlers.onLogout();
    }
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  const payload = text ? JSON.parse(text) : undefined;

  if (!response.ok) {
    throw new Error(payload?.error ?? `Request failed with ${response.status}`);
  }

  return payload as T;
}

function items<T>(payload: { items: T[] }) {
  return payload.items ?? [];
}

export const authApi = {
  login(username: string, password: string) {
    return request<LoginResult>(AUTH_BASE_URL, "/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    });
  },

  refresh(refreshToken: string) {
    return request<LoginResult>(AUTH_BASE_URL, "/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refreshToken }),
    });
  },

  logout(refreshToken: string, token?: string) {
    return request<void>(
      AUTH_BASE_URL,
      "/auth/logout",
      {
        method: "POST",
        body: JSON.stringify({ refreshToken }),
      },
      token,
    );
  },

  me(token: string) {
    return request<User>(AUTH_BASE_URL, "/auth/me", {}, token);
  },

  async users(token: string) {
    return items(await request<{ items: User[] }>(AUTH_BASE_URL, "/admin/users", {}, token));
  },

  createUser(
    token: string,
    payload: { username: string; password: string; role: string; applications: string[] },
  ) {
    return request<User>(
      AUTH_BASE_URL,
      "/admin/users",
      {
        method: "POST",
        body: JSON.stringify(payload),
      },
      token,
    );
  },

  replaceApplications(token: string, userId: string, applications: string[]) {
    return request<User>(
      AUTH_BASE_URL,
      `/admin/users/${userId}/applications`,
      {
        method: "PUT",
        body: JSON.stringify({ applications }),
      },
      token,
    );
  },

  async adminApplications(token: string) {
    return items(await request<{ items: string[] }>(AUTH_BASE_URL, "/admin/applications", {}, token));
  },

  createApplication(token: string, payload: { name: string; displayName?: string }) {
    return request<Application>(
      AUTH_BASE_URL,
      "/admin/applications",
      {
        method: "POST",
        body: JSON.stringify(payload),
      },
      token,
    );
  },
};

export const monitoringApi = {
  me(token: string) {
    return request<User>(MONITORING_BASE_URL, "/me", {}, token);
  },

  async applications(token: string) {
    return items(await request<{ items: string[] }>(MONITORING_BASE_URL, "/applications", {}, token));
  },

  async logs(
    token: string,
    filters: { app?: string[]; level?: string[]; from?: string; to?: string; limit?: number },
  ) {
    return items(
      await request<{ items: ProcessedLogEvent[] }>(
        MONITORING_BASE_URL,
        `/logs${queryString(filters)}`,
        {},
        token,
      ),
    );
  },

  async incidents(
    token: string,
    filters: { app?: string[]; level?: string[]; status?: string; limit?: number },
  ) {
    return items(
      await request<{ items: Incident[] }>(
        MONITORING_BASE_URL,
        `/incidents${queryString(filters)}`,
        {},
        token,
      ),
    );
  },

  updateIncidentStatus(token: string, id: string, status: IncidentStatus) {
    return request<void>(
      MONITORING_BASE_URL,
      `/incidents/${id}/status`,
      {
        method: "PATCH",
        body: JSON.stringify({ status }),
      },
      token,
    );
  },

  async health(
    token: string,
    filters: { app?: string[]; level?: string[]; from?: string; to?: string; limit?: number },
  ) {
    return items(
      await request<{ items: HealthRow[] }>(
        MONITORING_BASE_URL,
        `/analytics/health${queryString(filters)}`,
        {},
        token,
      ),
    );
  },

  async alertRules(token: string) {
    return items(
      await request<{ items: AlertRule[] }>(
        MONITORING_BASE_URL,
        "/admin/alert-rules",
        {},
        token,
      ),
    );
  },

  updateAlertRule(
    token: string,
    id: string,
    payload: { enabled: boolean; dedupWindowSeconds: number; telegramEnabled: boolean },
  ) {
    return request<void>(
      MONITORING_BASE_URL,
      `/admin/alert-rules/${id}`,
      {
        method: "PUT",
        body: JSON.stringify(payload),
      },
      token,
    );
  },

  createAlertRule(token: string, payload: AlertRuleCreate) {
    return request<AlertRule>(
      MONITORING_BASE_URL,
      "/admin/alert-rules",
      {
        method: "POST",
        body: JSON.stringify(payload),
      },
      token,
    );
  },
};

export function realtimeUrl(
  stream: "logs" | "alerts",
  token: string,
  filters: { app?: string[]; level?: string[] } = {},
) {
  const base = new URL(
    `${MONITORING_BASE_URL.replace(/^http/, "ws")}/realtime/${stream}`,
  );

  base.searchParams.set("token", token);
  if (filters.app?.length) {
    base.searchParams.set("app", filters.app.join(","));
  }
  if (filters.level?.length) {
    base.searchParams.set("level", filters.level.join(","));
  }

  return base.toString();
}
