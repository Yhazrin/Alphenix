"use client";

import { useState, useEffect, useCallback } from "react";
import {
  FileText,
  Clock,
  CheckCircle2,
  XCircle,
  AlertCircle,
  Loader2,
  MessageSquare,
  Flag,
  GitBranch,
  ChevronRight,
  Copy,
  Check,
} from "lucide-react";
import type { AgentTask, TaskReport, TaskTimelineEvent } from "@/shared/types";
import { tasksApi } from "@/shared/api/tasks";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { EmptyState } from "@/components/common/empty-state";

function StatusIcon({ status }: { status: string }) {
  switch (status) {
    case "completed":
      return <CheckCircle2 className="h-4 w-4 text-success" />;
    case "failed":
      return <XCircle className="h-4 w-4 text-destructive" />;
    case "running":
      return <Loader2 className="h-4 w-4 animate-spin text-info" />;
    case "cancelled":
      return <AlertCircle className="h-4 w-4 text-muted-foreground" />;
    default:
      return <Clock className="h-4 w-4 text-muted-foreground" />;
  }
}

function formatDuration(start?: string | null, end?: string | null): string {
  if (!start) return "—";
  const startTime = new Date(start).getTime();
  if (isNaN(startTime)) return "—";
  const endTime = end ? new Date(end).getTime() : Date.now();
  const ms = endTime - startTime;
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  return `${(ms / 60_000).toFixed(1)}m`;
}

function TimelineEventIcon({ eventType }: { eventType: string }) {
  switch (eventType) {
    case "message":
      return <MessageSquare className="h-3.5 w-3.5 text-info" />;
    case "checkpoint":
      return <Flag className="h-3.5 w-3.5 text-warning" />;
    case "review":
      return <GitBranch className="h-3.5 w-3.5 text-purple-500" />;
    default:
      return <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />;
  }
}

function SummaryTab({ report }: { report: TaskReport }) {
  return (
    <div className="space-y-4">
      {/* Status Card */}
      <div className="rounded-lg border p-4">
        <div className="flex items-center gap-3">
          <StatusIcon status={report.status} />
          <div>
            <div className="flex items-center gap-2">
              <span className="text-sm font-semibold capitalize">{report.status}</span>
              {report.review_status !== "none" && (
                <Badge variant="outline" className="text-[10px]">
                  {report.review_status}
                </Badge>
              )}
            </div>
            <p className="text-xs text-muted-foreground mt-0.5">
              {report.issue_title}
            </p>
          </div>
        </div>
      </div>

      {/* Details Grid */}
      <div className="grid grid-cols-2 gap-3">
        <div className="rounded-lg border p-3">
          <div className="text-[10px] uppercase tracking-wide text-muted-foreground">Agent</div>
          <div className="text-sm font-medium mt-0.5">{report.agent_name}</div>
        </div>
        <div className="rounded-lg border p-3">
          <div className="text-[10px] uppercase tracking-wide text-muted-foreground">Runtime</div>
          <div className="text-sm font-medium mt-0.5">{report.runtime_name ?? "—"}</div>
        </div>
        <div className="rounded-lg border p-3">
          <div className="text-[10px] uppercase tracking-wide text-muted-foreground">Duration</div>
          <div className="text-sm font-medium mt-0.5">
            {formatDuration(report.started_at, report.completed_at)}
          </div>
        </div>
        <div className="rounded-lg border p-3">
          <div className="text-[10px] uppercase tracking-wide text-muted-foreground">Priority</div>
          <div className="text-sm font-medium mt-0.5">{report.priority}</div>
        </div>
        <div className="rounded-lg border p-3">
          <div className="text-[10px] uppercase tracking-wide text-muted-foreground">Messages</div>
          <div className="text-sm font-medium mt-0.5">{report.message_count}</div>
        </div>
        <div className="rounded-lg border p-3">
          <div className="text-[10px] uppercase tracking-wide text-muted-foreground">Checkpoints</div>
          <div className="text-sm font-medium mt-0.5">{report.checkpoint_count}</div>
        </div>
      </div>

      {/* Timestamps */}
      <div className="rounded-lg border p-3 space-y-2">
        <div className="text-[10px] uppercase tracking-wide text-muted-foreground">Timeline</div>
        {report.dispatched_at && (
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground">Dispatched</span>
            <span>{new Date(report.dispatched_at).toLocaleString()}</span>
          </div>
        )}
        {report.started_at && (
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground">Started</span>
            <span>{new Date(report.started_at).toLocaleString()}</span>
          </div>
        )}
        {report.completed_at && (
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground">Completed</span>
            <span>{new Date(report.completed_at).toLocaleString()}</span>
          </div>
        )}
      </div>

      {/* Error */}
      {report.error && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-3">
          <div className="text-[10px] uppercase tracking-wide text-destructive mb-1">Error</div>
          <pre className="text-xs text-destructive whitespace-pre-wrap font-mono">{report.error}</pre>
        </div>
      )}
    </div>
  );
}

