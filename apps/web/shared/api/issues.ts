import type {
  Issue,
  CreateIssueRequest,
  UpdateIssueRequest,
  ListIssuesResponse,
  ListIssuesParams,
  Comment,
  TimelineEntry,
  Reaction,
  IssueReaction,
  IssueSubscriber,
  Attachment,
} from "@/shared/types";
import type { Logger } from "@/shared/logger";
import { noopLogger } from "@/shared/logger";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "";

let _token: string | null = null;
let _workspaceId: string | null = null;
let _logger: Logger = noopLogger;

export function configureIssuesApi(opts: {
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

export const issuesApi = {
  async listIssues(params?: ListIssuesParams): Promise<ListIssuesResponse> {
    const search = new URLSearchParams();
    if (params?.limit) search.set("limit", String(params.limit));
    if (params?.offset) search.set("offset", String(params.offset));
    const wsId = params?.workspace_id ?? _workspaceId;
    if (wsId) search.set("workspace_id", wsId);
    if (params?.status) search.set("status", params.status);
    if (params?.priority) search.set("priority", params.priority);
    if (params?.assignee_id) search.set("assignee_id", params.assignee_id);
    return apiFetch(`/api/issues?${search}`);
  },

  async getIssue(id: string): Promise<Issue> {
    return apiFetch(`/api/issues/${id}`);
  },

  async createIssue(data: CreateIssueRequest): Promise<Issue> {
    const search = new URLSearchParams();
    if (_workspaceId) search.set("workspace_id", _workspaceId);
    return apiFetch(`/api/issues?${search}`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async updateIssue(id: string, data: UpdateIssueRequest): Promise<Issue> {
    return apiFetch(`/api/issues/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  },

  async deleteIssue(id: string): Promise<void> {
    await apiFetch(`/api/issues/${id}`, { method: "DELETE" });
  },

  async batchUpdateIssues(issueIds: string[], updates: UpdateIssueRequest): Promise<{ updated: number }> {
    return apiFetch("/api/issues/batch-update", {
      method: "POST",
      body: JSON.stringify({ issue_ids: issueIds, updates }),
    });
  },

  async batchDeleteIssues(issueIds: string[]): Promise<{ deleted: number }> {
    return apiFetch("/api/issues/batch-delete", {
      method: "POST",
      body: JSON.stringify({ issue_ids: issueIds }),
    });
  },

  async resolveIssue(id: string): Promise<Issue> {
    return apiFetch(`/api/issues/${id}`, {
      method: "PUT",
      body: JSON.stringify({ status: "done" }),
    });
  },

  async reopenIssue(id: string): Promise<Issue> {
    return apiFetch(`/api/issues/${id}`, {
      method: "PUT",
      body: JSON.stringify({ status: "todo" }),
    });
  },

  async listComments(issueId: string): Promise<Comment[]> {
    return apiFetch(`/api/issues/${issueId}/comments`);
  },

  async createComment(
    issueId: string,
    content: string,
    type = "comment",
    parentId?: string,
    attachmentIds?: string[],
  ): Promise<Comment> {
    return apiFetch(`/api/issues/${issueId}/comments`, {
      method: "POST",
      body: JSON.stringify({
        content,
        type,
        ...(parentId ? { parent_id: parentId } : {}),
        ...(attachmentIds?.length ? { attachment_ids: attachmentIds } : {}),
      }),
    });
  },

  async listTimeline(issueId: string): Promise<TimelineEntry[]> {
    return apiFetch(`/api/issues/${issueId}/timeline`);
  },

  async updateComment(commentId: string, content: string): Promise<Comment> {
    return apiFetch(`/api/comments/${commentId}`, {
      method: "PUT",
      body: JSON.stringify({ content }),
    });
  },

  async deleteComment(commentId: string): Promise<void> {
    await apiFetch(`/api/comments/${commentId}`, { method: "DELETE" });
  },

  async addReaction(commentId: string, emoji: string): Promise<Reaction> {
    return apiFetch(`/api/comments/${commentId}/reactions`, {
      method: "POST",
      body: JSON.stringify({ emoji }),
    });
  },

  async removeReaction(commentId: string, emoji: string): Promise<void> {
    await apiFetch(`/api/comments/${commentId}/reactions`, {
      method: "DELETE",
      body: JSON.stringify({ emoji }),
    });
  },

  async addIssueReaction(issueId: string, emoji: string): Promise<IssueReaction> {
    return apiFetch(`/api/issues/${issueId}/reactions`, {
      method: "POST",
      body: JSON.stringify({ emoji }),
    });
  },

  async removeIssueReaction(issueId: string, emoji: string): Promise<void> {
    await apiFetch(`/api/issues/${issueId}/reactions`, {
      method: "DELETE",
      body: JSON.stringify({ emoji }),
    });
  },

  async listSubscribers(issueId: string): Promise<IssueSubscriber[]> {
    return apiFetch(`/api/issues/${issueId}/subscribers`);
  },

  async subscribe(issueId: string, userId?: string, userType?: string): Promise<void> {
    const body: Record<string, string> = {};
    if (userId) body.user_id = userId;
    if (userType) body.user_type = userType;
    await apiFetch(`/api/issues/${issueId}/subscribe`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  },

  async unsubscribe(issueId: string, userId?: string, userType?: string): Promise<void> {
    const body: Record<string, string> = {};
    if (userId) body.user_id = userId;
    if (userType) body.user_type = userType;
    await apiFetch(`/api/issues/${issueId}/unsubscribe`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  },

  async listAttachments(issueId: string): Promise<Attachment[]> {
    return apiFetch(`/api/issues/${issueId}/attachments`);
  },
};
