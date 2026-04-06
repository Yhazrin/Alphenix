import type { Run, RunStep, RunTodo, RunArtifact } from "@/shared/types";
import type { Logger } from "@/shared/logger";
import { noopLogger } from "@/shared/logger";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "";

let _token: string | null = null;
let _workspaceId: string | null = null;
let _logger: Logger = noopLogger;

export function configureRunsApi(opts: {
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
      const data = (await res.json()) as { error?: string };
      if (typeof data.error === "string" && data.error)
        message = `API error: ${data.error}`;
    } catch {
      /* ignore */
    }
    _logger.error(`← ${res.status} ${path}`, {
      rid,
      duration: `${Date.now() - start}ms`,
      error: message,
    });
    throw new Error(message);
  }

  _logger.info(`← ${res.status} ${path}`, {
    rid,
    duration: `${Date.now() - start}ms`,
  });

  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const runsApi = {
  async listRuns(limit = 50, offset = 0): Promise<Run[]> {
    return apiFetch(`/api/runs?limit=${limit}&offset=${offset}`);
  },

  async getRun(runId: string): Promise<Run> {
    return apiFetch(`/api/runs/${runId}`);
  },

  async createRun(data: {
    workspace_id: string;
    issue_id: string;
    agent_id: string;
    task_id?: string;
    parent_run_id?: string;
    team_id?: string;
    system_prompt: string;
    model_name: string;
    permission_mode: string;
  }): Promise<Run> {
    return apiFetch("/api/runs", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async startRun(runId: string): Promise<Run> {
    return apiFetch(`/api/runs/${runId}/start`, { method: "POST" });
  },

  async cancelRun(runId: string): Promise<Run> {
    return apiFetch(`/api/runs/${runId}/cancel`, { method: "POST" });
  },

  async completeRun(runId: string): Promise<Run> {
    return apiFetch(`/api/runs/${runId}/complete`, { method: "POST" });
  },

  async listRunsByIssue(issueId: string): Promise<Run[]> {
    return apiFetch(`/api/runs/by-issue/${issueId}`);
  },

  async getRunSteps(runId: string): Promise<RunStep[]> {
    return apiFetch(`/api/runs/${runId}/steps`);
  },

  async recordStep(
    runId: string,
    data: {
      tool_name: string;
      tool_input: Record<string, unknown>;
      tool_output?: string;
      is_error?: boolean;
    },
  ): Promise<RunStep> {
    return apiFetch(`/api/runs/${runId}/steps`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async getRunTodos(runId: string): Promise<RunTodo[]> {
    return apiFetch(`/api/runs/${runId}/todos`);
  },

  async createRunTodo(
    runId: string,
    data: { title: string; description?: string },
  ): Promise<RunTodo> {
    return apiFetch(`/api/runs/${runId}/todos`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async updateRunTodo(
    todoId: string,
    data: { status?: string; blocker?: string },
  ): Promise<RunTodo> {
    return apiFetch(`/api/runs/todos/${todoId}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  },

  async getRunArtifacts(runId: string): Promise<RunArtifact[]> {
    return apiFetch(`/api/runs/${runId}/artifacts`);
  },
};
