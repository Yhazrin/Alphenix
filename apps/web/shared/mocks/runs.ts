import type { Run, RunStep, RunTodo, RunArtifact } from "@/shared/types";

// Helper to generate ISO timestamps
function now() {
  return new Date().toISOString();
}

function minutesAgo(minutes: number): string {
  const d = new Date();
  d.setMinutes(d.getMinutes() - minutes);
  return d.toISOString();
}

// Mock Run Data Generator
export function createMockRun(overrides: Partial<Run> = {}): Run {
  return {
    id: "run_123",
    workspace_id: "ws_123",
    issue_id: "issue_456",
    task_id: null,
    agent_id: "agent_789",
    parent_run_id: null,
    team_id: null,
    phase: "executing",
    status: "running",
    system_prompt: "You are a helpful assistant.",
    model_name: "gpt-4",
    permission_mode: "auto",
    input_tokens: 150,
    output_tokens: 300,
    estimated_cost_usd: 0.045,
    started_at: minutesAgo(10),
    completed_at: null,
    created_at: minutesAgo(10),
    updated_at: now(),
    error_category: undefined,
    error_severity: undefined,
    ...overrides,
  };
}

// Mock RunStep Data Generator
export function createMockRunStep(overrides: Partial<RunStep> = {}): RunStep {
  return {
    id: `step_${Math.random().toString(36).substr(2, 9)}`,
    run_id: "run_123",
    seq: 1,
    step_type: "thinking",
    tool_name: "",
    call_id: null,
    tool_input: {},
    tool_output: null,
    is_error: false,
    started_at: minutesAgo(9),
    completed_at: minutesAgo(8),
    error_category: undefined,
    error_subcategory: undefined,
    error_severity: undefined,
    exclusion_reason: undefined,
    ...overrides,
  };
}

// Mock RunTodo Data Generator
export function createMockRunTodo(overrides: Partial<RunTodo> = {}): RunTodo {
  return {
    id: `todo_${Math.random().toString(36).substr(2, 9)}`,
    run_id: "run_123",
    seq: 1,
    title: "Analyze the issue",
    description: "Understand the user's request",
    status: "completed",
    blocker: null,
    created_at: minutesAgo(9),
    updated_at: minutesAgo(8),
    ...overrides,
  };
}

// Mock RunArtifact Data Generator
export function createMockRunArtifact(overrides: Partial<RunArtifact> = {}): RunArtifact {
  return {
    id: `artifact_${Math.random().toString(36).substr(2, 9)}`,
    run_id: "run_123",
    step_id: null,
    artifact_type: "text",
    name: "analysis.txt",
    content: "Analysis content...",
    mime_type: "text/plain",
    created_at: minutesAgo(5),
    ...overrides,
  };
}

// Complex Mock Data for Run Detail View
export function getMockRunDetailData() {
  const run = createMockRun({
    error_category: "TOOL_ERROR",
    error_severity: "TRANSIENT",
  });

  const steps: RunStep[] = [
    createMockRunStep({
      seq: 1,
      step_type: "thinking",
      tool_output: "I need to read the file to understand the current implementation.",
      started_at: minutesAgo(9),
      completed_at: minutesAgo(8.5),
    }),
    createMockRunStep({
      seq: 2,
      step_type: "tool_use",
      tool_name: "Read",
      call_id: "call_1",
      tool_input: { path: "/d/project/multicode/multica/apps/web/features/runs/components/step-card.tsx" },
      started_at: minutesAgo(8.5),
      completed_at: minutesAgo(8),
    }),
    createMockRunStep({
      seq: 3,
      step_type: "tool_result",
      tool_name: "Read",
      call_id: "call_1",
      tool_output: "File content...",
      started_at: minutesAgo(8),
      completed_at: minutesAgo(7.5),
    }),
    createMockRunStep({
      seq: 4,
      step_type: "thinking",
      tool_output: "Now I understand the structure. I will update the component.",
      started_at: minutesAgo(7.5),
      completed_at: minutesAgo(7),
    }),
    createMockRunStep({
      seq: 5,
      step_type: "tool_use",
      tool_name: "Edit",
      call_id: "call_2",
      tool_input: { path: "/d/project/multicode/multica/apps/web/features/runs/components/step-card.tsx", action: "update" },
      started_at: minutesAgo(7),
      completed_at: minutesAgo(6),
    }),
    createMockRunStep({
      seq: 6,
      step_type: "tool_result",
      tool_name: "Edit",
      call_id: "call_2",
      tool_output: "File updated successfully.",
      started_at: minutesAgo(6),
      completed_at: minutesAgo(5.5),
    }),
  ];

  const todos: RunTodo[] = [
    createMockRunTodo({
      seq: 1,
      title: "Analyze the issue",
      status: "completed",
      created_at: minutesAgo(9),
      updated_at: minutesAgo(8),
    }),
    createMockRunTodo({
      seq: 2,
      title: "Read the file",
      status: "completed",
      created_at: minutesAgo(8.5),
      updated_at: minutesAgo(7.5),
    }),
    createMockRunTodo({
      seq: 3,
      title: "Update the component",
      status: "in_progress",
      created_at: minutesAgo(7),
      updated_at: minutesAgo(6),
    }),
    createMockRunTodo({
      seq: 4,
      title: "Verify the changes",
      status: "pending",
      created_at: minutesAgo(5),
      updated_at: minutesAgo(5),
    }),
  ];

  const artifacts: RunArtifact[] = [
    createMockRunArtifact({
      step_id: steps[2]?.id ?? null,
      artifact_type: "file_content",
      name: "step-card.tsx",
      content: "import { useState } from 'react';...",
      mime_type: "text/typescript",
    }),
  ];

  return { run, steps, todos, artifacts };
}