function TimelineTab({ taskId }: { taskId: string }) {
  const [events, setEvents] = useState<TaskTimelineEvent[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadTimeline = useCallback(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    tasksApi
      .getTimeline(taskId)
      .then((data) => { if (!cancelled) setEvents(data); })
      .catch((e: unknown) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : "Failed to load timeline");
        setEvents([]);
      })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [taskId]);

  useEffect(() => {
    const cleanup = loadTimeline();
    return cleanup;
  }, [loadTimeline]);

  if (loading) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="flex gap-3">
            <Skeleton className="h-7 w-7 rounded-full shrink-0" />
            <div className="flex-1 space-y-1.5">
              <Skeleton className="h-4 w-1/3" />
              <Skeleton className="h-3 w-2/3" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <EmptyState
        icon={AlertCircle}
        title="Failed to load timeline"
        description={error}
        actions={[{ label: "Retry", onClick: loadTimeline }]}
      />
    );
  }

  if (!events || events.length === 0) {
    return (
      <EmptyState
        icon={Clock}
        title="No timeline events yet"
        description="Events will appear here as the task progresses."
      />
    );
  }

  return (
    <div className="relative space-y-0">
      {/* Vertical line */}
      <div className="absolute left-[13px] top-4 bottom-4 w-px bg-border" />

      {events.map((event) => (
        <div key={event.id} className="relative flex gap-3 py-3">
          <div className="relative z-10 flex h-7 w-7 shrink-0 items-center justify-center rounded-full border bg-background">
            <TimelineEventIcon eventType={event.event_type} />
          </div>
          <div className="min-w-0 flex-1 pb-3">
            <div className="flex items-center gap-2">
              <span className="text-xs font-medium">{event.title}</span>
              <Badge variant="outline" className="text-[10px]">
                {event.event_type}
              </Badge>
            </div>
            {event.detail && (
              <p className="mt-1 text-xs text-muted-foreground line-clamp-3">{event.detail}</p>
            )}
            <span className="text-[10px] text-muted-foreground mt-1 block">
              {(() => { const d = new Date(event.timestamp); return isNaN(d.getTime()) ? "" : d.toLocaleString(); })()}
            </span>
          </div>
        </div>
      ))}
    </div>
  );
}

function OutputTab({ report }: { report: TaskReport }) {
  const [copied, setCopied] = useState(false);

  if (!report.result && !report.error) {
    return (
      <EmptyState
        icon={FileText}
        title="No output yet"
        description="Task output will appear here once the agent produces results."
      />
    );
  }

  const content = report.result
    ? typeof report.result === "string"
      ? report.result
      : JSON.stringify(report.result, null, 2)
    : report.error ?? "";

  const handleCopy = () => {
    navigator.clipboard.writeText(content).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    }).catch(() => {
      toast.error("Failed to copy to clipboard");
    });
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-xs text-muted-foreground">
          {report.result ? "Result" : "Error output"}
        </span>
        <Button variant="ghost" size="icon-sm" onClick={handleCopy} aria-label="Copy output">
          {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
        </Button>
      </div>
      <pre className="max-h-[60vh] overflow-auto rounded bg-muted/50 p-4 text-xs whitespace-pre-wrap break-words">
        {content}
      </pre>
    </div>
  );
}

export function TaskReportPanel({
  task,
  onClose,
}: {
  task: AgentTask;
  onClose: () => void;
}) {
  const [report, setReport] = useState<TaskReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadReport = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const r = await tasksApi.getReport(task.id);
      setReport(r);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load report");
    } finally {
      setLoading(false);
    }
  }, [task.id]);

  useEffect(() => {
    loadReport();
  }, [loadReport]);

  if (loading) {
    return (
      <div className="space-y-4 p-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }

  if (error) {
    return (
      <EmptyState
        icon={FileText}
        title="Failed to load report"
        description={error}
        actions={[
          { label: "Retry", onClick: loadReport },
          { label: "Close", onClick: onClose, variant: "outline" },
        ]}
      />
    );
  }

  if (!report) return null;

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
        <div className="flex items-center gap-2">
          <FileText className="h-4 w-4 text-muted-foreground" />
          <h3 className="text-sm font-semibold">Task Report</h3>
          <Badge variant="outline" className="text-[10px]">
            {task.id.slice(0, 8)}
          </Badge>
        </div>
        <Button variant="ghost" size="sm" onClick={onClose}>
          Close
        </Button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6">
        <Tabs defaultValue="summary">
          <TabsList variant="line" className="mb-4">
            <TabsTrigger value="summary" className="text-xs">Summary</TabsTrigger>
            <TabsTrigger value="timeline" className="text-xs">Timeline</TabsTrigger>
            <TabsTrigger value="output" className="text-xs">Output</TabsTrigger>
          </TabsList>
          <TabsContent value="summary">
            <SummaryTab report={report} />
          </TabsContent>
          <TabsContent value="timeline">
            <TimelineTab taskId={task.id} />
          </TabsContent>
          <TabsContent value="output">
            <OutputTab report={report} />
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}
