"use client";

import { useState, useEffect } from "react";
import {
  ListTodo,
  FileText,
  AlertCircle,
} from "lucide-react";
import type {
  Agent,
  AgentTask,
} from "@/shared/types";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { api } from "@/shared/api";
import { useIssueStore } from "@/features/issues";
import { taskStatusConfig } from "./agent-configs";
import { TaskReportPanel } from "./task-report-panel";

export function TasksTab({ agent }: { agent: Agent }) {
  const [tasks, setTasks] = useState<AgentTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reportTask, setReportTask] = useState<AgentTask | null>(null);
  const issues = useIssueStore((s) => s.issues);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    api
      .listAgentTasks(agent.id)
      .then((data) => { if (!cancelled) setTasks(data); })
      .catch((e: unknown) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : "Failed to load tasks");
        setTasks([]);
      })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [agent.id]);

  if (loading) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="flex items-center gap-3 rounded-lg border px-4 py-3">
            <Skeleton className="h-4 w-4 rounded shrink-0" />
            <div className="flex-1 space-y-1.5">
              <Skeleton className="h-4 w-1/2" />
              <Skeleton className="h-3 w-1/3" />
            </div>
            <Skeleton className="h-4 w-16" />
          </div>
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-12">
        <AlertCircle className="h-8 w-8 text-destructive/60" aria-hidden="true" />
        <p className="mt-3 text-sm text-destructive">{error}</p>
      </div>
    );
  }

  // Sort: active tasks (running > dispatched > queued) first, then completed/failed by date
  const activeStatuses = ["running", "dispatched", "queued"];
  const sortedTasks = [...tasks].sort((a, b) => {
    const aActive = activeStatuses.indexOf(a.status);
    const bActive = activeStatuses.indexOf(b.status);
    const aIsActive = aActive !== -1;
    const bIsActive = bActive !== -1;
    if (aIsActive && !bIsActive) return -1;
    if (!aIsActive && bIsActive) return 1;
    if (aIsActive && bIsActive) return aActive - bActive;
    return (new Date(b.created_at).getTime() || 0) - (new Date(a.created_at).getTime() || 0);
  });

  const issueMap = new Map(issues.map((i) => [i.id, i]));

  if (reportTask) {
    return (
      <TaskReportPanel task={reportTask} onClose={() => setReportTask(null)} />
    );
  }

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold">Task Queue</h3>
        <p className="text-xs text-muted-foreground mt-0.5">
          Issues assigned to this agent and their execution status.
        </p>
      </div>

      {tasks.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-12">
          <ListTodo className="h-8 w-8 text-muted-foreground/40" aria-hidden="true" />
          <p className="mt-3 text-sm text-muted-foreground">No tasks in queue</p>
          <p className="mt-1 text-xs text-muted-foreground">
            Assign an issue to this agent to get started.
          </p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {sortedTasks.map((task) => {
            const config = taskStatusConfig[task.status] ?? taskStatusConfig.queued!;
            const Icon = config.icon;
            const issue = issueMap.get(task.issue_id);
            const isActive = task.status === "running" || task.status === "dispatched";
            const isRunning = task.status === "running";

            return (
              <div
                key={task.id}
                className={`flex items-center gap-3 rounded-lg border px-4 py-3 ${
                  isRunning
                    ? "border-success/40 bg-success/5"
                    : task.status === "dispatched"
                      ? "border-info/40 bg-info/5"
                      : ""
                }`}
              >
                <Icon
                  aria-hidden="true"
                  className={`h-4 w-4 shrink-0 ${config.color} ${
                    isRunning ? "animate-spin" : ""
                  }`}
                />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    {issue && (
                      <span className="shrink-0 text-xs font-mono text-muted-foreground">
                        {issue.identifier}
                      </span>
                    )}
                    <span className={`text-sm truncate ${isActive ? "font-medium" : ""}`}>
                      {issue?.title ?? `Issue ${task.issue_id.slice(0, 8)}...`}
                    </span>
                  </div>
                  <div className="text-xs text-muted-foreground mt-0.5">
                    {isRunning && task.started_at
                      ? `Started ${new Date(task.started_at).toLocaleString()}`
                      : task.status === "dispatched" && task.dispatched_at
                        ? `Dispatched ${new Date(task.dispatched_at).toLocaleString()}`
                        : task.status === "completed" && task.completed_at
                          ? `Completed ${new Date(task.completed_at).toLocaleString()}`
                          : task.status === "failed" && task.completed_at
                            ? `Failed ${new Date(task.completed_at).toLocaleString()}`
                            : `Queued ${new Date(task.created_at).toLocaleString()}`}
                  </div>
                </div>
                <span className={`shrink-0 text-xs font-medium ${config.color}`}>
                  {config.label}
                </span>
                {(task.status === "completed" || task.status === "failed" || task.status === "cancelled") && (
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    aria-label="View report"
                    onClick={() => setReportTask(task)}
                    className="shrink-0 text-muted-foreground hover:text-foreground"
                  >
                    <FileText className="h-3.5 w-3.5" />
                  </Button>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