// Legacy constants for backward compatibility
export const mockRun: Run = {
  id: "run_abc123",
  workspace_id: "ws_123",
  issue_id: "issue_456",
  task_id: null,
  agent_id: "agent_789",
  parent_run_id: null,
  team_id: null,
  phase: "executing",
  status: "running",
  system_prompt: "You are a helpful assistant.",
  model_name: "gpt-4",
  permission_mode: "standard",
  input_tokens: 1500,
  output_tokens: 800,
  estimated_cost_usd: 0.023,
  started_at: "2026-04-06T15:00:00Z",
  completed_at: null,
  created_at: "2026-04-06T14:59:55Z",
  updated_at: "2026-04-06T15:00:05Z",
  error_category: undefined,
  error_severity: undefined,
};

export const mockRunSteps: RunStep[] = [
  {
    id: "step_1",
    run_id: "run_abc123",
    seq: 1,
    step_type: "thinking",
    tool_name: "",
    call_id: null,
    tool_input: {},
    tool_output: null,
    is_error: false,
    started_at: "2026-04-06T15:00:01Z",
    completed_at: "2026-04-06T15:00:02Z",
    summary: "Thinking about the next step...",
    error_category: undefined,
    error_subcategory: undefined,
    error_severity: undefined,
    exclusion_reason: undefined,
  },
  {
    id: "step_2",
    run_id: "run_abc123",
    seq: 2,
    step_type: "tool_use",
    tool_name: "read_file",
    call_id: "call_123",
    tool_input: { path: "/app/main.tsx" },
    tool_output: null,
    is_error: false,
    started_at: "2026-04-06T15:00:03Z",
    completed_at: null,
    error_category: undefined,
    error_subcategory: undefined,
    error_severity: undefined,
    exclusion_reason: undefined,
  },
  {
    id: "step_3",
    run_id: "run_abc123",
    seq: 3,
    step_type: "tool_result",
    tool_name: "read_file",
    call_id: "call_123",
    tool_input: {},
    tool_output: "File content...",
    is_error: false,
    started_at: "2026-04-06T15:00:05Z",
    completed_at: "2026-04-06T15:00:06Z",
    error_category: undefined,
    error_subcategory: undefined,
    error_severity: undefined,
    exclusion_reason: undefined,
  },
  {
    id: "step_4",
    run_id: "run_abc123",
    seq: 4,
    step_type: "text",
    tool_name: "",
    call_id: null,
    tool_input: {},
    tool_output: "I have read the file content.",
    is_error: false,
    started_at: "2026-04-06T15:00:07Z",
    completed_at: "2026-04-06T15:00:08Z",
    error_category: undefined,
    error_subcategory: undefined,
    error_severity: undefined,
    exclusion_reason: undefined,
  },
];

export const mockRunTodos: RunTodo[] = [
  {
    id: "todo_1",
    run_id: "run_abc123",
    seq: 1,
    title: "Analyze code structure",
    content: "Review the main application architecture",
    description: "Review the main application architecture",
    status: "completed",
    blocker: null,
    created_at: "2026-04-06T15:00:00Z",
    updated_at: "2026-04-06T15:05:00Z",
  },
  {
    id: "todo_2",
    run_id: "run_abc123",
    seq: 2,
    title: "Refactor component",
    content: "Optimize StepRenderer performance",
    description: "Optimize StepRenderer performance",
    status: "in_progress",
    blocker: null,
    created_at: "2026-04-06T15:05:00Z",
    updated_at: "2026-04-06T15:10:00Z",
  },
];

export const mockRunArtifacts: RunArtifact[] = [
  {
    id: "artifact_1",
    run_id: "run_abc123",
    step_id: "step_3",
    artifact_type: "file_diff",
    name: "main.tsx.diff",
    content: "--- a/main.tsx\n+++ b/main.tsx\n@@ -1,3 +1,4 @@",
    mime_type: "text/x-diff",
    created_at: "2026-04-06T15:10:00Z",
  },
];
