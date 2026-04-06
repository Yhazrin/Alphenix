"use client";

import { useState } from "react";
import {
  ChevronRight,
  ChevronUp,
  Clock,
  FileText,
  CheckCircle2,
  XCircle,
  Circle,
  Loader2,
  ListTodo,
  Wrench,
  AlertCircle,
} from "lucide-react";
import type { Run, RunStep, RunTodo } from "@/shared/types";
import { useRunTimeline } from "../hooks/use-run-timeline";
import { cn } from "@/lib/utils";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { Badge } from "@/components/ui/badge";

// ─── RunTimeline (main container) ──────────────────────────────────────────

interface RunTimelineProps {
  issueId: string;
}

export function RunTimeline({ issueId }: RunTimelineProps) {
  const { runs, stepsByRun, todosByRun, loading, error } = useRunTimeline(issueId);
  const [open, setOpen] = useState(false);

  if (loading) return null;

  if (error) {
    return (
      <div className="flex items-center gap-2 rounded border border-dashed px-3 py-2 text-xs text-destructive">
        <AlertCircle className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        {error}
      </div>
    );
  }

  if (runs.length === 0) return null;

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex w-full items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors py-1">
        <ChevronRight
          className={cn("h-3 w-3 transition-transform", open && "rotate-90")}
        />
        <Clock className="h-3 w-3" />
        <span>Agent runs ({runs.length})</span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="mt-1 space-y-2">
          {runs.map((run) => (
            <RunEntry
              key={run.id}
              run={run}
              steps={stepsByRun.get(run.id) ?? []}
              todos={todosByRun.get(run.id) ?? []}
            />
          ))}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

// ─── RunEntry (single run with expandable details) ─────────────────────────

function RunEntry({
  run,
  steps,
  todos,
}: {
  run: Run;
  steps: RunStep[];
  todos: RunTodo[];
}) {
  const [open, setOpen] = useState(false);

  const duration =
    run.started_at && run.completed_at
      ? formatRunDuration(run.started_at, run.completed_at)
      : null;

  const totalTokens = run.input_tokens + run.output_tokens;

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs hover:bg-accent/30 transition-colors border border-transparent hover:border-border">
        <ChevronRight
          className={cn(
            "h-3 w-3 shrink-0 text-muted-foreground transition-transform",
            open && "rotate-90",
          )}
        />
        <RunPhaseIcon phase={run.phase} />
        <span className="text-muted-foreground">
          {(() => { const d = new Date(run.created_at); return isNaN(d.getTime()) ? "\u2014" : d.toLocaleString(undefined, {
            month: "short",
            day: "numeric",
            hour: "2-digit",
            minute: "2-digit",
          })})()}
        </span>
        {duration && (
          <span className="text-muted-foreground">{duration}</span>
        )}
        {run.model_name && (
          <Badge variant="outline" className="text-[10px] px-1 py-0 h-4">
            {run.model_name}
          </Badge>
        )}
        {run.permission_mode && run.permission_mode !== "default" && (
          <Badge variant="secondary" className="text-[10px] px-1 py-0 h-4">
            {run.permission_mode}
          </Badge>
        )}
        <span className="ml-auto">
          <RunPhaseBadge phase={run.phase} />
        </span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="ml-5 mt-1 space-y-2">
          {/* Token usage summary */}
          {totalTokens > 0 && (
            <div className="flex items-center gap-3 text-[10px] text-muted-foreground px-2">
              <span>{totalTokens.toLocaleString()} tokens</span>
              {run.estimated_cost_usd > 0 && (
                <span>${run.estimated_cost_usd.toFixed(4)}</span>
              )}
            </div>
          )}

          {/* System prompt */}
          {run.system_prompt && (
            <RunSystemPrompt prompt={run.system_prompt} />
          )}

          {/* Steps */}
          {steps.length > 0 && (
            <RunStepLog steps={steps} />
          )}

          {/* Todos */}
          {todos.length > 0 && (
            <RunTodoList todos={todos} />
          )}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

// ─── RunStepLog (tool invocation steps) ────────────────────────────────────

function RunStepLog({ steps }: { steps: RunStep[] }) {
  const [open, setOpen] = useState(false);

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex items-center gap-1.5 text-[11px] text-muted-foreground hover:text-foreground transition-colors px-2">
        <ChevronRight
          className={cn("h-3 w-3 transition-transform", open && "rotate-90")}
        />
        <Wrench className="h-3 w-3" />
        <span>Steps ({steps.length})</span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="ml-4 mt-1 space-y-0.5 max-h-48 overflow-y-auto rounded border bg-muted/30 px-3 py-2">
          {steps.map((step, idx) => (
            <div
              key={step.id ?? idx}
              className="flex items-center gap-2 text-[11px] py-0.5"
            >
              {step.is_error ? (
                <AlertCircle className="h-3 w-3 shrink-0 text-destructive" />
              ) : step.completed_at ? (
                <CheckCircle2 className="h-3 w-3 shrink-0 text-success" />
              ) : (
                <Loader2 className="h-3 w-3 shrink-0 animate-spin text-muted-foreground" />
              )}
              <span className="font-mono text-foreground">
                {step.tool_name}
              </span>
              {step.tool_output && step.completed_at && (
                <span className="text-muted-foreground truncate max-w-[200px]">
                  {truncate(step.tool_output, 80)}
                </span>
              )}
            </div>
          ))}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

// ─── RunTodoList (agent todo items) ────────────────────────────────────────

function RunTodoList({ todos }: { todos: RunTodo[] }) {
  const [open, setOpen] = useState(false);

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex items-center gap-1.5 text-[11px] text-muted-foreground hover:text-foreground transition-colors px-2">
        <ChevronRight
          className={cn("h-3 w-3 transition-transform", open && "rotate-90")}
        />
        <ListTodo className="h-3 w-3" />
        <span>
          Todos ({todos.filter((t) => t.status === "completed").length}/
          {todos.length})
        </span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="ml-4 mt-1 space-y-1 rounded border bg-muted/30 px-3 py-2">
          {todos.map((todo) => (
            <div
              key={todo.id}
              className="flex items-start gap-2 text-[11px]"
            >
              <TodoStatusIcon status={todo.status} />
              <div className="min-w-0">
                <span
                  className={cn(
                    todo.status === "completed" &&
                      "line-through text-muted-foreground",
                  )}
                >
                  {todo.title}
                </span>
                {todo.description && (
                  <p className="text-muted-foreground/70 text-[10px] mt-0.5 leading-snug">
                    {todo.description}
                  </p>
                )}
                {todo.blocker && (
                  <p className="text-destructive text-[10px] mt-0.5">
                    Blocked: {todo.blocker}
                  </p>
                )}
              </div>
            </div>
          ))}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

// ─── RunSystemPrompt (expandable system prompt view) ────────────────────────

function RunSystemPrompt({ prompt }: { prompt: string }) {
  const [open, setOpen] = useState(false);
  const preview = truncate(prompt, 120);

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex items-center gap-1.5 text-[11px] text-muted-foreground hover:text-foreground transition-colors px-2">
        <ChevronRight
          className={cn("h-3 w-3 transition-transform", open && "rotate-90")}
        />
        <FileText className="h-3 w-3" />
        <span>System prompt</span>
        {!open && (
          <span className="text-muted-foreground/60 truncate max-w-[200px]">
            — {preview}
          </span>
        )}
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="ml-4 mt-1 rounded border bg-muted/30 px-3 py-2 max-h-48 overflow-y-auto">
          <pre className="text-[11px] text-foreground whitespace-pre-wrap break-words font-mono">
            {prompt}
          </pre>
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

// ─── Helpers ───────────────────────────────────────────────────────────────

function RunPhaseIcon({ phase }: { phase: Run["phase"] }) {
  switch (phase) {
    case "pending":
      return <Circle className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />;
    case "planning":
    case "executing":
    case "reviewing":
      return <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin text-info" />;
    case "completed":
      return <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-success" />;
    case "failed":
      return <XCircle className="h-3.5 w-3.5 shrink-0 text-destructive" />;
    case "cancelled":
      return <XCircle className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />;
    default:
      return <Circle className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />;
  }
}

function RunPhaseBadge({ phase }: { phase: Run["phase"] }) {
  const variant =
    phase === "completed"
      ? "default"
      : phase === "failed"
        ? "destructive"
        : phase === "cancelled"
          ? "secondary"
          : "outline";

  return (
    <Badge variant={variant} className="text-[10px] px-1.5 py-0 h-4 capitalize">
      {phase}
    </Badge>
  );
}

function TodoStatusIcon({ status }: { status: RunTodo["status"] }) {
  switch (status) {
    case "completed":
      return <CheckCircle2 className="h-3 w-3 shrink-0 text-success mt-0.5" />;
    case "in_progress":
      return <Loader2 className="h-3 w-3 shrink-0 animate-spin text-info mt-0.5" />;
    case "blocked":
      return <AlertCircle className="h-3 w-3 shrink-0 text-destructive mt-0.5" />;
    default:
      return <Circle className="h-3 w-3 shrink-0 text-muted-foreground mt-0.5" />;
  }
}

function formatRunDuration(start: string, end: string): string {
  const ms = new Date(end).getTime() - new Date(start).getTime();
  if (isNaN(ms)) return "---";
  if (ms < 1000) return `${ms}ms`;
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  if (minutes < 60) return `${minutes}m ${remainingSeconds}s`;
  const hours = Math.floor(minutes / 60);
  return `${hours}h ${minutes % 60}m`;
}

function truncate(str: string, maxLen: number): string {
  if (str.length <= maxLen) return str;
  return str.slice(0, maxLen) + "...";
}
