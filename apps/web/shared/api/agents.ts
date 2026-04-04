import type {
  Agent,
  CreateAgentRequest,
  UpdateAgentRequest,
  AgentTask,
  Skill,
  SetAgentSkillsRequest,
  AgentMessage,
  SendMessageRequest,
  AgentMemory,
  StoreMemoryRequest,
  RecallMemoryRequest,
  PromptPreviewResponse,
  TaskContextPreviewResponse,
} from "@/shared/types";
import type { Logger } from "@/shared/logger";
import { noopLogger } from "@/shared/logger";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "";

let _token: string | null = null;
let _workspaceId: string | null = null;
let _logger: Logger = noopLogger;

export function configureAgentsApi(opts: {
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

export const agentsApi = {
  async list(params?: { workspace_id?: string; include_archived?: boolean }): Promise<Agent[]> {
    const search = new URLSearchParams();
    const wsId = params?.workspace_id ?? _workspaceId;
    if (wsId) search.set("workspace_id", wsId);
    if (params?.include_archived) search.set("include_archived", "true");
    return apiFetch(`/api/agents?${search}`);
  },

  async get(id: string): Promise<Agent> {
    return apiFetch(`/api/agents/${id}`);
  },

  async create(data: CreateAgentRequest): Promise<Agent> {
    return apiFetch("/api/agents", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async update(id: string, data: UpdateAgentRequest): Promise<Agent> {
    return apiFetch(`/api/agents/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  },

  async archive(id: string): Promise<Agent> {
    return apiFetch(`/api/agents/${id}/archive`, { method: "POST" });
  },

  async restore(id: string): Promise<Agent> {
    return apiFetch(`/api/agents/${id}/restore`, { method: "POST" });
  },

  async listTasks(agentId: string): Promise<AgentTask[]> {
    return apiFetch(`/api/agents/${agentId}/tasks`);
  },

  async listSkills(agentId: string): Promise<Skill[]> {
    return apiFetch(`/api/agents/${agentId}/skills`);
  },

  async setSkills(agentId: string, data: SetAgentSkillsRequest): Promise<void> {
    await apiFetch(`/api/agents/${agentId}/skills`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  },

  async sendMessage(agentId: string, data: SendMessageRequest): Promise<AgentMessage> {
    return apiFetch(`/api/agents/${agentId}/messages`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async listMessages(agentId: string, params?: { unread?: boolean; task_id?: string }): Promise<AgentMessage[]> {
    const search = new URLSearchParams();
    if (params?.unread) search.set("unread", "true");
    if (params?.task_id) search.set("task_id", params.task_id);
    return apiFetch(`/api/agents/${agentId}/messages?${search}`);
  },

  async markMessagesRead(agentId: string): Promise<{ status: string }> {
    return apiFetch(`/api/agents/${agentId}/messages/read`, { method: "POST" });
  },

  async storeMemory(agentId: string, data: StoreMemoryRequest): Promise<AgentMemory> {
    return apiFetch(`/api/agents/${agentId}/memory`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async recallMemory(agentId: string, data: RecallMemoryRequest): Promise<AgentMemory[]> {
    return apiFetch(`/api/agents/${agentId}/memory/recall`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async listMemory(agentId: string): Promise<AgentMemory[]> {
    return apiFetch(`/api/agents/${agentId}/memory`);
  },

  async deleteMemory(agentId: string, memoryId: string): Promise<{ status: string }> {
    return apiFetch(`/api/agents/${agentId}/memory/${memoryId}`, { method: "DELETE" });
  },

  async previewPrompt(agentId: string): Promise<PromptPreviewResponse> {
    return apiFetch(`/api/agents/${agentId}/prompt-preview`);
  },

  async previewTaskContext(taskId: string): Promise<TaskContextPreviewResponse> {
    return apiFetch(`/api/tasks/${taskId}/context-preview`);
  },
};
