export type IssueStatus =
  | "backlog"
  | "todo"
  | "in_progress"
  | "in_review"
  | "done"
  | "blocked"
  | "cancelled";

export type IssuePriority = "urgent" | "high" | "medium" | "low" | "none";

export type IssueAssigneeType = "member" | "agent" | "team";

export interface IssueReaction {
  id: string;
  issue_id: string;
  actor_type: string;
  actor_id: string;
  emoji: string;
  created_at: string;
}

export interface IssueDependency {
  id: string;
  issue_id: string;
  depends_on_issue_id: string;
  type: "blocks" | "blocked_by" | "related";
  created_at: string;
}

export interface SubtaskPreview {
  title: string;
  description: string;
  deliverable: string;
  depends_on: number[];
  assignee_type?: string | null;
  assignee_id?: string | null;
}

export interface DecomposePreview {
  subtasks: SubtaskPreview[];
  plan_summary: string;
  risks: string[];
}

export interface DecomposeResponse {
  run_id: string;
  status: string;
  preview?: DecomposePreview | null;
  error?: string;
}

export interface ConfirmDecomposeRequest {
  subtasks: SubtaskPreview[];
}

export interface ConfirmDecomposeResponse {
  issues: Issue[];
  total: number;
}

export interface Issue {
  id: string;
  workspace_id: string;
  /** Workspace channel (project) this issue belongs to */
  channel_id: string;
  number: number;
  identifier: string;
  title: string;
  description: string | null;
  status: IssueStatus;
  priority: IssuePriority;
  assignee_type: IssueAssigneeType | null;
  assignee_id: string | null;
  creator_type: IssueAssigneeType;
  creator_id: string;
  parent_issue_id: string | null;
  position: number;
  due_date: string | null;
  issue_kind?: "goal" | "task";
  repo_id: string | null;
  reactions?: IssueReaction[];
  latest_task_status?: string | null;
  created_at: string;
  updated_at: string;
}
