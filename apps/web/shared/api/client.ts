import type {
  Issue,
  CreateIssueRequest,
  UpdateIssueRequest,
  ListIssuesResponse,
  UpdateMeRequest,
  CreateMemberRequest,
  UpdateMemberRequest,
  ListIssuesParams,
  Agent,
  CreateAgentRequest,
  UpdateAgentRequest,
  AgentTask,
  AgentRuntime,
  Team,
  CreateTeamRequest,
  UpdateTeamRequest,
  InboxItem,
  IssueSubscriber,
  Comment,
  Reaction,
  IssueReaction,
  Workspace,
  WorkspaceRepo,
  MemberWithUser,
  User,
  Skill,
  CreateSkillRequest,
  UpdateSkillRequest,
  SetAgentSkillsRequest,
  PersonalAccessToken,
  CreatePersonalAccessTokenRequest,
  CreatePersonalAccessTokenResponse,
  RuntimeUsage,
  RuntimeHourlyActivity,
  RuntimePing,
  RuntimeUpdate,
  TimelineEntry,
  TaskMessagePayload,
  Attachment,
  AgentMessage,
  SendMessageRequest,
  TaskDependency,
  AddDependencyRequest,
  TaskCheckpoint,
  SaveCheckpointRequest,
  AgentMemory,
  StoreMemoryRequest,

  ChainTaskRequest,
  SubmitReviewRequest,
  CreateWorkspaceRepoRequest,
  UpdateWorkspaceRepoRequest,
  IssueDependency,
  DecomposeResponse,
  ConfirmDecomposeRequest,
  ConfirmDecomposeResponse,
} from "@/shared/types";
import { type Logger, noopLogger } from "@/shared/logger";

export interface LoginResponse {
  token: string;
  user: User;
}

export class ApiClient {
  private baseUrl: string;
  private token: string | null = null;
  private workspaceId: string | null = null;
  private logger: Logger;

  constructor(baseUrl: string, options?: { logger?: Logger }) {
    this.baseUrl = baseUrl;
    this.logger = options?.logger ?? noopLogger;
  }

  setToken(token: string | null) {
    this.token = token;
  }

  setWorkspaceId(id: string | null) {
    this.workspaceId = id;
  }

  private authHeaders(): Record<string, string> {
    const headers: Record<string, string> = {};
    if (this.token) headers["Authorization"] = `Bearer ${this.token}`;
    if (this.workspaceId) headers["X-Workspace-ID"] = this.workspaceId;
    return headers;
  }

  private handleUnauthorized() {
    if (typeof window !== "undefined") {
      this.token = null;
      this.workspaceId = null;
      if (window.location.pathname !== "/") {
        window.location.href = "/";
      }
    }
  }

  private async parseErrorMessage(res: Response, fallback: string): Promise<string> {
    try {
      const data = await res.json() as { error?: string };
      if (typeof data.error === "string" && data.error) return data.error;
    } catch {
      // Ignore non-JSON error bodies.
    }
    return fallback;
  }

  private async fetch<T>(path: string, init?: RequestInit): Promise<T> {
    const rid = crypto.randomUUID().slice(0, 8);
    const start = Date.now();
    const method = init?.method ?? "GET";
    const isGet = method === "GET";
    const maxRetries = isGet ? 3 : 0;

    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      "X-Request-ID": rid,
      ...this.authHeaders(),
      ...((init?.headers as Record<string, string>) ?? {}),
    };

    this.logger.info(`→ ${method} ${path}`, { rid });

