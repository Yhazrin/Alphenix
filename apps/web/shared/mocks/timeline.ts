import type { Run, RunStep, StepType, RunTodo, RunArtifact } from "@/shared/types";

/** Mock Run with realistic agent execution data. */
export const mockRun: Run = {
  id: "run_mock_001",
  workspace_id: "ws_001",
  issue_id: "iss_042",
  task_id: "task_101",
  agent_id: "agent_claude",
  parent_run_id: null,
  team_id: null,
  phase: "executing",
  status: "running",
  system_prompt: "You are a senior software engineer...",
  model_name: "claude-sonnet-4-20250514",
  permission_mode: "auto",
  input_tokens: 12450,
  output_tokens: 8320,
  estimated_cost_usd: 0.0847,
  started_at: new Date(Date.now() - 120000).toISOString(),
  completed_at: null,
  created_at: new Date(Date.now() - 130000).toISOString(),
  updated_at: new Date().toISOString(),
};

export const mockRunCompleted: Run = {
  ...mockRun,
  id: "run_mock_002",
  phase: "completed",
  status: "completed",
  input_tokens: 24500,
  output_tokens: 18320,
  estimated_cost_usd: 0.1947,
  completed_at: new Date(Date.now() - 5000).toISOString(),
};

export const mockRunFailed: Run = {
  ...mockRun,
  id: "run_mock_003",
  phase: "failed",
  status: "failed",
  completed_at: new Date(Date.now() - 10000).toISOString(),
};

let stepSeq = 0;
function makeStep(
  stepType: StepType,
  toolName: string,
  input: Record<string, unknown>,
  output: string | null,
  opts: { error?: boolean; startedAgo?: number; durationMs?: number; callId?: string } = {},
): RunStep {
  const started = Date.now() - (opts.startedAgo ?? 0);
  const duration = opts.durationMs ?? 2000;
  return {
    id: `step_mock_${String(++stepSeq).padStart(3, "0")}`,
    run_id: mockRun.id,
    seq: stepSeq,
    step_type: stepType,
    tool_name: toolName,
    call_id: opts.callId ?? null,
    tool_input: input,
    tool_output: output,
    is_error: opts.error ?? false,
    started_at: new Date(started).toISOString(),
    completed_at: new Date(started + duration).toISOString(),
  };
}

/** Generate a deterministic call_id for tool_use/tool_result pairing. */
let callSeq = 0;
function nextCallId(): string {
  return `call_${String(++callSeq).padStart(3, "0")}`;
}

