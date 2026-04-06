"use client";

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { ChevronRight, ChevronUp, Loader2, Clock, CheckCircle2, XCircle, ExternalLink, AlertCircle } from "lucide-react";
import { api } from "@/shared/api";
import { toast } from "sonner";
import { useWSEvent } from "@/features/realtime";
import type { TaskCompletedPayload, TaskFailedPayload, TaskCancelledPayload } from "@/shared/types/events";
import type { AgentTask } from "@/shared/types/agent";
import { cn } from "@/lib/utils";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { buildTimeline, formatDuration, type TimelineItem } from "./timeline-helpers";
import { TimelineRow } from "./timeline-row";

// ─── Runtime type guard ─────────────────────────────────────────────────────

function isTaskEventPayload(val: unknown): val is TaskCompletedPayload {
  if (typeof val !== "object" || val === null) return false;
  const v = val as Record<string, unknown>;
  return typeof v.task_id === "string" && typeof v.issue_id === "string";
}

// ─── TaskRunHistory ─────────────────────────────────────────────────────────

interface TaskRunHistoryProps {
  issueId: string;
}

export function TaskRunHistory({ issueId }: TaskRunHistoryProps) {
  const [tasks, setTasks] = useState<AgentTask[]>([]);
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setError(null);
    api
      .listTasksByIssue(issueId)
      .then(setTasks)
      .catch((e: unknown) => {
        setError(e instanceof Error ? e.message : "Failed to load execution history");
      });
  }, [issueId]);

  // Refresh when a task completes
  useWSEvent(
    "task:completed",
    useCallback((payload: unknown) => {
      if (!isTaskEventPayload(payload)) return;
      if (payload.issue_id !== issueId) return;
      api.listTasksByIssue(issueId).then((data) => { setTasks(data); setError(null); }).catch((e) => {
        console.error(e);
        toast.error("Failed to refresh execution history");
      });
    }, [issueId]),
  );

  useWSEvent(
    "task:failed",
    useCallback((payload: unknown) => {
      if (!isTaskEventPayload(payload)) return;
      if (payload.issue_id !== issueId) return;
      api.listTasksByIssue(issueId).then((data) => { setTasks(data); setError(null); }).catch((e) => {
        console.error(e);
        toast.error("Failed to refresh execution history");
      });
    }, [issueId]),
  );

  // Refresh when a task is cancelled
  useWSEvent(
    "task:cancelled",
    useCallback((payload: unknown) => {
      if (!isTaskEventPayload(payload)) return;
      if (payload.issue_id !== issueId) return;
      api.listTasksByIssue(issueId).then((data) => { setTasks(data); setError(null); }).catch((e) => {
        console.error(e);
        toast.error("Failed to refresh execution history");
      });
    }, [issueId]),
  );

  const completedTasks = tasks.filter((t) => t.status === "completed" || t.status === "failed" || t.status === "cancelled");
  if (completedTasks.length === 0 && !error) return null;

  if (error) {
    return (
      <div className="flex items-center gap-2 rounded border border-dashed px-3 py-2 text-xs text-destructive">
        <AlertCircle className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        {error}
      </div>
    );
  }

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex w-full items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors py-1">
        <ChevronRight className={cn("h-3 w-3 transition-transform", open && "rotate-90")} aria-hidden="true" />
        <Clock className="h-3 w-3" aria-hidden="true" />
        <span>Execution history ({completedTasks.length})</span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="mt-1 space-y-2">
          {completedTasks.map((task) => (
            <TaskRunEntry key={task.id} task={task} />
          ))}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

function TaskRunEntry({ task }: { task: AgentTask }) {
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<TimelineItem[] | null>(null);

  const loadMessages = useCallback(() => {
    if (items !== null) return; // already loaded
    api.listTaskMessages(task.id).then((msgs) => {
      setItems(buildTimeline(msgs));
    }).catch((e) => {
      console.error(e);
      setItems([]);
    });
  }, [task.id, items]);

  useEffect(() => {
    if (open) loadMessages();
  }, [open, loadMessages]);

  const duration = task.started_at && task.completed_at
    ? formatDuration(task.started_at, task.completed_at)
    : null;

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs hover:bg-accent/30 transition-colors border border-transparent hover:border-border">
        <ChevronRight className={cn("h-3 w-3 shrink-0 text-muted-foreground transition-transform", open && "rotate-90")} aria-hidden="true" />
        {task.status === "completed" ? (
          <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-success" aria-hidden="true" />
        ) : (
          <XCircle className="h-3.5 w-3.5 shrink-0 text-destructive" aria-hidden="true" />
        )}
        <span className="text-muted-foreground">
          {new Date(task.created_at).toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}
        </span>
        {duration && <span className="text-muted-foreground">{duration}</span>}
        <span className={cn("ml-auto capitalize", task.status === "completed" ? "text-success" : "text-destructive")}>
          {task.status}
        </span>
        <Link
          href={`/tasks/${task.id}`}
          className="text-muted-foreground hover:text-foreground transition-colors"
          onClick={(e) => e.stopPropagation()}
          aria-label="View task details"
        >
          <ExternalLink className="h-3 w-3" />
        </Link>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="ml-5 mt-1 max-h-64 overflow-y-auto rounded border bg-muted/30 px-3 py-2 space-y-0.5">
          {items === null ? (
            <div className="flex items-center gap-2 text-xs text-muted-foreground py-2">
              <Loader2 className="h-3 w-3 animate-spin" aria-hidden="true" />
              Loading...
            </div>
          ) : items.length === 0 ? (
            <p className="text-xs text-muted-foreground py-2">No execution data recorded.</p>
          ) : (
            items.map((item, idx) => (
              <TimelineRow key={`${item.seq}-${idx}`} item={item} />
            ))
          )}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}
