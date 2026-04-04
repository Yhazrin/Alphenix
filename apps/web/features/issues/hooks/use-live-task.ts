"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { api } from "@/shared/api";
import { useWSEvent } from "@/features/realtime";
import { toast } from "sonner";
import type {
  TaskMessagePayload,
  TaskCompletedPayload,
  TaskFailedPayload,
  TaskCancelledPayload,
  TaskProgressPayload,
} from "@/shared/types/events";
import type { AgentTask } from "@/shared/types/agent";
import { buildTimeline, type TimelineItem } from "../components/timeline-helpers";

// ─── Runtime type guards ────────────────────────────────────────────────────

function hasStringProp(obj: Record<string, unknown>, key: string): boolean {
  return key in obj && typeof obj[key] === "string";
}

function hasNumberProp(obj: Record<string, unknown>, key: string): boolean {
  return key in obj && typeof obj[key] === "number";
}

function isTaskMessagePayload(val: unknown): val is TaskMessagePayload {
  if (typeof val !== "object" || val === null) return false;
  const v = val as Record<string, unknown>;
  return (
    hasStringProp(v, "task_id") &&
    hasStringProp(v, "issue_id") &&
    hasNumberProp(v, "seq") &&
    typeof v.type === "string"
  );
}

function isTaskEventPayload(val: unknown): val is TaskCompletedPayload {
  if (typeof val !== "object" || val === null) return false;
  const v = val as Record<string, unknown>;
  return hasStringProp(v, "task_id") && hasStringProp(v, "issue_id");
}

function isTaskProgressPayload(val: unknown): val is TaskProgressPayload {
  if (typeof val !== "object" || val === null) return false;
  const v = val as Record<string, unknown>;
  return (
    hasStringProp(v, "task_id") &&
    hasStringProp(v, "summary") &&
    hasNumberProp(v, "step") &&
    hasNumberProp(v, "total")
  );
}

// ─── Hook ───────────────────────────────────────────────────────────────────

const MAX_SEEN_SEQS = 500;

export interface UseLiveTaskResult {
  activeTask: AgentTask | null;
  items: TimelineItem[];
  progress: { summary: string; step: number; total: number } | null;
  cancelling: boolean;
  handleCancel: () => Promise<void>;
  seenSeqsRef: React.RefObject<Set<string>>;
}

export function useLiveTask(issueId: string): UseLiveTaskResult {
  const [activeTask, setActiveTask] = useState<AgentTask | null>(null);
  const [items, setItems] = useState<TimelineItem[]>([]);
  const [progress, setProgress] = useState<{ summary: string; step: number; total: number } | null>(null);
  const [cancelling, setCancelling] = useState(false);
  const seenSeqs = useRef(new Set<string>());

  // Check for active task on mount
  useEffect(() => {
    let cancelled = false;
    api.getActiveTaskForIssue(issueId).then(({ task }) => {
      if (!cancelled) {
        setActiveTask(task);
        if (task) {
          api.listTaskMessages(task.id).then((msgs) => {
            if (!cancelled) {
              const timeline = buildTimeline(msgs);
              setItems(timeline);
              for (const m of msgs) seenSeqs.current.add(`${m.task_id}:${m.seq}`);
            }
          }).catch(console.error);
        }
      }
    }).catch(console.error);

    return () => { cancelled = true; };
  }, [issueId]);

  // Handle real-time task messages
  useWSEvent(
    "task:message",
    useCallback((payload: unknown) => {
      if (!isTaskMessagePayload(payload)) return;
      const msg = payload;
      if (msg.issue_id !== issueId) return;
      const key = `${msg.task_id}:${msg.seq}`;
      if (seenSeqs.current.has(key)) return;
      seenSeqs.current.add(key);
      // Sliding window cap: evict oldest entries when exceeding limit
      if (seenSeqs.current.size > MAX_SEEN_SEQS) {
        const iter = seenSeqs.current.values();
        while (seenSeqs.current.size > MAX_SEEN_SEQS) {
          const next = iter.next();
          if (next.done) break;
          seenSeqs.current.delete(next.value);
        }
      }

      setItems((prev) => {
        const item: TimelineItem = {
          seq: msg.seq,
          type: msg.type,
          tool: msg.tool,
          content: msg.content,
          input: msg.input,
          output: msg.output,
        };
        const next = [...prev, item];
        next.sort((a, b) => a.seq - b.seq);
        return next;
      });
    }, [issueId]),
  );

  // Handle task completion/failure
  useWSEvent(
    "task:completed",
    useCallback((payload: unknown) => {
      if (!isTaskEventPayload(payload)) return;
      if (payload.issue_id !== issueId) return;
      setActiveTask(null);
      setItems([]);
      setProgress(null);
      seenSeqs.current.clear();
      setCancelling(false);
    }, [issueId]),
  );

  useWSEvent(
    "task:failed",
    useCallback((payload: unknown) => {
      if (!isTaskEventPayload(payload)) return;
      if (payload.issue_id !== issueId) return;
      setActiveTask(null);
      setItems([]);
      setProgress(null);
      seenSeqs.current.clear();
      setCancelling(false);
    }, [issueId]),
  );

  useWSEvent(
    "task:cancelled",
    useCallback((payload: unknown) => {
      if (!isTaskEventPayload(payload)) return;
      if (payload.issue_id !== issueId) return;
      setActiveTask(null);
      setItems([]);
      setProgress(null);
      seenSeqs.current.clear();
      setCancelling(false);
    }, [issueId]),
  );

  // Pick up new tasks — skip if we're already showing an active task to avoid
  // replacing its timeline mid-execution (per-issue serialization in the
  // backend prevents this race, but this is a defensive safeguard).
  useWSEvent(
    "task:dispatch",
    useCallback(() => {
      if (activeTask) return;
      api.getActiveTaskForIssue(issueId).then(({ task }) => {
        if (task) {
          setActiveTask(task);
          setItems([]);
          setProgress(null);
          seenSeqs.current.clear();
        }
      }).catch(console.error);
    }, [issueId, activeTask]),
  );

  // Handle task progress updates
  useWSEvent(
    "task:progress",
    useCallback((payload: unknown) => {
      if (!isTaskProgressPayload(payload)) return;
      if (activeTask && payload.task_id !== activeTask.id) return;
      setProgress({ summary: payload.summary, step: payload.step, total: payload.total });
    }, [activeTask]),
  );

  const handleCancel = useCallback(async () => {
    if (!activeTask || cancelling) return;
    setCancelling(true);
    try {
      await api.cancelTask(issueId, activeTask.id);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to cancel task");
      setCancelling(false);
    }
  }, [activeTask, issueId, cancelling]);

  return { activeTask, items, progress, cancelling, handleCancel, seenSeqsRef: seenSeqs };
}
