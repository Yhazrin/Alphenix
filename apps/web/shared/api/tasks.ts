import type {
  AgentTask,
  TaskMessagePayload,
  TaskDependency,
  AddDependencyRequest,
  TaskCheckpoint,
  SaveCheckpointRequest,
  ChainTaskRequest,
  SubmitReviewRequest,
} from "@/shared/types";
import type { Logger } from "@/shared/logger";
import { noopLogger } from "@/shared/logger";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "";

let _token: string | null = null;
let _workspaceId: string | null = null;
let _logger: Logger = noopLogger;

export function configureTasksApi(opts: {
  token?: string | null;
  workspaceId?: string | null;
  logger?: Logger;
}) {
  if (opts.token !== undefined) _token = opts.token;
  if (opts.workspaceId !== undefined) _workspaceId = opts.workspaceId;
  if (opts.logger !== undefined) _logger = opts.logger;
}

function authHeaders(): Record<string, string> {
  const headers: Record<string, string> = {};
  if (_token) headers["Authorization"] = `Bearer ${_token}`;
  if (_workspaceId) headers["X-Workspace-ID"] = _workspaceId;
  return headers;
}

function handleUnauthorized() {
  if (typeof window !== "undefined") {
    localStorage.removeItem("multica_token");
    localStorage.removeItem("multica_workspace_id");
    _token = null;
    _workspaceId = null;
    if (window.location.pathname !== "/") {
      window.location.href = "/";
    }
  }
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const rid = crypto.randomUUID().slice(0, 8);
  const start = Date.now();
  const method = init?.method ?? "GET";

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "X-Request-ID": rid,
    ...authHeaders(),
    ...((init?.headers as Record<string, string>) ?? {}),
  };

  _logger.info(`→ ${method} ${path}`, { rid });

  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
    credentials: "include",
  });

  if (!res.ok) {
    if (res.status === 401) handleUnauthorized();
    let message = `API error: ${res.status} ${res.statusText}`;
    try {
      const data = await res.json() as { error?: string };
      if (typeof data.error === "string" && data.error) message = `API error: ${data.error}`;
    } catch { /* ignore */ }
    _logger.error(`← ${res.status} ${path}`, { rid, duration: `${Date.now() - start}ms`, error: message });
    throw new Error(message);
  }

  _logger.info(`← ${res.status} ${path}`, { rid, duration: `${Date.now() - start}ms` });

  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const tasksApi = {
  async getTask(taskId: string): Promise<AgentTask> {
    return apiFetch(`/api/tasks/${taskId}`);
  },

  async getReadyTasks(): Promise<AgentTask[]> {
    return apiFetch("/api/tasks/ready");
  },

  async listByIssue(issueId: string): Promise<AgentTask[]> {
    return apiFetch(`/api/issues/${issueId}/task-runs`);
  },

  async getActiveTaskForIssue(issueId: string): Promise<{ task: AgentTask | null }> {
    return apiFetch(`/api/issues/${issueId}/active-task`);
  },

  async enqueueForIssue(issueId: string, data: unknown): Promise<AgentTask> {
    return apiFetch(`/api/issues/${issueId}/tasks`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async claim(taskId: string): Promise<AgentTask> {
    return apiFetch(`/api/tasks/${taskId}/claim`, { method: "POST" });
  },

  async start(taskId: string): Promise<AgentTask> {
    return apiFetch(`/api/tasks/${taskId}/start`, { method: "POST" });
  },

  async complete(taskId: string, result?: unknown): Promise<AgentTask> {
    return apiFetch(`/api/tasks/${taskId}/complete`, {
      method: "POST",
      body: JSON.stringify(result ?? {}),
    });
  },

  async fail(taskId: string, error?: string): Promise<AgentTask> {
    return apiFetch(`/api/tasks/${taskId}/fail`, {
      method: "POST",
      body: JSON.stringify({ error }),
    });
  },

  async cancel(issueId: string, taskId: string): Promise<AgentTask> {
    return apiFetch(`/api/issues/${issueId}/tasks/${taskId}/cancel`, { method: "POST" });
  },

  async listMessages(taskId: string): Promise<TaskMessagePayload[]> {
    return apiFetch(`/api/daemon/tasks/${taskId}/messages`);
  },

  async addDependency(taskId: string, data: AddDependencyRequest): Promise<TaskDependency> {
    return apiFetch(`/api/tasks/${taskId}/dependencies`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async removeDependency(taskId: string, data: AddDependencyRequest): Promise<{ status: string }> {
    return apiFetch(`/api/tasks/${taskId}/dependencies`, {
      method: "DELETE",
      body: JSON.stringify(data),
    });
  },

  async listDependencies(taskId: string): Promise<TaskDependency[]> {
    return apiFetch(`/api/tasks/${taskId}/dependencies`);
  },

  async saveCheckpoint(taskId: string, data: SaveCheckpointRequest): Promise<TaskCheckpoint> {
    return apiFetch(`/api/tasks/${taskId}/checkpoints`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async listCheckpoints(taskId: string): Promise<TaskCheckpoint[]> {
    return apiFetch(`/api/tasks/${taskId}/checkpoints`);
  },

  async getLatestCheckpoint(taskId: string): Promise<TaskCheckpoint> {
    return apiFetch(`/api/tasks/${taskId}/checkpoints/latest`);
  },

  async chainTask(taskId: string, data: ChainTaskRequest): Promise<AgentTask> {
    return apiFetch(`/api/tasks/${taskId}/chain`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async submitReview(taskId: string, data: SubmitReviewRequest): Promise<AgentTask> {
    return apiFetch(`/api/tasks/${taskId}/review`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  },
};
