import type {
  Skill,
  CreateSkillRequest,
  UpdateSkillRequest,
} from "@/shared/types";
import type { Logger } from "@/shared/logger";
import { noopLogger } from "@/shared/logger";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "";

let _token: string | null = null;
let _workspaceId: string | null = null;
let _logger: Logger = noopLogger;

export function configureSkillsApi(opts: {
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
    localStorage.removeItem("multicode_token");
    localStorage.removeItem("multicode_workspace_id");
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

export const skillsApi = {
  async list(): Promise<Skill[]> {
    return apiFetch("/api/skills");
  },

  async get(id: string): Promise<Skill> {
    return apiFetch(`/api/skills/${id}`);
  },

  async create(data: CreateSkillRequest): Promise<Skill> {
    return apiFetch("/api/skills", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async update(id: string, data: UpdateSkillRequest): Promise<Skill> {
    return apiFetch(`/api/skills/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  },

  async delete(id: string): Promise<void> {
    await apiFetch(`/api/skills/${id}`, { method: "DELETE" });
  },

  async importSkill(data: { url: string }): Promise<Skill> {
    return apiFetch("/api/skills/import", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },
};
