import type { Issue, IssueReaction } from "./issue";
import type { Agent } from "./agent";
import type { InboxItem } from "./inbox";
import type { Comment, Reaction } from "./comment";
import type { TimelineEntry } from "./activity";
import type { Workspace, MemberWithUser } from "./workspace";

// WebSocket event types (matching Go server protocol/events.go)
export type WSEventType =
  | "issue:created"
  | "issue:updated"
  | "issue:deleted"
  | "comment:created"
  | "comment:updated"
  | "comment:deleted"
  | "agent:status"
  | "agent:created"
  | "agent:archived"
  | "agent:restored"
  | "task:dispatch"
  | "task:progress"
  | "task:completed"
  | "task:failed"
  | "task:message"
  | "task:cancelled"
  | "task:chained"
  | "task:in_review"
  | "task:reviewed"
  | "inbox:new"
  | "inbox:read"
  | "inbox:archived"
  | "inbox:batch-read"
  | "inbox:batch-archived"
  | "workspace:updated"
  | "workspace:deleted"
  | "member:added"
  | "member:updated"
  | "member:removed"
  | "daemon:heartbeat"
  | "daemon:register"
  | "skill:created"
  | "skill:updated"
  | "skill:deleted"
  | "subscriber:added"
  | "subscriber:removed"
  | "activity:created"
  | "reaction:added"
  | "reaction:removed"
  | "issue_reaction:added"
  | "issue_reaction:removed"
  | "agent:message"
  | "agent:tool_use"
  | "agent:tool_result"
  | "agent:started"
  | "agent:completed"
  | "agent:failed"
  | "agent:stop"
  | "agent:session_start"
  | "task_dep:created"
  | "task_dep:deleted"
  | "task:checkpoint"
  | "memory:stored"
  | "memory:recalled";

export interface WSMessage<T = unknown> {
  type: WSEventType;
  payload: T;
  actor_id?: string;
}

export interface IssueCreatedPayload {
  issue: Issue;
}

export interface IssueUpdatedPayload {
  issue: Issue;
}

export interface IssueDeletedPayload {
  issue_id: string;
}

export interface AgentStatusPayload {
  agent: Agent;
}

export interface AgentCreatedPayload {
  agent: Agent;
}

export interface AgentArchivedPayload {
  agent: Agent;
}

export interface AgentRestoredPayload {
  agent: Agent;
}

export interface InboxNewPayload {
  item: InboxItem;
}

export interface InboxReadPayload {
  item_id: string;
  recipient_id: string;
}

export interface InboxArchivedPayload {
  item_id: string;
  recipient_id: string;
}

export interface InboxBatchReadPayload {
  recipient_id: string;
  count: number;
}

export interface InboxBatchArchivedPayload {
  recipient_id: string;
  count: number;
}

export interface CommentCreatedPayload {
  comment: Comment;
}

export interface CommentUpdatedPayload {
  comment: Comment;
}

export interface CommentDeletedPayload {
  comment_id: string;
  issue_id: string;
}

export interface WorkspaceUpdatedPayload {
  workspace: Workspace;
}

export interface WorkspaceDeletedPayload {
  workspace_id: string;
}

export interface MemberUpdatedPayload {
  member: MemberWithUser;
}

export interface MemberAddedPayload {
  member: MemberWithUser;
  workspace_id: string;
  workspace_name?: string;
}

export interface MemberRemovedPayload {
  member_id: string;
  user_id: string;
  workspace_id: string;
}

export interface SubscriberAddedPayload {
  issue_id: string;
  user_type: string;
  user_id: string;
  reason: string;
}

export interface SubscriberRemovedPayload {
  issue_id: string;
  user_type: string;
  user_id: string;
}

export interface ActivityCreatedPayload {
  issue_id: string;
  entry: TimelineEntry;
}

export interface TaskMessagePayload {
  task_id: string;
  issue_id: string;
  seq: number;
  type: "text" | "thinking" | "tool_use" | "tool_result" | "error";
  tool?: string;
  content?: string;
  input?: Record<string, unknown>;
  output?: string;
}

export interface TaskCompletedPayload {
  task_id: string;
  agent_id: string;
  issue_id: string;
  status: string;
}

export interface TaskFailedPayload {
  task_id: string;
  agent_id: string;
  issue_id: string;
  status: string;
}

export interface TaskCancelledPayload {
  task_id: string;
  agent_id: string;
  issue_id: string;
  status: string;
}

export interface ReactionAddedPayload {
  reaction: Reaction;
  issue_id: string;
}

export interface ReactionRemovedPayload {
  comment_id: string;
  issue_id: string;
  emoji: string;
  actor_type: string;
  actor_id: string;
}

export interface IssueReactionAddedPayload {
  reaction: IssueReaction;
  issue_id: string;
}

export interface IssueReactionRemovedPayload {
  issue_id: string;
  emoji: string;
  actor_type: string;
  actor_id: string;
}

// --- Collaboration event payloads ---

export interface AgentMessagePayload {
  message_id: string;
  from_agent_id: string;
  to_agent_id: string;
  task_id?: string;
  content: string;
  message_type: string;
  reply_to_id?: string;
}

export interface TaskDependencyPayload {
  task_id: string;
  depends_on_task_id: string;
}

export interface TaskCheckpointPayload {
  checkpoint_id: string;
  task_id: string;
  label: string;
}

export interface MemoryStoredPayload {
  memory_id: string;
  agent_id: string;
  workspace_id: string;
  content: string;
}

export interface MemoryRecalledPayload {
  workspace_id: string;
  query: string;
  count: number;
}

export interface TaskProgressPayload {
  task_id: string;
  summary: string;
  step: number;
  total: number;
}

export interface TaskChainedPayload {
  task_id: string;
  source_task_id: string;
  issue_id: string;
}

export interface TaskInReviewPayload {
  task_id: string;
  issue_id: string;
  review_id: string;
}

export interface TaskReviewedPayload {
  task_id: string;
  issue_id: string;
  review_id: string;
  approved: boolean;
  score: number;
}

// --- Agent lifecycle event payloads ---

export interface AgentToolUsePayload {
  agent_id: string;
  task_id: string;
  issue_id: string;
  tool: string;
  call_id: string;
  input?: Record<string, unknown>;
}

export interface AgentToolResultPayload {
  agent_id: string;
  task_id: string;
  issue_id: string;
  tool: string;
  call_id: string;
  output?: string;
  is_error?: boolean;
}

export interface AgentStartedPayload {
  agent_id: string;
  task_id: string;
  issue_id: string;
  session_id?: string;
}

export interface AgentCompletedPayload {
  agent_id: string;
  task_id: string;
  issue_id: string;
  session_id?: string;
  output?: string;
  duration_ms?: number;
}

export interface AgentFailedPayload {
  agent_id: string;
  task_id: string;
  issue_id: string;
  session_id?: string;
  error?: string;
}

export interface AgentStopPayload {
  agent_id: string;
  task_id: string;
  issue_id: string;
  reason?: string;
}

export interface AgentSessionStartPayload {
  agent_id: string;
  task_id: string;
  issue_id: string;
  session_id: string;
}
