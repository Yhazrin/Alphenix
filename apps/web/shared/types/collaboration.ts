// Collaboration types matching Go backend handler responses.

export interface AgentMessage {
  id: string;
  workspace_id: string;
  from_agent_id: string;
  to_agent_id: string;
  task_id?: string;
  content: string;
  message_type: string;
  reply_to_id?: string;
  read_at: string | null;
  created_at: string;
}

export interface SendMessageRequest {
  to_agent_id: string;
  content: string;
  message_type?: string;
  task_id?: string;
  reply_to_id?: string;
}

export interface TaskDependency {
  task_id: string;
  depends_on_id: string;
  created_at: string;
}

export interface AddDependencyRequest {
  depends_on_task_id: string;
}

export interface TaskCheckpoint {
  id: string;
  task_id: string;
  workspace_id: string;
  label: string;
  state?: Record<string, unknown>;
  files_changed?: string[];
  created_at: string;
}

export interface SaveCheckpointRequest {
  label: string;
  state?: Record<string, unknown>;
  files_changed?: string[];
}

export interface AgentMemory {
  id: string;
  agent_id: string;
  content: string;
  metadata?: Record<string, unknown>;
  similarity?: number;
  created_at: string;
  expires_at?: string | null;
}

export interface StoreMemoryRequest {
  content: string;
  embedding?: number[];
  metadata?: Record<string, unknown>;
  expires_at?: string;
}

export interface RecallMemoryRequest {
  embedding: number[];
  limit?: number;
}

export interface ChainTaskRequest {
  target_agent_id: string;
  chain_reason?: string;
}

export interface SubmitReviewRequest {
  verdict: "pass" | "fail" | "retry";
  feedback?: string;
}

export interface RecallWorkspaceMemoryRequest {
  query: string;
  limit?: number;
  agent_id?: string;
}

export interface SharedContext {
  task_id: string;
  workspace_id: string;
  context: Record<string, unknown>;
  updated_at: string;
}

export interface TaskDependencyInfo {
  task_id: string;
  depends_on_id: string;
  status: string;
  created_at: string;
}
