"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { runsApi } from "@/shared/api";
import { useWSEvent } from "@/features/realtime";
import { toast } from "sonner";
import type { Run, RunStep, RunTodo } from "@/shared/types";
import type {
  RunCreatedPayload,
  RunStartedPayload,
  RunPhaseChangedPayload,
  RunCompletedPayload,
  RunFailedPayload,
  RunCancelledPayload,
  RunStepStartedPayload,
  RunStepCompletedPayload,
  RunTodoCreatedPayload,
  RunTodoUpdatedPayload,
} from "@/shared/types/events";

// ─── Runtime type guards ────────────────────────────────────────────────────

function isRunEventPayload(val: unknown): val is { run_id: string; issue_id?: string } {
  if (typeof val !== "object" || val === null) return false;
  return "run_id" in val;
}

function isRunStepPayload(val: unknown): val is RunStepStartedPayload {
  if (typeof val !== "object" || val === null) return false;
  const v = val as Record<string, unknown>;
  return typeof v.run_id === "string" && typeof v.step_id === "string";
}

function isRunTodoPayload(val: unknown): val is RunTodoCreatedPayload {
  if (typeof val !== "object" || val === null) return false;
  const v = val as Record<string, unknown>;
  return typeof v.run_id === "string" && typeof v.todo_id === "string";
}

// ─── Hook ───────────────────────────────────────────────────────────────────

export interface RunTimelineData {
  runs: Run[];
  stepsByRun: Map<string, RunStep[]>;
  todosByRun: Map<string, RunTodo[]>;
  loading: boolean;
  error: string | null;
}