    let lastError: Error | null = null;
    for (let attempt = 0; attempt <= maxRetries; attempt++) {
      if (attempt > 0) {
        const delay = Math.min(1000 * 2 ** (attempt - 1), 5000);
        await new Promise((r) => setTimeout(r, delay));
        this.logger.info(`↻ retry ${attempt}/${maxRetries} ${method} ${path}`, { rid });
      }

      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 30000);
      try {
        const res = await fetch(`${this.baseUrl}${path}`, {
          ...init,
          headers,
          credentials: "include",
          signal: controller.signal,
        });

        if (!res.ok) {
          if (res.status === 401) this.handleUnauthorized();
          const message = await this.parseErrorMessage(res, `API error: ${res.status} ${res.statusText}`);
          // Don't retry client errors (4xx except 408/429)
          if (res.status >= 400 && res.status < 500 && res.status !== 408 && res.status !== 429) {
            this.logger.error(`← ${res.status} ${path}`, { rid, duration: `${Date.now() - start}ms`, error: message });
            throw new Error(`API error: ${message}`);
          }
          lastError = new Error(message);
          continue;
        }

        this.logger.info(`← ${res.status} ${path}`, { rid, duration: `${Date.now() - start}ms` });

        // Handle 204 No Content
        if (res.status === 204) {
          return undefined as T;
        }

        return res.json() as Promise<T>;
      } catch (err) {
        if (err instanceof Error && err.message.startsWith("API error:")) throw err;
        lastError = err instanceof Error ? err : new Error("Network error");
      } finally {
        clearTimeout(timeout);
      }
    }

    this.logger.error(`✗ ${method} ${path} failed after ${maxRetries + 1} attempts`, { rid, duration: `${Date.now() - start}ms` });
    throw lastError ?? new Error("Request failed");
  }

  // Auth
  async sendCode(email: string): Promise<void> {
    await this.fetch("/auth/send-code", {
      method: "POST",
      body: JSON.stringify({ email }),
    });
  }

  async verifyCode(email: string, code: string): Promise<LoginResponse> {
    return this.fetch("/auth/verify-code", {
      method: "POST",
      body: JSON.stringify({ email, code }),
    });
  }

  async getMe(): Promise<User> {
    return this.fetch("/api/me");
  }

  async updateMe(data: UpdateMeRequest): Promise<User> {
    return this.fetch("/api/me", {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  }

  async logout(): Promise<void> {
    await this.fetch("/auth/logout", { method: "POST" });
    this.token = null;
    this.workspaceId = null;
  }

  // Issues
  async searchIssues(query: string, limit?: number): Promise<ListIssuesResponse> {
    const search = new URLSearchParams();
    search.set("q", query);
    if (limit !== undefined) search.set("limit", String(limit));
    return this.fetch(`/api/issues/search?${search}`);
  }

  async listIssues(params?: ListIssuesParams): Promise<ListIssuesResponse> {
    const search = new URLSearchParams();
    if (params?.limit !== undefined) search.set("limit", String(params.limit));
    if (params?.offset !== undefined) search.set("offset", String(params.offset));
    if (params?.cursor) search.set("cursor", params.cursor);
    const wsId = params?.workspace_id ?? this.workspaceId;
    if (wsId) search.set("workspace_id", wsId);
    if (params?.status) search.set("status", params.status);
    if (params?.priority) search.set("priority", params.priority);
    if (params?.assignee_id) search.set("assignee_id", params.assignee_id);
    return this.fetch(`/api/issues?${search}`);
  }

  async getIssue(id: string): Promise<Issue> {
    return this.fetch(`/api/issues/${id}`);
  }

  async createIssue(data: CreateIssueRequest): Promise<Issue> {
    const search = new URLSearchParams();
    if (this.workspaceId) search.set("workspace_id", this.workspaceId);
    return this.fetch(`/api/issues?${search}`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async updateIssue(id: string, data: UpdateIssueRequest): Promise<Issue> {
    return this.fetch(`/api/issues/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  async deleteIssue(id: string): Promise<void> {
    await this.fetch(`/api/issues/${id}`, { method: "DELETE" });
  }

  async batchUpdateIssues(issueIds: string[], updates: UpdateIssueRequest): Promise<{ updated: number }> {
    return this.fetch("/api/issues/batch-update", {
      method: "POST",
      body: JSON.stringify({ issue_ids: issueIds, updates }),
    });
  }

  async batchDeleteIssues(issueIds: string[]): Promise<{ deleted: number }> {
    return this.fetch("/api/issues/batch-delete", {
      method: "POST",
      body: JSON.stringify({ issue_ids: issueIds }),
    });
  }

  // Comments
  async listComments(issueId: string): Promise<Comment[]> {
    return this.fetch(`/api/issues/${issueId}/comments`);
  }

  async createComment(issueId: string, content: string, type?: string, parentId?: string, attachmentIds?: string[]): Promise<Comment> {
    return this.fetch(`/api/issues/${issueId}/comments`, {
      method: "POST",
      body: JSON.stringify({
        content,
        type: type ?? "comment",
        ...(parentId ? { parent_id: parentId } : {}),
        ...(attachmentIds?.length ? { attachment_ids: attachmentIds } : {}),
      }),
    });
  }

  async listTimeline(issueId: string): Promise<TimelineEntry[]> {
    return this.fetch(`/api/issues/${issueId}/timeline`);
  }

  async updateComment(commentId: string, content: string): Promise<Comment> {
    return this.fetch(`/api/comments/${commentId}`, {
      method: "PUT",
      body: JSON.stringify({ content }),
    });
  }

  async deleteComment(commentId: string): Promise<void> {
    await this.fetch(`/api/comments/${commentId}`, { method: "DELETE" });
  }

  async addReaction(commentId: string, emoji: string): Promise<Reaction> {
    return this.fetch(`/api/comments/${commentId}/reactions`, {
      method: "POST",
      body: JSON.stringify({ emoji }),
    });
  }

  async removeReaction(commentId: string, emoji: string): Promise<void> {
    await this.fetch(`/api/comments/${commentId}/reactions`, {
      method: "DELETE",
      body: JSON.stringify({ emoji }),
    });
  }

  async addIssueReaction(issueId: string, emoji: string): Promise<IssueReaction> {
    return this.fetch(`/api/issues/${issueId}/reactions`, {
      method: "POST",
      body: JSON.stringify({ emoji }),
    });
  }

  async removeIssueReaction(issueId: string, emoji: string): Promise<void> {
    await this.fetch(`/api/issues/${issueId}/reactions`, {
      method: "DELETE",
      body: JSON.stringify({ emoji }),
    });
  }

  // Subscribers
  async listIssueSubscribers(issueId: string): Promise<IssueSubscriber[]> {
    return this.fetch(`/api/issues/${issueId}/subscribers`);
  }

  async subscribeToIssue(issueId: string, userId?: string, userType?: string): Promise<void> {
    const body: Record<string, string> = {};
    if (userId) body.user_id = userId;
    if (userType) body.user_type = userType;
    await this.fetch(`/api/issues/${issueId}/subscribe`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async unsubscribeFromIssue(issueId: string, userId?: string, userType?: string): Promise<void> {
    const body: Record<string, string> = {};
    if (userId) body.user_id = userId;
    if (userType) body.user_type = userType;
    await this.fetch(`/api/issues/${issueId}/unsubscribe`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  // Agents
  async listAgents(params?: { workspace_id?: string; include_archived?: boolean }): Promise<Agent[]> {
    const search = new URLSearchParams();
    const wsId = params?.workspace_id ?? this.workspaceId;
    if (wsId) search.set("workspace_id", wsId);
    if (params?.include_archived) search.set("include_archived", "true");
    return this.fetch(`/api/agents?${search}`);
  }

  async getAgent(id: string): Promise<Agent> {
    return this.fetch(`/api/agents/${id}`);
  }

  async createAgent(data: CreateAgentRequest): Promise<Agent> {
    return this.fetch("/api/agents", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async updateAgent(id: string, data: UpdateAgentRequest): Promise<Agent> {
    return this.fetch(`/api/agents/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  async archiveAgent(id: string): Promise<Agent> {
    return this.fetch(`/api/agents/${id}/archive`, { method: "POST" });
  }

  async restoreAgent(id: string): Promise<Agent> {
    return this.fetch(`/api/agents/${id}/restore`, { method: "POST" });
  }

  async listRuntimes(params?: { workspace_id?: string }): Promise<AgentRuntime[]> {
    const search = new URLSearchParams();
    const wsId = params?.workspace_id ?? this.workspaceId;
    if (wsId) search.set("workspace_id", wsId);
    return this.fetch(`/api/runtimes?${search}`);
  }

  async getRuntimeUsage(runtimeId: string, params?: { days?: number }): Promise<RuntimeUsage[]> {
    const search = new URLSearchParams();
    if (params?.days !== undefined) search.set("days", String(params.days));
    return this.fetch(`/api/runtimes/${runtimeId}/usage?${search}`);
  }

  async getRuntimeTaskActivity(runtimeId: string): Promise<RuntimeHourlyActivity[]> {
    return this.fetch(`/api/runtimes/${runtimeId}/activity`);
  }

  async pingRuntime(runtimeId: string): Promise<RuntimePing> {
    return this.fetch(`/api/runtimes/${runtimeId}/ping`, { method: "POST" });
  }

  async getPingResult(runtimeId: string, pingId: string): Promise<RuntimePing> {
    return this.fetch(`/api/runtimes/${runtimeId}/ping/${pingId}`);
  }

  async initiateUpdate(
    runtimeId: string,
    targetVersion: string,
  ): Promise<RuntimeUpdate> {
    return this.fetch(`/api/runtimes/${runtimeId}/update`, {
      method: "POST",
      body: JSON.stringify({ target_version: targetVersion }),
    });
  }

  async getUpdateResult(
    runtimeId: string,
    updateId: string,
  ): Promise<RuntimeUpdate> {
    return this.fetch(`/api/runtimes/${runtimeId}/update/${updateId}`);
  }

  async listAgentTasks(agentId: string): Promise<AgentTask[]> {
    return this.fetch(`/api/agents/${agentId}/tasks`);
  }

  async getActiveTaskForIssue(issueId: string): Promise<{ task: AgentTask | null }> {
    return this.fetch(`/api/issues/${issueId}/active-task`);
  }

  async listTaskMessages(taskId: string): Promise<TaskMessagePayload[]> {
    return this.fetch(`/api/daemon/tasks/${taskId}/messages`);
  }

  async listTasksByIssue(issueId: string): Promise<AgentTask[]> {
    return this.fetch(`/api/issues/${issueId}/task-runs`);
  }

  async cancelTask(issueId: string, taskId: string): Promise<AgentTask> {
    return this.fetch(`/api/issues/${issueId}/tasks/${taskId}/cancel`, {
      method: "POST",
    });
  }

  // Inbox
  async listInbox(): Promise<InboxItem[]> {
    return this.fetch("/api/inbox");
  }

  async markInboxRead(id: string): Promise<InboxItem> {
    return this.fetch(`/api/inbox/${id}/read`, { method: "POST" });
  }

  async archiveInbox(id: string): Promise<InboxItem> {
    return this.fetch(`/api/inbox/${id}/archive`, { method: "POST" });
  }

  async getUnreadInboxCount(): Promise<{ count: number }> {
    return this.fetch("/api/inbox/unread-count");
  }

  async markAllInboxRead(): Promise<{ count: number }> {
    return this.fetch("/api/inbox/mark-all-read", { method: "POST" });
  }

  async archiveAllInbox(): Promise<{ count: number }> {
    return this.fetch("/api/inbox/archive-all", { method: "POST" });
  }

  async archiveAllReadInbox(): Promise<{ count: number }> {
    return this.fetch("/api/inbox/archive-all-read", { method: "POST" });
  }

  async archiveCompletedInbox(): Promise<{ count: number }> {
    return this.fetch("/api/inbox/archive-completed", { method: "POST" });
  }

  // Workspaces
  async listWorkspaces(): Promise<Workspace[]> {
    return this.fetch("/api/workspaces");
  }

  async getWorkspace(id: string): Promise<Workspace> {
    return this.fetch(`/api/workspaces/${id}`);
  }

  async createWorkspace(data: { name: string; slug: string; description?: string; context?: string }): Promise<Workspace> {
    return this.fetch("/api/workspaces", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async updateWorkspace(id: string, data: { name?: string; description?: string; context?: string; issue_prefix?: string; settings?: Record<string, unknown>; repos?: WorkspaceRepo[] }): Promise<Workspace> {
    return this.fetch(`/api/workspaces/${id}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  }

  // Members
  async listMembers(workspaceId: string): Promise<MemberWithUser[]> {
    return this.fetch(`/api/workspaces/${workspaceId}/members`);
  }

  async createMember(workspaceId: string, data: CreateMemberRequest): Promise<MemberWithUser> {
    return this.fetch(`/api/workspaces/${workspaceId}/members`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async updateMember(workspaceId: string, memberId: string, data: UpdateMemberRequest): Promise<MemberWithUser> {
    return this.fetch(`/api/workspaces/${workspaceId}/members/${memberId}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  }

  async deleteMember(workspaceId: string, memberId: string): Promise<void> {
    await this.fetch(`/api/workspaces/${workspaceId}/members/${memberId}`, {
      method: "DELETE",
    });
  }

  async leaveWorkspace(workspaceId: string): Promise<void> {
    await this.fetch(`/api/workspaces/${workspaceId}/leave`, {
      method: "POST",
    });
  }

  async deleteWorkspace(workspaceId: string): Promise<void> {
    await this.fetch(`/api/workspaces/${workspaceId}`, {
      method: "DELETE",
    });
  }

  // Workspace Repos
  async listWorkspaceRepos(workspaceId: string): Promise<WorkspaceRepo[]> {
    return this.fetch(`/api/workspaces/${workspaceId}/repos`);
  }

  async createWorkspaceRepo(workspaceId: string, data: CreateWorkspaceRepoRequest): Promise<WorkspaceRepo> {
    return this.fetch(`/api/workspaces/${workspaceId}/repos`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async getWorkspaceRepo(workspaceId: string, repoId: string): Promise<WorkspaceRepo> {
    return this.fetch(`/api/workspaces/${workspaceId}/repos/${repoId}`);
  }

  async updateWorkspaceRepo(workspaceId: string, repoId: string, data: UpdateWorkspaceRepoRequest): Promise<WorkspaceRepo> {
    return this.fetch(`/api/workspaces/${workspaceId}/repos/${repoId}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  }

  async deleteWorkspaceRepo(workspaceId: string, repoId: string): Promise<void> {
    await this.fetch(`/api/workspaces/${workspaceId}/repos/${repoId}`, {
      method: "DELETE",
    });
  }

  async listIssuesByRepo(workspaceId: string, repoId: string): Promise<ListIssuesResponse> {
    return this.fetch(`/api/workspaces/${workspaceId}/repos/${repoId}/issues`);
  }

  // Skills
  async listSkills(): Promise<Skill[]> {
    return this.fetch("/api/skills");
  }

  async getSkill(id: string): Promise<Skill> {
    return this.fetch(`/api/skills/${id}`);
  }

  async createSkill(data: CreateSkillRequest): Promise<Skill> {
    return this.fetch("/api/skills", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async updateSkill(id: string, data: UpdateSkillRequest): Promise<Skill> {
    return this.fetch(`/api/skills/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  async deleteSkill(id: string): Promise<void> {
    await this.fetch(`/api/skills/${id}`, { method: "DELETE" });
  }

  async importSkill(data: { url: string }): Promise<Skill> {
    return this.fetch("/api/skills/import", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async listAgentSkills(agentId: string): Promise<Skill[]> {
    return this.fetch(`/api/agents/${agentId}/skills`);
  }

  async setAgentSkills(agentId: string, data: SetAgentSkillsRequest): Promise<void> {
    await this.fetch(`/api/agents/${agentId}/skills`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  // Teams
  async listTeams(params?: { workspace_id?: string }): Promise<Team[]> {
    const search = new URLSearchParams();
    const wsId = params?.workspace_id ?? this.workspaceId;
    if (wsId) search.set("workspace_id", wsId);
    return this.fetch(`/api/teams?${search}`);
  }

  async getTeam(id: string): Promise<Team> {
    return this.fetch(`/api/teams/${id}`);
  }

  async createTeam(data: CreateTeamRequest): Promise<Team> {
    return this.fetch("/api/teams", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async updateTeam(id: string, data: UpdateTeamRequest): Promise<Team> {
    return this.fetch(`/api/teams/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  async archiveTeam(id: string): Promise<Team> {
    return this.fetch(`/api/teams/${id}/archive`, { method: "POST" });
  }

  async restoreTeam(id: string): Promise<Team> {
    return this.fetch(`/api/teams/${id}/restore`, { method: "POST" });
  }

  async addTeamMember(teamId: string, agentId: string): Promise<void> {
    await this.fetch(`/api/teams/${teamId}/members`, {
      method: "POST",
      body: JSON.stringify({ agent_id: agentId }),
    });
  }

  async removeTeamMember(teamId: string, agentId: string): Promise<void> {
    await this.fetch(`/api/teams/${teamId}/members/${agentId}`, { method: "DELETE" });
  }

  async setTeamLead(teamId: string, agentId: string): Promise<void> {
    await this.fetch(`/api/teams/${teamId}/lead`, {
      method: "POST",
      body: JSON.stringify({ agent_id: agentId }),
    });
  }

  // Personal Access Tokens
  async listPersonalAccessTokens(): Promise<PersonalAccessToken[]> {
    return this.fetch("/api/tokens");
  }

  async createPersonalAccessToken(data: CreatePersonalAccessTokenRequest): Promise<CreatePersonalAccessTokenResponse> {
    return this.fetch("/api/tokens", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async revokePersonalAccessToken(id: string): Promise<void> {
    await this.fetch(`/api/tokens/${id}`, { method: "DELETE" });
  }

  // File Upload & Attachments
  async uploadFile(file: File, opts?: { issueId?: string; commentId?: string }): Promise<Attachment> {
    const formData = new FormData();
    formData.append("file", file);
    if (opts?.issueId) formData.append("issue_id", opts.issueId);
    if (opts?.commentId) formData.append("comment_id", opts.commentId);

    const rid = crypto.randomUUID().slice(0, 8);
    const start = Date.now();
    this.logger.info("→ POST /api/upload-file", { rid });

    const res = await fetch(`${this.baseUrl}/api/upload-file`, {
      method: "POST",
      headers: this.authHeaders(),
      body: formData,
      credentials: "include",
    });

    if (!res.ok) {
      if (res.status === 401) this.handleUnauthorized();
      const message = await this.parseErrorMessage(res, `Upload failed: ${res.status}`);
      this.logger.error(`← ${res.status} /api/upload-file`, { rid, duration: `${Date.now() - start}ms`, error: message });
      throw new Error(message);
    }

    this.logger.info(`← ${res.status} /api/upload-file`, { rid, duration: `${Date.now() - start}ms` });
    return res.json() as Promise<Attachment>;
  }

  async listAttachments(issueId: string): Promise<Attachment[]> {
    return this.fetch(`/api/issues/${issueId}/attachments`);
  }

  async deleteAttachment(id: string): Promise<void> {
    await this.fetch(`/api/attachments/${id}`, { method: "DELETE" });
  }

  // Agent Messaging
  async sendAgentMessage(agentId: string, data: SendMessageRequest): Promise<AgentMessage> {
    return this.fetch(`/api/agents/${agentId}/messages`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async listAgentMessages(agentId: string, params?: { unread?: boolean; task_id?: string }): Promise<AgentMessage[]> {
    const search = new URLSearchParams();
    if (params?.unread) search.set("unread", "true");
    if (params?.task_id) search.set("task_id", params.task_id);
    return this.fetch(`/api/agents/${agentId}/messages?${search}`);
  }

  async markAgentMessagesRead(agentId: string): Promise<{ status: string }> {
    return this.fetch(`/api/agents/${agentId}/messages/read`, { method: "POST" });
  }

  // Task Dependencies
  async addTaskDependency(taskId: string, data: AddDependencyRequest): Promise<TaskDependency> {
    return this.fetch(`/api/tasks/${taskId}/dependencies`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async removeTaskDependency(taskId: string, data: AddDependencyRequest): Promise<{ status: string }> {
    return this.fetch(`/api/tasks/${taskId}/dependencies`, {
      method: "DELETE",
      body: JSON.stringify(data),
    });
  }

  async listTaskDependencies(taskId: string): Promise<TaskDependency[]> {
    return this.fetch(`/api/tasks/${taskId}/dependencies`);
  }

  async getTask(taskId: string): Promise<AgentTask> {
    return this.fetch(`/api/tasks/${taskId}`);
  }

  async getReadyTasks(): Promise<AgentTask[]> {
    return this.fetch("/api/tasks/ready");
  }

  // Task Checkpoints
  async saveTaskCheckpoint(taskId: string, data: SaveCheckpointRequest): Promise<TaskCheckpoint> {
    return this.fetch(`/api/tasks/${taskId}/checkpoints`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async listTaskCheckpoints(taskId: string): Promise<TaskCheckpoint[]> {
    return this.fetch(`/api/tasks/${taskId}/checkpoints`);
  }

  async getLatestCheckpoint(taskId: string): Promise<TaskCheckpoint> {
    return this.fetch(`/api/tasks/${taskId}/checkpoints/latest`);
  }

  // Agent Memory
  async storeAgentMemory(agentId: string, data: StoreMemoryRequest): Promise<AgentMemory> {
    return this.fetch(`/api/agents/${agentId}/memory`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async listAgentMemory(agentId: string): Promise<AgentMemory[]> {
    return this.fetch(`/api/agents/${agentId}/memory`);
  }

  async deleteAgentMemory(agentId: string, memoryId: string): Promise<{ status: string }> {
    return this.fetch(`/api/agents/${agentId}/memory/${memoryId}`, { method: "DELETE" });
  }

  // Issue Decomposition
  async decomposeIssue(issueId: string): Promise<DecomposeResponse> {
    return this.fetch(`/api/issues/${issueId}/decompose`, { method: "POST" });
  }

  async getDecomposeResult(issueId: string, runId: string): Promise<DecomposeResponse> {
    return this.fetch(`/api/issues/${issueId}/decompose/${runId}`);
  }

  async confirmDecompose(issueId: string, data: ConfirmDecomposeRequest): Promise<ConfirmDecomposeResponse> {
    return this.fetch(`/api/issues/${issueId}/decompose/confirm`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  // Issue Dependencies
  async listIssueDependencies(issueId: string): Promise<IssueDependency[]> {
    return this.fetch(`/api/issues/${issueId}/dependencies`);
  }

  // Sub-Issues
  async listSubIssues(issueId: string): Promise<Issue[]> {
    return this.fetch(`/api/issues/${issueId}/sub-issues`);
  }

  // Task Chaining
  async chainTask(taskId: string, data: ChainTaskRequest): Promise<AgentTask> {
    return this.fetch(`/api/tasks/${taskId}/chain`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  // Task Review
  async submitReview(taskId: string, data: SubmitReviewRequest): Promise<AgentTask> {
    return this.fetch(`/api/tasks/${taskId}/review`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  // Task Retry
  async retryTask(issueId: string, taskId: string): Promise<AgentTask> {
    return this.fetch(`/api/issues/${issueId}/tasks/${taskId}/retry`, {
      method: "POST",
    });
  }
}
