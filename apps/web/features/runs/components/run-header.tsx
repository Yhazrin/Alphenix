"use client";

import type { Run, RunStep } from "@/shared/types";
import { Badge } from "@/components/ui/badge";
import { Clock, DollarSign, Cpu, ArrowLeft } from "lucide-react";
import Link from "next/link";

interface RunHeaderProps {
  run: Run;
  steps: RunStep[];
}

function phaseColor(phase: string): string {
  switch (phase) {
    case "executing":
    case "planning":
    case "reviewing":
      return "bg-blue-500/10 text-blue-600 border-blue-200";
    case "completed":
      return "bg-green-500/10 text-green-600 border-green-200";
    case "failed":
      return "bg-red-500/10 text-red-600 border-red-200";
    case "cancelled":
      return "bg-yellow-500/10 text-yellow-600 border-yellow-200";
    default:
      return "bg-muted text-muted-foreground";
  }
}

function formatDuration(startedAt: string | null, completedAt: string | null): string {
  if (!startedAt) return "—";
  const start = new Date(startedAt).getTime();
  if (isNaN(start)) return "—";
  const end = completedAt ? new Date(completedAt).getTime() : Date.now();
  const secs = Math.round((end - start) / 1000);
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  const remSecs = secs % 60;
  return `${mins}m ${remSecs}s`;
}

export function RunHeader({ run, steps }: RunHeaderProps) {
  const completedSteps = steps.filter((s) => s.completed_at).length;
  const erroredSteps = steps.filter((s) => s.is_error).length;

  return (
    <div className="border-b px-4 py-3 space-y-2">
      <div className="flex items-center gap-2">
        <Link
          href={`/issues/${run.issue_id}`}
          className="text-muted-foreground hover:text-foreground transition-colors"
        >
          <ArrowLeft className="h-4 w-4" />
        </Link>
        <h1 className="text-sm font-semibold" data-testid="run-header-id">
          Run {run.id.slice(0, 12)}
        </h1>
        <Badge variant="outline" className={`text-[10px] ${phaseColor(run.phase)}`}>
          {run.phase}
        </Badge>
      </div>

      <div className="flex items-center gap-4 text-xs text-muted-foreground">
        <span className="flex items-center gap-1">
          <Cpu className="h-3 w-3" aria-hidden="true" />
          {run.model_name}
        </span>
        <span className="flex items-center gap-1">
          <Clock className="h-3 w-3" aria-hidden="true" />
          {formatDuration(run.started_at, run.completed_at)}
        </span>
        <span className="flex items-center gap-1">
          <DollarSign className="h-3 w-3" aria-hidden="true" />
          ${run.estimated_cost_usd.toFixed(4)}
        </span>
        <span>{run.input_tokens.toLocaleString()} in / {run.output_tokens.toLocaleString()} out</span>
        <span>{completedSteps}/{steps.length} steps{erroredSteps > 0 ? ` (${erroredSteps} errors)` : ""}</span>
      </div>
    </div>
  );
}
