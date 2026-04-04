export type AgentStatus = "idle" | "working" | "blocked" | "error" | "offline";

export type AgentRuntimeMode = "local" | "cloud";

export type AgentVisibility = "workspace" | "private";

export type AgentTriggerType = "on_assign" | "on_comment" | "scheduled";

export type ApprovalStatus = "pending" | "approved" | "rejected" | "revoked";
export type TrustLevel = "self" | "trusted_member" | "restricted";

export interface RuntimeDevice {
  id: string;
  workspace_id: string;
  daemon_id: string | null;
  instance_id: string;
  name: string;
  runtime_mode: AgentRuntimeMode;
  provider: string;
  status: "online" | "offline";
  device_info: string;
  metadata: Record<string, unknown>;
  last_seen_at: string | null;
  created_at: string;
  updated_at: string;
  owner_user_id: string | null;
  approval_status: ApprovalStatus;
  visibility: "private" | "workspace" | "team";
  trust_level: TrustLevel;
  drain_mode: boolean;
  paused: boolean;
  tags: string[];
  max_concurrent_tasks_override: number | null;
  last_claimed_at: string | null;
  success_count_24h: number;
  failure_count_24h: number;
  avg_task_duration_ms: number;
}

export type AgentRuntime = RuntimeDevice;

export interface AgentTool {
  id: string;
  name: string;
  description: string;
  auth_type: "oauth" | "api_key" | "none";
  connected: boolean;
  config: Record<string, unknown>;
}

export interface AgentTrigger {
  id: string;
  type: AgentTriggerType;
  enabled: boolean;
  config: Record<string, unknown>;
}

export interface AgentTask {
  id: string;
  agent_id: string;
  runtime_id: string;
  issue_id: string;
  status: "queued" | "dispatched" | "running" | "completed" | "failed" | "cancelled";
  priority: number;
  dispatched_at: string | null;
  started_at: string | null;
  completed_at: string | null;
  result: unknown;
  error: string | null;
  created_at: string;
}

export interface Agent {
  id: string;
  workspace_id: string;
  runtime_id: string;
  name: string;
  description: string;
  instructions: string;
  avatar_url: string | null;
  runtime_mode: AgentRuntimeMode;
  runtime_config: Record<string, unknown>;
  visibility: AgentVisibility;
  status: AgentStatus;
  max_concurrent_tasks: number;
  owner_id: string | null;
  skills: Skill[];
  tools: AgentTool[];
  triggers: AgentTrigger[];
  created_at: string;
  updated_at: string;
  archived_at: string | null;
  archived_by: string | null;
}

export interface CreateAgentRequest {
  name: string;
  description?: string;
  instructions?: string;
  avatar_url?: string;
  runtime_id: string;
  runtime_config?: Record<string, unknown>;
  visibility?: AgentVisibility;
  max_concurrent_tasks?: number;
  tools?: AgentTool[];
  triggers?: AgentTrigger[];
}

export interface UpdateAgentRequest {
  name?: string;
  description?: string;
  instructions?: string;
  avatar_url?: string;
  runtime_id?: string;
  runtime_config?: Record<string, unknown>;
  visibility?: AgentVisibility;
  status?: AgentStatus;
  max_concurrent_tasks?: number;
  tools?: AgentTool[];
  triggers?: AgentTrigger[];
}

// Skills

export interface Skill {
  id: string;
  workspace_id: string;
  name: string;
  description: string;
  content: string;
  config: Record<string, unknown>;
  files: SkillFile[];
  created_by: string | null;
  created_at: string;
  updated_at: string;
}

export interface SkillFile {
  id: string;
  skill_id: string;
  path: string;
  content: string;
  created_at: string;
  updated_at: string;
}

export interface CreateSkillRequest {
  name: string;
  description?: string;
  content?: string;
  config?: Record<string, unknown>;
  files?: { path: string; content: string }[];
}

export interface UpdateSkillRequest {
  name?: string;
  description?: string;
  content?: string;
  config?: Record<string, unknown>;
  files?: { path: string; content: string }[];
}

export interface SetAgentSkillsRequest {
  skill_ids: string[];
}

export type RuntimePingStatus = "pending" | "running" | "completed" | "failed" | "timeout";

export interface RuntimePing {
  id: string;
  runtime_id: string;
  status: RuntimePingStatus;
  output?: string;
  error?: string;
  duration_ms?: number;
  created_at: string;
  updated_at: string;
}

export interface RuntimeUsage {
  runtime_id: string;
  date: string;
  provider: string;
  model: string;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
}

export interface RuntimeHourlyActivity {
  hour: number;
  count: number;
}

export type RuntimeUpdateStatus =
  | "pending"
  | "running"
  | "completed"
  | "failed"
  | "timeout";

export interface RuntimeUpdate {
  id: string;
  runtime_id: string;
  status: RuntimeUpdateStatus;
  target_version: string;
  output?: string;
  error?: string;
  created_at: string;
  updated_at: string;
}

export interface RuntimeJoinToken {
  id: string;
  workspace_id: string;
  created_by: string;
  token_prefix: string;
  expires_at: string;
  used_at: string | null;
  created_at: string;
  metadata: Record<string, unknown>;
}

export interface RuntimeAuditLog {
  id: string;
  workspace_id: string;
  runtime_id: string;
  actor_user_id: string | null;
  action: string;
  details: Record<string, unknown>;
  created_at: string;
}

export interface CreateRuntimeJoinTokenRequest {
  expires_in_minutes?: number;
}

export interface CreateRuntimeJoinTokenResponse {
  token: string;
  token_prefix: string;
  expires_at: string;
}

export interface RegisterRuntimeWithJoinTokenRequest {
  join_token: string;
  daemon_id: string;
  instance_id: string;
  name: string;
  provider: string;
  runtime_mode: "local_daemon";
  device_info: string;
  metadata: Record<string, unknown>;
}

export interface RegisterRuntimeWithJoinTokenResponse {
  runtime_id: string;
  approval_status: ApprovalStatus;
  status: string;
}

export interface TaskReport {
  id: string;
  agent_id: string;
  agent_name: string;
  issue_id: string;
  issue_title: string;
  runtime_id: string;
  runtime_name: string | null;
  status: string;
  priority: number;
  dispatched_at: string | null;
  started_at: string | null;
  completed_at: string | null;
  result: unknown;
  error: string | null;
  created_at: string;
  review_status: string;
  message_count: number;
  checkpoint_count: number;
}

export interface TaskTimelineEvent {
  event_type: string;
  id: string;
  timestamp: string;
  title: string;
  detail: string;
  meta: unknown;
}

export interface RuntimePolicy {
  id: string;
  workspace_id: string;
  agent_id: string;
  team_id?: string;
  required_tags: string[];
  forbidden_tags: string[];
  preferred_runtime_ids: string[];
  fallback_runtime_ids: string[];
  max_queue_depth: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateRuntimePolicyRequest {
  agent_id: string;
  team_id?: string;
  required_tags?: string[];
  forbidden_tags?: string[];
  preferred_runtime_ids?: string[];
  fallback_runtime_ids?: string[];
  max_queue_depth?: number;
  is_active?: boolean;
}

export interface UpdateRuntimePolicyRequest {
  required_tags?: string[];
  forbidden_tags?: string[];
  preferred_runtime_ids?: string[];
  fallback_runtime_ids?: string[];
  max_queue_depth?: number;
  is_active?: boolean;
}
