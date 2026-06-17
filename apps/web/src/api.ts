import type {
  Agent,
  AIConfig,
  AuditLog,
  ContinuousProfile,
  ContinuousTrend,
  ContinuousWindowFilters,
  ContinuousWindow,
  ContinuousWindowSummary,
  CreateContinuousProfileInput,
  CreateTaskInput,
  LoginRequest,
  LoginResponse,
  Task,
  UpdateAIConfigInput,
  UserProfile,
} from "./types";

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "";
const AUTH_TOKEN_STORAGE_KEY = "mini-drop-demo-auth-token";
const AUTH_USER_STORAGE_KEY = "mini-drop-demo-auth-user";

export function getStoredAuthToken() {
  return window.localStorage.getItem(AUTH_TOKEN_STORAGE_KEY) ?? "";
}

export function getStoredUserProfile(): UserProfile | null {
  const raw = window.localStorage.getItem(AUTH_USER_STORAGE_KEY);
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw) as UserProfile;
  } catch {
    return null;
  }
}

export function storeAuthSession(session: LoginResponse) {
  window.localStorage.setItem(AUTH_TOKEN_STORAGE_KEY, session.token);
  window.localStorage.setItem(AUTH_USER_STORAGE_KEY, JSON.stringify(session.user));
}

export function clearAuthSession() {
  window.localStorage.removeItem(AUTH_TOKEN_STORAGE_KEY);
  window.localStorage.removeItem(AUTH_USER_STORAGE_KEY);
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getStoredAuthToken();
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(init?.headers ?? {}),
    },
  });

  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { error?: string } | null;
    throw new Error(payload?.error ?? `Request failed with status ${response.status}`);
  }

  return (await response.json()) as T;
}

export async function login(input: LoginRequest) {
  return request<LoginResponse>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function getCurrentUser() {
  return request<{ user: UserProfile }>("/api/v1/auth/me");
}

export async function getAgents() {
  return request<{ agents: Agent[] }>("/api/v1/agents");
}

export async function getTasks() {
  return request<{ tasks: Task[] }>("/api/v1/tasks");
}

export async function getTask(taskId: string) {
  return request<{ task: Task }>(`/api/v1/tasks/${taskId}`);
}

export async function createTask(input: CreateTaskInput) {
  return request<{ task: Task }>("/api/v1/tasks", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function getContinuousProfiles() {
  return request<{ profiles: ContinuousProfile[] }>("/api/v1/continuous-profiles");
}

export async function createContinuousProfile(input: CreateContinuousProfileInput) {
  return request<{ profile: ContinuousProfile }>("/api/v1/continuous-profiles", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function setContinuousProfileEnabled(profileId: string, enabled: boolean) {
  return request<{ profile: ContinuousProfile }>(`/api/v1/continuous-profiles/${profileId}/${enabled ? "enable" : "disable"}`, {
    method: "POST",
  });
}

export async function getContinuousProfileWindows(profileId: string, filters: ContinuousWindowFilters = {}) {
  const params = new URLSearchParams();
  if (filters.status && filters.status !== "ALL") {
    params.set("status", filters.status);
  }
  if (filters.from) {
    params.set("from", filters.from);
  }
  if (filters.to) {
    params.set("to", filters.to);
  }
  if (filters.limit) {
    params.set("limit", String(filters.limit));
  }
  const query = params.toString();
  return request<{ windows: ContinuousWindow[]; summary: ContinuousWindowSummary }>(
    `/api/v1/continuous-profiles/${profileId}/windows${query ? `?${query}` : ""}`,
  );
}

export async function getContinuousProfileTrends(profileId: string, limit = 12) {
  const params = new URLSearchParams({ limit: String(limit) });
  return request<ContinuousTrend>(`/api/v1/continuous-profiles/${profileId}/trends?${params.toString()}`);
}

export async function getAuditLogs() {
  return request<{ audit_logs: AuditLog[] }>("/api/v1/audit-logs");
}

export async function getAIConfig() {
  return request<AIConfig>("/api/v1/ai-config");
}

export async function updateAIConfig(input: UpdateAIConfigInput) {
  return request<AIConfig>("/api/v1/ai-config", {
    method: "POST",
    body: JSON.stringify(input),
  });
}
