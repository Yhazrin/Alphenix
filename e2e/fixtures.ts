/**
 * TestApiClient — lightweight API helper for E2E test data setup/teardown.
 *
 * Uses raw fetch so E2E tests have zero build-time coupling to the web app.
 */

import pg from "pg";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? `http://localhost:${process.env.PORT ?? "8080"}`;
const DATABASE_URL = process.env.DATABASE_URL ?? "postgres://alphenix:alphenix@localhost:5432/alphenix?sslmode=disable";

interface TestWorkspace {
  id: string;
  name: string;
  slug: string;
}

export class TestApiClient {
  private token: string | null = null;
  private workspaceId: string | null = null;
  private createdIssueIds: string[] = [];
  private createdAgentIds: string[] = [];
  private createdRuntimeIds: string[] = [];
  private createdPolicyIds: string[] = [];
  private createdTaskIds: string[] = [];

  async login(email: string, name: string) {
    // Step 1: Send verification code (retry on 429 rate limit)
    // The server has a 10s rate limit per email; parallel test workers
    // sharing the same email can hit this, so retry after the window.
    let sendOk = false;
    for (let attempt = 0; attempt < 3 && !sendOk; attempt++) {
      const sendRes = await fetch(`${API_BASE}/auth/send-code`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      if (sendRes.ok) {
        sendOk = true;
      } else if (sendRes.status === 429) {
        console.warn(`[fixtures] send-code rate-limited for ${email}, attempt ${attempt + 1}/3`);
        await new Promise((r) => setTimeout(r, 11000));
      } else {
        const body = await sendRes.text();
        throw new Error(`send-code failed: ${sendRes.status} ${body}`);
      }
    }

    // Step 2: Verify using master code (888888) — avoids DB read timing issues
    const verifyRes = await fetch(`${API_BASE}/auth/verify-code`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, code: "888888" }),
    });
    const data = await verifyRes.json();
    this.token = data.token;

    // Update user name if needed
    if (name && data.user?.name !== name) {
      await this.authedFetch("/api/me", {
        method: "PATCH",
        body: JSON.stringify({ name }),
      });
    }

    return data;
  }

  async getWorkspaces(): Promise<TestWorkspace[]> {
    const res = await this.authedFetch("/api/workspaces");
    return res.json();
  }

  setWorkspaceId(id: string) {
    this.workspaceId = id;
  }

  async ensureWorkspace(name = "E2E Workspace", slug = "e2e-workspace") {
    const workspaces = await this.getWorkspaces();
    const workspace = workspaces.find((item) => item.slug === slug) ?? workspaces[0];
    if (workspace) {
      this.workspaceId = workspace.id;
      return workspace;
    }

    const res = await this.authedFetch("/api/workspaces", {
      method: "POST",
      body: JSON.stringify({ name, slug }),
    });
    if (res.ok) {
      const created = (await res.json()) as TestWorkspace;
      this.workspaceId = created.id;
      return created;
    }

    const refreshed = await this.getWorkspaces();
    const created = refreshed.find((item) => item.slug === slug) ?? refreshed[0];
    if (created) {
      this.workspaceId = created.id;
      return created;
    }

    throw new Error(`Failed to ensure workspace ${slug}: ${res.status} ${res.statusText}`);
  }

  async createIssue(title: string, opts?: Record<string, unknown>) {
    const res = await this.authedFetch("/api/issues", {
      method: "POST",
      body: JSON.stringify({ title, ...opts }),
    });
    const issue = await res.json();
    this.createdIssueIds.push(issue.id);
    return issue;
  }

  async deleteIssue(id: string) {
    await this.authedFetch(`/api/issues/${id}`, { method: "DELETE" });
  }

  /** Create a test agent via direct DB insert (requires runtime_id). */
  async createAgent(name: string, opts?: { instructions?: string; description?: string }) {
    const runtimeId = await this.ensureRuntime();
    const res = await this.authedFetch("/api/agents", {
      method: "POST",
      body: JSON.stringify({
        name,
        description: opts?.description ?? "",
        instructions: opts?.instructions ?? "You are a helpful test agent.",
        runtime_id: runtimeId,
        visibility: "workspace",
      }),
    });
    if (!res.ok) {
      throw new Error(`createAgent failed: ${res.status} ${await res.text()}`);
    }
    const agent = await res.json();
    this.createdAgentIds.push(agent.id);
    return agent;
  }

  /** Ensure a test runtime exists in the workspace, return its ID. */
  private async ensureRuntime(): Promise<string> {
    // Try to list existing runtimes first
    const listRes = await this.authedFetch("/api/runtimes");
    if (listRes.ok) {
      const runtimes = await listRes.json();
      if (Array.isArray(runtimes) && runtimes.length > 0) {
        return runtimes[0].id;
      }
    }
    // No runtime available — create one via direct DB insert
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      const wsId = this.workspaceId;
      const result = await client.query(
        `INSERT INTO agent_runtime (workspace_id, daemon_id, name, runtime_mode, provider, status, device_info, metadata, last_seen_at)
         VALUES ($1, NULL, 'E2E Test Runtime', 'cloud', 'e2e_test', 'online', '{}', '{}'::jsonb, now())
         RETURNING id`,
        [wsId]
      );
      const id = result.rows[0].id;
      this.createdRuntimeIds.push(id);
      return id;
    } finally {
      await client.end();
    }
  }

  /** Create a runtime policy for an agent. */
  async createRuntimePolicy(agentId: string, opts?: {
    required_tags?: string[];
    forbidden_tags?: string[];
    max_queue_depth?: number;
    is_active?: boolean;
  }) {
    const res = await this.authedFetch("/api/runtime-policies", {
      method: "POST",
      body: JSON.stringify({
        agent_id: agentId,
        required_tags: opts?.required_tags ?? [],
        forbidden_tags: opts?.forbidden_tags ?? [],
        preferred_runtime_ids: [],
        fallback_runtime_ids: [],
        max_queue_depth: opts?.max_queue_depth ?? 0,
        is_active: opts?.is_active ?? true,
      }),
    });
    if (!res.ok) {
      throw new Error(`createRuntimePolicy failed: ${res.status} ${await res.text()}`);
    }
    const policy = await res.json();
    this.createdPolicyIds.push(policy.id);
    return policy;
  }

  /** Create a task for an agent via direct DB insert. */
  async createTaskWithReport(agentId: string, issueId: string, opts?: {
    status?: string;
    result?: string;
    error?: string;
  }) {
    const runtimeId = await this.ensureRuntime();
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      const result = await client.query(
        `INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, result, error, created_at)
         VALUES ($1, $2, $3, $4, $5::jsonb, $6, now())
         RETURNING id`,
        [
          agentId,
          runtimeId,
          issueId,
          opts?.status ?? "completed",
          opts?.result ? JSON.stringify(opts.result) : null,
          opts?.error ?? null,
        ]
      );
      const id = result.rows[0].id;
      this.createdTaskIds.push(id);
      return { id, agent_id: agentId, issue_id: issueId, status: opts?.status ?? "completed" };
    } finally {
      await client.end();
    }
  }

  /** Clean up all issues created during this test. */
  async cleanup() {
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      // Delete tasks via DB (no API endpoint for task deletion)
      for (const id of this.createdTaskIds) {
        try { await client.query(`DELETE FROM agent_task_queue WHERE id = $1`, [id]); } catch { /* ignore */ }
      }
      // Delete policies via API
      for (const id of this.createdPolicyIds) {
        try { await this.authedFetch(`/api/runtime-policies/${id}`, { method: "DELETE" }); } catch { /* ignore */ }
      }
      // Delete agents via DB (cascading)
      for (const id of this.createdAgentIds) {
        try { await client.query(`DELETE FROM agent WHERE id = $1`, [id]); } catch { /* ignore */ }
      }
      // Delete test runtimes via DB
      for (const id of this.createdRuntimeIds) {
        try { await client.query(`DELETE FROM agent_runtime WHERE id = $1`, [id]); } catch { /* ignore */ }
      }
    } finally {
      await client.end();
    }
    // Delete issues via API
    for (const id of this.createdIssueIds) {
      try { await this.deleteIssue(id); } catch { /* ignore */ }
    }
    this.createdIssueIds = [];
    this.createdAgentIds = [];
    this.createdRuntimeIds = [];
    this.createdPolicyIds = [];
    this.createdTaskIds = [];
  }

  setToken(token: string) {
    this.token = token;
  }

  getToken() {
    return this.token;
  }

  private async authedFetch(path: string, init?: RequestInit) {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...((init?.headers as Record<string, string>) ?? {}),
    };
    if (this.token) headers["Authorization"] = `Bearer ${this.token}`;
    if (this.workspaceId) headers["X-Workspace-ID"] = this.workspaceId;
    return fetch(`${API_BASE}${path}`, { ...init, headers });
  }
}
