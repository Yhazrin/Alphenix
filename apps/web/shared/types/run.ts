/** Run phases — mirrors server/service/run_orchestrator.go */
export type RunPhase =
  | "pending"
  | "planning"
  | "executing"
  | "reviewing"
  | "completed"
  | "failed"
  | "cancelled";

/** A single agent run execution. */
export interface Run {
  id: string;
  workspace_id: string;
  issue_id: string;
  task_id: string | null;
  agent_id: string;
  parent_run_id: string | null;
  team_id: string | null;
  phase: RunPhase;
  status: string;
  system_prompt: string;
  model_name: string;
  permission_mode: string;
  input_tokens: number;
  output_tokens: number;
  estimated_cost_usd: number;
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
  updated_at: string;
  error_category?: string;
  error_severity?: string;
}

/** Step type — mirrors server schema v1.0 step_type enum. */
export type StepType = "thinking" | "text" | "tool_use" | "tool_result" | "error";

/** A single execution step within a run. */
export interface RunStep {
  id: string;
  run_id: string;
  seq: number;
  step_type: StepType;
  tool_name: string;
  call_id: string | null;
  tool_input: Record<string, unknown>;
  tool_output: string | null;
  is_error: boolean;
  started_at: string;
  completed_at: string | null;
  summary?: string;
  error_category?: string;
  error_subcategory?: string;
  error_severity?: string;
  exclusion_reason?: string;
}

/** A todo item tracked by the agent during a run. */
export interface RunTodo {
  id: string;
  run_id: string;
  seq: number;
  title: string;
  content?: string;
  description: string;
  status: "pending" | "in_progress" | "completed" | "blocked";
  blocker: string | null;
  created_at: string;
  updated_at: string;
}

/** An artifact (file output, report, etc.) produced during a run. */
export interface RunArtifact {
  id: string;
  run_id: string;
  step_id: string | null;
  artifact_type: string;
  name: string;
  content: string;
  mime_type: string;
  created_at: string;
}
