"use client";

import { useState } from "react";
import type { RunStep, RunArtifact } from "@/shared/types";
import { Badge } from "@/components/ui/badge";
import { StepInputRenderer } from "./step-input-renderer";
import {
  ChevronDown,
  ChevronRight,
  Brain,
  FileText,
  Terminal,
  Wrench,
  Paperclip,
  MessageSquare,
  AlertCircle,
} from "lucide-react";

interface StepCardProps {
  step: RunStep;
  artifact?: RunArtifact;
  onArtifactClick?: (artifact: RunArtifact) => void;
}

function stepIcon(step: RunStep) {
  if (step.step_type === "thinking") return <Brain className="h-3.5 w-3.5 text-purple-500" />;
  if (step.step_type === "text") return <MessageSquare className="h-3.5 w-3.5 text-blue-500" />;
  if (step.step_type === "error") return <AlertCircle className="h-3.5 w-3.5 text-red-500" />;
  switch (step.tool_name) {
    case "read_file":
    case "Read":
      return <FileText className="h-3.5 w-3.5 text-blue-500" />;
    case "bash":
    case "Bash":
      return <Terminal className="h-3.5 w-3.5 text-green-500" />;
    case "edit_file":
    case "Edit":
      return <Wrench className="h-3.5 w-3.5 text-orange-500" />;
    default:
      return <Wrench className="h-3.5 w-3.5 text-muted-foreground" />;
  }
}

function stepLabel(step: RunStep): string {
  if (step.step_type === "thinking") return "Thinking";
  if (step.step_type === "text") return "Response";
  if (step.step_type === "error") return "Error";
  if (step.tool_name === "bash" || step.tool_name === "Bash")
    return (step.tool_input.command as string) ?? (step.tool_input.cmd as string) ?? "bash";
  if (step.tool_name === "read_file" || step.tool_name === "Read") {
    const path = (step.tool_input.path as string) ?? "";
    return `Read ${path.split("/").pop() ?? path}`;
  }
  if (step.tool_name === "edit_file" || step.tool_name === "Edit") {
    const action = step.tool_input.action as string;
    const path = (step.tool_input.path as string)?.split("/").pop() ?? "";
    return action ? `${action} → ${path}` : `Edit ${path}`;
  }
  return step.tool_name;
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "—";
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function elapsedMs(step: RunStep): number {
  if (!step.completed_at) return 0;
  const ms = new Date(step.completed_at).getTime() - new Date(step.started_at).getTime();
  return isNaN(ms) ? 0 : ms;
}

function formatElapsed(step: RunStep): string {
  const ms = elapsedMs(step);
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

export function StepCard({ step, artifact, onArtifactClick }: StepCardProps) {
  const isThinkingOrText = step.step_type === "thinking" || step.step_type === "text";
  const [expanded, setExpanded] = useState(isThinkingOrText);

  return (
    <div
      className={`group relative flex gap-3 py-2 ${step.is_error ? "border-l-2 border-l-red-400 pl-3" : "pl-3"}`}
      data-testid={`timeline-step-${step.seq}`}
      id={`step-${step.seq}`}
    >
      {/* Timeline dot */}
      <div className="mt-1 shrink-0">{stepIcon(step)}</div>

      {/* Content */}
      <div className="min-w-0 flex-1">
        <button
          type="button"
          className="flex w-full items-center gap-2 text-left"
          onClick={() => setExpanded(!expanded)}
        >
          <span className="truncate text-sm font-medium">
            {stepLabel(step)}
          </span>
          {step.is_error && (
            <Badge variant="destructive" className="text-[10px] px-1 py-0">error</Badge>
          )}
          <span className="ml-auto shrink-0 text-[10px] text-muted-foreground">
            {formatElapsed(step)}
          </span>
          <span className="shrink-0 text-[10px] text-muted-foreground">
            {formatTime(step.started_at)}
          </span>
          {expanded ? (
            <ChevronDown className="h-3 w-3 text-muted-foreground shrink-0" />
          ) : (
            <ChevronRight className="h-3 w-3 text-muted-foreground shrink-0" />
          )}
        </button>

        {expanded && (
          <div className="mt-1 space-y-2">
            {isThinkingOrText ? (
              <p className="text-xs text-muted-foreground leading-relaxed whitespace-pre-wrap">
                {step.tool_output}
              </p>
            ) : (
              <>
                <StepInputRenderer toolName={step.tool_name} toolInput={step.tool_input} />
                {step.tool_output && (
                  <pre className={`text-[11px] rounded p-2 overflow-x-auto ${step.is_error ? "bg-red-50 dark:bg-red-950/20 text-red-700 dark:text-red-300" : "bg-muted"}`}>
                    {step.tool_output}
                  </pre>
                )}
              </>
            )}

            {artifact && onArtifactClick && (
              <button
                type="button"
                className="flex items-center gap-1 text-[11px] text-primary hover:underline"
                onClick={() => onArtifactClick(artifact)}
              >
                <Paperclip className="h-3 w-3" />
                {artifact.name}
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