export function useRunTimeline(issueId: string): RunTimelineData {
  const [runs, setRuns] = useState<Run[]>([]);
  const [stepsByRun, setStepsByRun] = useState<Map<string, RunStep[]>>(new Map());
  const [todosByRun, setTodosByRun] = useState<Map<string, RunTodo[]>>(new Map());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const loadedRuns = useRef(new Set<string>());

  // Initial load: fetch all runs for this issue, then load steps/todos for each
  useEffect(() => {
    let cancelled = false;
    setError(null);

    async function load() {
      try {
        const issueRuns = await runsApi.listRunsByIssue(issueId);
        if (cancelled) return;

        setRuns(issueRuns);

        // Load steps and todos for each run in parallel
        const stepEntries = await Promise.all(
          issueRuns.map(async (run) => {
            const steps = await runsApi.getRunSteps(run.id);
            return [run.id, steps] as const;
          }),
        );
        const todoEntries = await Promise.all(
          issueRuns.map(async (run) => {
            const todos = await runsApi.getRunTodos(run.id);
            return [run.id, todos] as const;
          }),
        );

        if (cancelled) return;

        setStepsByRun(new Map(stepEntries));
        setTodosByRun(new Map(todoEntries));
        for (const run of issueRuns) loadedRuns.current.add(run.id);
      } catch (e: unknown) {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : "Failed to load run timeline");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    load();
    return () => { cancelled = true; };
  }, [issueId]);

  // --- WS event handlers ---

  // New run created
  useWSEvent("run:created", useCallback((payload: unknown) => {
    if (!isRunEventPayload(payload)) return;
    if (payload.issue_id !== issueId) return;
    // Refetch the run to get full data
    runsApi.getRun(payload.run_id).then((run) => {
      setRuns((prev) => {
        if (prev.some((r) => r.id === run.id)) return prev;
        return [...prev, run];
      });
      loadedRuns.current.add(run.id);
    }).catch((e) => {
      console.error(e);
      toast.error("Failed to load new run data");
    });
  }, [issueId]));

  // Run started — update phase/status
  useWSEvent("run:started", useCallback((payload: unknown) => {
    const p = payload as RunStartedPayload;
    if (!p?.run_id) return;
    setRuns((prev) =>
      prev.map((r) => (r.id === p.run_id ? { ...r, phase: "executing" as const, status: "active" } : r)),
    );
  }, []));

  // Phase changed
  useWSEvent("run:phase_changed", useCallback((payload: unknown) => {
    const p = payload as RunPhaseChangedPayload;
    if (!p?.run_id || !p.phase) return;
    setRuns((prev) =>
      prev.map((r) => (r.id === p.run_id ? { ...r, phase: p.phase as Run["phase"] } : r)),
    );
  }, []));

  // Run completed/failed/cancelled
  const updateRunTerminal = useCallback((runId: string, phase: Run["phase"]) => {
    setRuns((prev) =>
      prev.map((r) => (r.id === runId ? { ...r, phase } : r)),
    );
  }, []);

  useWSEvent("run:completed", useCallback((payload: unknown) => {
    const p = payload as RunCompletedPayload;
    if (p?.run_id) updateRunTerminal(p.run_id, "completed");
  }, [updateRunTerminal]));

  useWSEvent("run:failed", useCallback((payload: unknown) => {
    const p = payload as RunFailedPayload;
    if (p?.run_id) updateRunTerminal(p.run_id, "failed");
  }, [updateRunTerminal]));

  useWSEvent("run:cancelled", useCallback((payload: unknown) => {
    const p = payload as RunCancelledPayload;
    if (p?.run_id) updateRunTerminal(p.run_id, "cancelled");
  }, [updateRunTerminal]));

  // Step started — fetch step data
  useWSEvent("run:step_started", useCallback((payload: unknown) => {
    if (!isRunStepPayload(payload)) return;
    const p = payload as RunStepStartedPayload;
    // Add a placeholder step; the real data will come from step_completed
    setStepsByRun((prev) => {
      const existing = prev.get(p.run_id) ?? [];
      if (existing.some((s) => s.id === p.step_id)) return prev;
      const placeholder: RunStep = {
        id: p.step_id,
        run_id: p.run_id,
        seq: p.seq,
        step_type: (p.step_type as RunStep["step_type"]) ?? "tool_use",
        tool_name: p.tool_name,
        call_id: p.call_id ?? null,
        tool_input: {},
        tool_output: null,
        is_error: p.is_error,
        started_at: new Date().toISOString(),
        completed_at: null,
      };
      const next = new Map(prev);
      next.set(p.run_id, [...existing, placeholder]);
      return next;
    });
  }, []));

  // Step completed — update existing step
  useWSEvent("run:step_completed", useCallback((payload: unknown) => {
    if (!isRunStepPayload(payload)) return;
    const p = payload as RunStepCompletedPayload;
    setStepsByRun((prev) => {
      const existing = prev.get(p.run_id);
      if (!existing) return prev;
      const next = new Map(prev);
      next.set(
        p.run_id,
        existing.map((s) =>
          s.id === p.step_id
            ? { ...s, is_error: p.is_error, completed_at: new Date().toISOString() }
            : s,
        ),
      );
      return next;
    });
  }, []));

  // Todo created
  useWSEvent("run:todo_created", useCallback((payload: unknown) => {
    if (!isRunTodoPayload(payload)) return;
    const p = payload as RunTodoCreatedPayload;
    setTodosByRun((prev) => {
      const existing = prev.get(p.run_id) ?? [];
      if (existing.some((t) => t.id === p.todo_id)) return prev;
      const placeholder: RunTodo = {
        id: p.todo_id,
        run_id: p.run_id,
        seq: p.seq,
        title: p.title,
        description: "",
        status: "pending",
        blocker: null,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      };
      const next = new Map(prev);
      next.set(p.run_id, [...existing, placeholder]);
      return next;
    });
  }, []));

  // Todo updated
  useWSEvent("run:todo_updated", useCallback((payload: unknown) => {
    const p = payload as RunTodoUpdatedPayload;
    if (!p?.run_id || !p.todo_id) return;
    setTodosByRun((prev) => {
      const existing = prev.get(p.run_id);
      if (!existing) return prev;
      const next = new Map(prev);
      next.set(
        p.run_id,
        existing.map((t) =>
          t.id === p.todo_id
            ? { ...t, status: p.status as RunTodo["status"], blocker: p.blocker ?? null }
            : t,
        ),
      );
      return next;
    });
  }, []));

  return { runs, stepsByRun, todosByRun, loading, error };
}
