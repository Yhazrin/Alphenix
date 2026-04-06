"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import type { Run, RunStep, RunTodo } from "@/shared/types";
import { runsApi } from "@/shared/api/runs";

interface UseRunSubscriptionOptions {
  runId: string;
  onStep?: (step: RunStep) => void;
  onTodo?: (todo: RunTodo) => void;
  onRunUpdate?: (run: Run) => void;
}

/**
 * Subscribes to a run's real-time updates via WebSocket.
 * Falls back to polling when WS is unavailable (mock phase).
 *
 * When #22 WS events are ready, this hook will:
 * 1. Subscribe to `run:{id}:steps` channel
 * 2. Listen for `step:created`, `step:updated`, `todo:updated`, `run:phase_changed`
 * 3. Push updates via the callbacks
 *
 * For now, it fetches initial data via REST and exposes it.
 */
export function useRunSubscription({
  runId,
  onStep,
  onTodo,
  onRunUpdate,
}: UseRunSubscriptionOptions) {
  const [run, setRun] = useState<Run | null>(null);
  const [steps, setSteps] = useState<RunStep[]>([]);
  const [todos, setTodos] = useState<RunTodo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const onStepRef = useRef(onStep);
  const onTodoRef = useRef(onTodo);
  const onRunUpdateRef = useRef(onRunUpdate);
  onStepRef.current = onStep;
  onTodoRef.current = onTodo;
  onRunUpdateRef.current = onRunUpdate;

  // Initial data fetch
  useEffect(() => {
    let cancelled = false;

    async function fetchRunData() {
      setLoading(true);
      setError(null);
      try {
        const [runData, stepsData, todosData] = await Promise.all([
          runsApi.getRun(runId),
          runsApi.getRunSteps(runId),
          runsApi.getRunTodos(runId),
        ]);
        if (cancelled) return;
        setRun(runData);
        setSteps(stepsData);
        setTodos(todosData);
      } catch (e) {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : "Failed to load run");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    fetchRunData();
    return () => { cancelled = true; };
  }, [runId]);

  // TODO: WebSocket subscription — enabled when #22 WS events are ready
  // useEffect(() => {
  //   const ws = getWSClient();
  //   const unsubStep = ws.on("step:created", (payload) => {
  //     const step = payload as RunStep;
  //     setSteps((prev) => [...prev, step]);
  //     onStepRef.current?.(step);
  //   });
  //   const unsubTodo = ws.on("todo:updated", (payload) => {
  //     const todo = payload as RunTodo;
  //     setTodos((prev) => prev.map((t) => t.id === todo.id ? todo : t));
  //     onTodoRef.current?.(todo);
  //   });
  //   const unsubRun = ws.on("run:phase_changed", (payload) => {
  //     const updatedRun = payload as Run;
  //     setRun(updatedRun);
  //     onRunUpdateRef.current?.(updatedRun);
  //   });
  //   ws.send({ type: "subscribe", payload: { channel: `run:${runId}:steps` } });
  //   return () => {
  //     unsubStep();
  //     unsubTodo();
  //     unsubRun();
  //     ws.send({ type: "unsubscribe", payload: { channel: `run:${runId}:steps` } });
  //   };
  // }, [runId]);

  const addStep = useCallback((step: RunStep) => {
    setSteps((prev) => [...prev, step]);
  }, []);

  return { run, steps, todos, loading, error, addStep, setRun, setSteps, setTodos };
}

/**
 * Simple hook for fetching run steps with optional polling fallback.
 * Useful when you only need steps (not full subscription).
 */
export function useRunSteps(runId: string, pollInterval?: number) {
  const [steps, setSteps] = useState<RunStep[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    let interval: ReturnType<typeof setInterval> | undefined;

    async function fetchSteps() {
      try {
        const data = await runsApi.getRunSteps(runId);
        if (!cancelled) setSteps(data);
      } catch {
        // silently fail on poll
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    fetchSteps();
    if (pollInterval) {
      interval = setInterval(fetchSteps, pollInterval);
    }

    return () => {
      cancelled = true;
      if (interval) clearInterval(interval);
    };
  }, [runId, pollInterval]);

  return { steps, loading };
}
