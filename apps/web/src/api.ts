import type {
  Agent,
  AuditLog,
  ContinuousProfile,
  ContinuousWindow,
  CreateContinuousProfileInput,
  CreateTaskInput,
  Task,
} from "./types";

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "http://127.0.0.1:8080";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
  });

  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { error?: string } | null;
    throw new Error(payload?.error ?? `Request failed with status ${response.status}`);
  }

  return (await response.json()) as T;
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

export async function getContinuousProfileWindows(profileId: string) {
  return request<{ windows: ContinuousWindow[] }>(`/api/v1/continuous-profiles/${profileId}/windows`);
}

export async function getAuditLogs() {
  return request<{ audit_logs: AuditLog[] }>("/api/v1/audit-logs");
}
