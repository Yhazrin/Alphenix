"use client";

import { useState } from "react";
import type { RunStep, RunArtifact } from "@/shared/types";
import { Badge } from "@/components/ui/badge";
import {
  ChevronDown,
  ChevronRight,
  Brain,
  FileText,
  Terminal,
  Wrench,
  AlertCircle,
  Paperclip,
} from "lucide-react";

interface TimelineFeedProps {
  steps: RunStep[];
  artifacts: RunArtifact[];
  onArtifactClick: (artifact: RunArtifact) => void;
}

function toolIcon(toolName: string) {
  switch (toolName) {
    case "thinking":
      return <Brain className="h-3.5 w-3.5 text-purple-500" />;
    case "read_file":
      return <FileText className="h-3.5 w-3.5 text-blue-500" />;
    case "bash":
      return <Terminal className="h-3.5 w-3.5 text-green-500" />;
    case "edit_file":
      return <Wrench className="h-3.5 w-3.5 text-orange-500" />;
    default:
      return <Wrench className="h-3.5 w-3.5 text-muted-foreground" />;
  }
}

function toolLabel(step: RunStep): string {
  if (step.tool_name === "thinking") return "Thinking";
  if (step.tool_name === "bash") return (step.tool_input.command as string) ?? "bash";
  if (step.tool_name === "read_file") return `Read ${(step.tool_input.path as string)?.split("/").pop() ?? ""}`;
  if (step.tool_name === "edit_file") {
    const action = step.tool_input.action as string;
    const path = (step.tool_input.path as string)?.split("/").pop() ?? "";
    return action ? `${action} → ${path}` : `Edit ${path}`;
  }
  return step.tool_name;
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function StepCard({ step, artifact, onArtifactClick }: { step: RunStep; artifact?: RunArtifact; onArtifactClick: (a: RunArtifact) => void }) {
  const [expanded, setExpanded] = useState(step.tool_name === "thinking");

  return (
    <div
      className={`group relative flex gap-3 py-2 ${step.is_error ? "border-l-2 border-l-red-400 pl-3" : "pl-3"}`}
      data-testid={`timeline-step-${step.seq}`}
    >
      {/* Timeline dot */}
      <div className="mt-1 shrink-0">{toolIcon(step.tool_name)}</div>

      {/* Content */}
      <div className="min-w-0 flex-1">
        <button
          type="button"
          className="flex w-full items-center gap-2 text-left"
          onClick={() => setExpanded(!expanded)}
        >
          <span className="truncate text-sm font-medium">
            {toolLabel(step)}
          </span>
          {step.is_error && (
            <Badge variant="destructive" className="text-[10px] px-1 py-0">error</Badge>
          )}
          <span className="ml-auto shrink-0 text-[10px] text-muted-foreground">
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
            {step.tool_name === "thinking" ? (
              <p className="text-xs text-muted-foreground leading-relaxed whitespace-pre-wrap">
                {step.tool_output}
              </p>
            ) : (
              <>
                {Object.keys(step.tool_input).length > 0 && (
                  <pre className="text-[11px] bg-muted rounded p-2 overflow-x-auto">
                    {JSON.stringify(step.tool_input, null, 2)}
                  </pre>
                )}
                {step.tool_output && (
                  <pre className={`text-[11px] rounded p-2 overflow-x-auto ${step.is_error ? "bg-red-50 dark:bg-red-950/20 text-red-700 dark:text-red-300" : "bg-muted"}`}>
                    {step.tool_output}
                  </pre>
                )}
              </>
            )}

            {artifact && (
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

export function TimelineFeed({ steps, artifacts, onArtifactClick }: TimelineFeedProps) {
  const artifactByStep = new Map<string, RunArtifact>();
  for (const a of artifacts) {
    if (a.step_id) artifactByStep.set(a.step_id, a);
  }

  return (
    <div className="space-y-0.5" data-testid="timeline-feed">
      {steps.map((step) => (
        <StepCard
          key={step.id}
          step={step}
          artifact={artifactByStep.get(step.id)}
          onArtifactClick={onArtifactClick}
        />
      ))}
    </div>
  );
}