export const mockSteps: RunStep[] = [
  makeStep("thinking", "thinking", {}, "Analyzing the issue: user reports slow page load on /dashboard. Need to investigate bundle size, API response times, and render performance.", { startedAgo: 115000, durationMs: 3000 }),
  makeStep("tool_use", "read_file", { path: "apps/web/app/(dashboard)/page.tsx" }, null, { startedAgo: 110000, durationMs: 1500, callId: nextCallId() }),
  makeStep("tool_result", "read_file", { path: "apps/web/app/(dashboard)/page.tsx" }, "<file content truncated>", { startedAgo: 108500, durationMs: 500, callId: "call_001" }),
  makeStep("tool_use", "read_file", { path: "apps/web/app/(dashboard)/layout.tsx" }, null, { startedAgo: 108000, durationMs: 1200, callId: nextCallId() }),
  makeStep("tool_result", "read_file", { path: "apps/web/app/(dashboard)/layout.tsx" }, "<file content truncated>", { startedAgo: 106800, durationMs: 400, callId: "call_002" }),
  makeStep("thinking", "thinking", {}, "The dashboard layout imports 14 heavy components eagerly. The main bundle is 2.1MB. Need to add lazy loading and code splitting.", { startedAgo: 105000, durationMs: 2500 }),
  makeStep("tool_use", "bash", { command: "npx next-bundle-analyzer" }, null, { startedAgo: 100000, durationMs: 8000, callId: nextCallId() }),
  makeStep("tool_result", "bash", { command: "npx next-bundle-analyzer" }, "Bundle analysis complete. Largest chunks: vendor (1.2MB), dashboard (890KB), ui-components (340KB).", { startedAgo: 92000, durationMs: 500, callId: "call_003" }),
  makeStep("tool_use", "edit_file", { path: "apps/web/app/(dashboard)/page.tsx", action: "add dynamic imports" }, null, { startedAgo: 90000, durationMs: 5000, callId: nextCallId() }),
  makeStep("tool_result", "edit_file", { path: "apps/web/app/(dashboard)/page.tsx", action: "add dynamic imports" }, "Added React.lazy() wrapping for 6 dashboard sections. Estimated bundle reduction: 45%.", { startedAgo: 85000, durationMs: 500, callId: "call_004" }),
  makeStep("thinking", "thinking", {}, "Now checking API response times. The /api/workspace endpoint takes 2.3s — likely N+1 query.", { startedAgo: 82000, durationMs: 2000 }),
  makeStep("tool_use", "read_file", { path: "server/handlers/workspace.go" }, null, { startedAgo: 79000, durationMs: 1000, callId: nextCallId() }),
  makeStep("tool_result", "read_file", { path: "server/handlers/workspace.go" }, "<file content truncated>", { startedAgo: 78000, durationMs: 400, callId: "call_005" }),
  makeStep("tool_use", "bash", { command: "grep -n 'Query(' server/handlers/workspace.go" }, null, { startedAgo: 76000, durationMs: 3000, callId: nextCallId() }),
  makeStep("tool_result", "bash", { command: "grep -n 'Query(' server/handlers/workspace.go" }, "Found 8 sequential queries in GetWorkspace handler. Need to batch into single JOIN.", { startedAgo: 73000, durationMs: 500, callId: "call_006" }),
  makeStep("tool_use", "edit_file", { path: "server/handlers/workspace.go", action: "consolidate queries" }, null, { startedAgo: 70000, durationMs: 6000, callId: nextCallId() }),
  makeStep("tool_result", "edit_file", { path: "server/handlers/workspace.go", action: "consolidate queries" }, "Merged 8 queries into 2 with LEFT JOINs. Response time: 2.3s → 180ms.", { startedAgo: 64000, durationMs: 500, callId: "call_007" }),
  makeStep("thinking", "thinking", {}, "Bundle and API optimizations done. Now checking render performance — suspect unnecessary re-renders in sidebar.", { startedAgo: 62000, durationMs: 2000 }),
  makeStep("tool_use", "read_file", { path: "apps/web/app/(dashboard)/_components/app-sidebar.tsx" }, null, { startedAgo: 59000, durationMs: 1200, callId: nextCallId() }),
  makeStep("tool_result", "read_file", { path: "apps/web/app/(dashboard)/_components/app-sidebar.tsx" }, "<file content truncated>", { startedAgo: 57800, durationMs: 400, callId: "call_008" }),
  makeStep("thinking", "thinking", {}, "Sidebar re-renders on every state change because workspace selector subscribes to entire workspace store. Need granular selectors.", { startedAgo: 56000, durationMs: 2500 }),
  makeStep("tool_use", "edit_file", { path: "apps/web/app/(dashboard)/_components/app-sidebar.tsx", action: "add granular selectors" }, null, { startedAgo: 50000, durationMs: 4000, callId: nextCallId() }),
  makeStep("tool_result", "edit_file", { path: "apps/web/app/(dashboard)/_components/app-sidebar.tsx", action: "add granular selectors" }, "Split workspace store subscription into 3 granular selectors. Re-render count reduced from 12/s to 2/s.", { startedAgo: 46000, durationMs: 500, callId: "call_009" }),
  makeStep("tool_use", "bash", { command: "npm run build 2>&1 | tail -20" }, null, { startedAgo: 44000, durationMs: 15000, callId: nextCallId() }),
  makeStep("tool_result", "bash", { command: "npm run build 2>&1 | tail -20" }, "Build successful. New bundle size: 1.1MB (down from 2.1MB). All routes under 200KB each.", { startedAgo: 29000, durationMs: 500, callId: "call_010" }),
  makeStep("thinking", "thinking", {}, "All optimizations applied. Summary: 1) Lazy loading reduced bundle 45%, 2) Query consolidation 2.3s→180ms, 3) Granular selectors reduced re-renders 6x. Ready to report.", { startedAgo: 27000, durationMs: 3000 }),
];

export const mockTodos: RunTodo[] = [
  { id: "todo_001", run_id: mockRun.id, seq: 1, title: "Investigate bundle size", description: "Analyze and reduce JS bundle size on /dashboard", status: "completed", blocker: null, created_at: new Date(Date.now() - 115000).toISOString(), updated_at: new Date(Date.now() - 90000).toISOString() },
  { id: "todo_002", run_id: mockRun.id, seq: 2, title: "Fix N+1 query in workspace API", description: "Consolidate sequential queries into JOINs", status: "completed", blocker: null, created_at: new Date(Date.now() - 115000).toISOString(), updated_at: new Date(Date.now() - 70000).toISOString() },
  { id: "todo_003", run_id: mockRun.id, seq: 3, title: "Optimize sidebar re-renders", description: "Add granular store selectors to prevent unnecessary renders", status: "completed", blocker: null, created_at: new Date(Date.now() - 115000).toISOString(), updated_at: new Date(Date.now() - 50000).toISOString() },
  { id: "todo_004", run_id: mockRun.id, seq: 4, title: "Write performance regression tests", description: "Add E2E tests for page load time assertions", status: "in_progress", blocker: null, created_at: new Date(Date.now() - 115000).toISOString(), updated_at: new Date(Date.now() - 25000).toISOString() },
  { id: "todo_005", run_id: mockRun.id, seq: 5, title: "Update PR with changes", description: "Create PR with all performance fixes", status: "pending", blocker: "Waiting for tests to pass", created_at: new Date(Date.now() - 115000).toISOString(), updated_at: new Date(Date.now() - 115000).toISOString() },
];

export const mockArtifacts: RunArtifact[] = [
  { id: "art_001", run_id: mockRun.id, step_id: "step_mock_005", artifact_type: "report", name: "Bundle Analysis Report", content: "# Bundle Analysis\n\n## Before\n- Total: 2.1MB\n\n## After\n- Total: 1.1MB (-45%)", mime_type: "text/markdown", created_at: new Date(Date.now() - 90000).toISOString() },
  { id: "art_002", run_id: mockRun.id, step_id: "step_mock_010", artifact_type: "diff", name: "Query Optimization Diff", content: "- 8 sequential queries\n+ 2 JOIN queries\n\nResponse time: 2.3s → 180ms", mime_type: "text/plain", created_at: new Date(Date.now() - 70000).toISOString() },
];
