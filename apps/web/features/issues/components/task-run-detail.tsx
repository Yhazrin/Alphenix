"use client";

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  Clock,
  CheckCircle2,
  XCircle,
  Loader2,
  AlertTriangle,
  Copy,
  Check,
  FileText,
  MessageSquare,
  GitBranch,
  Eye,
  ChevronRight,
} from "lucide-react";
import { tasksApi } from "@/shared/api/tasks";
import { agentsApi } from "@/shared/api/agents";
import { api } from "@/shared/api";
import { useWSEvent } from "@/features/realtime";
import type { TaskReport, TaskTimelineEvent } from "@/shared/types/agent";
import type { TaskCheckpoint } from "@/shared/types/collaboration";
import type { TaskContextSection } from "@/shared/types/prompt-preview";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { buildTimeline, formatDuration, type TimelineItem } from "./timeline-helpers";
import { TimelineRow } from "./timeline-row";

// ─── Helpers ───────────────────────────────────────────────────────────

function statusIcon(status: string) {
  switch (status) {
    case "completed":
      return <CheckCircle2 className="h-4 w-4 text-success" />;
    case "failed":
      return <XCircle className="h-4 w-4 text-destructive" />;
    case "cancelled":
      return <AlertTriangle className="h-4 w-4 text-warning" />;
    case "running":
      return <Loader2 className="h-4 w-4 animate-spin text-info" />;
    default:
      return <Clock className="h-4 w-4 text-muted-foreground" />;
  }
}

function statusBadgeVariant(status: string): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "completed":
      return "default";
    case "failed":
      return "destructive";
    case "cancelled":
      return "secondary";
    default:
      return "outline";
  }
}

function formatDate(s: string | null): string {
  if (!s) return "—";
  return new Date(s).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    }).catch(() => {});
  }, [text]);
  return (
    <Button variant="ghost" size="icon-sm" onClick={handleCopy} aria-label="Copy">
      {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
    </Button>
  );
}

// ─── Sub-components ────────────────────────────────────────────────────

function SummaryTab({ report }: { report: TaskReport }) {
  const duration =
    report.started_at && report.completed_at
      ? formatDuration(report.started_at, report.completed_at)
      : null;

  const fields: { label: string; value: React.ReactNode }[] = [
    { label: "Status", value: <Badge variant={statusBadgeVariant(report.status)}>{report.status}</Badge> },
    { label: "Agent", value: <Link href={`/agents/${report.agent_id}`} className="underline underline-offset-2 hover:text-foreground">{report.agent_name}</Link> },
    { label: "Issue", value: <Link href={`/issues/${report.issue_id}`} className="underline underline-offset-2 hover:text-foreground">{report.issue_title}</Link> },
    { label: "Priority", value: String(report.priority) },
    { label: "Created", value: formatDate(report.created_at) },
    { label: "Dispatched", value: formatDate(report.dispatched_at) },
    { label: "Started", value: formatDate(report.started_at) },
    { label: "Completed", value: formatDate(report.completed_at) },
    { label: "Duration", value: duration ?? "—"},
    { label: "Review", value: report.review_status },
    { label: "Messages", value: String(report.message_count) },
    { label: "Checkpoints", value: String(report.checkpoint_count) },
  ];

  if (report.runtime_name) {
    fields.splice(2, 0, { label: "Runtime", value: report.runtime_name });
  }

  return (
    <div className="space-y-2 text-sm">
      {fields.map((f) => (
        <div key={f.label} className="flex gap-6">
          <span className="w-28 shrink-0 text-muted-foreground">{f.label}</span>
          <span>{f.value}</span>
        </div>
      ))}
      {report.error && (
        <div className="flex gap-6">
          <span className="w-28 shrink-0 text-muted-foreground">Error</span>
          <span className="text-destructive">{report.error}</span>
        </div>
      )}
    </div>
  );
}

function TimelineTab({ events }: { events: TaskTimelineEvent[] }) {
  if (events.length === 0) {
    return <p className="text-sm text-muted-foreground py-8 text-center">No timeline events.</p>;
  }

  return (
    <div className="space-y-1">
      {events.map((ev) => (
        <div key={ev.id} className="flex items-start gap-3 py-2 border-b border-border/50 last:border-0">
          <div className="mt-0.5 h-2 w-2 rounded-full bg-primary/40 shrink-0" />
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <span className="font-medium text-sm">{ev.title}</span>
              <Badge variant="outline" className="text-[10px] px-1 py-0">
                {ev.event_type}
              </Badge>
            </div>
            {ev.detail && (
              <p className="text-xs text-muted-foreground mt-0.5 line-clamp-3">{ev.detail}</p>
            )}
            <span className="text-[11px] text-muted-foreground/60">
              {formatDate(ev.timestamp)}
            </span>
          </div>
        </div>
      ))}
    </div>
  );
}

function MessagesTab({ items }: { items: TimelineItem[] }) {
  if (items.length === 0) {
    return <p className="text-sm text-muted-foreground py-8 text-center">No messages recorded yet. Messages appear as the agent works.</p>;
  }

  return (
    <div className="space-y-0.5">
      {items.map((item, idx) => (
        <TimelineRow key={`${item.seq}-${idx}`} item={item} />
      ))}
    </div>
  );
}

function CheckpointsTab({ checkpoints }: { checkpoints: TaskCheckpoint[] }) {
  if (checkpoints.length === 0) {
    return <p className="text-sm text-muted-foreground py-8 text-center">No checkpoints saved.</p>;
  }

  return (
    <div className="space-y-3">
      {checkpoints.map((cp) => (
        <Collapsible key={cp.id} className="rounded border">
          <CollapsibleTrigger className="flex w-full items-center gap-2 px-3 py-2 text-sm hover:bg-accent/30 transition-colors">
            <ChevronRight className="h-3 w-3 text-muted-foreground transition-transform [[data-state=open]>&]:rotate-90" />
            <span className="font-medium">{cp.label}</span>
            <span className="ml-auto text-xs text-muted-foreground">
              {formatDate(cp.created_at)}
            </span>
          </CollapsibleTrigger>
          <CollapsibleContent>
            <div className="px-3 pb-3">
              <pre className="max-h-64 overflow-auto rounded bg-muted/50 p-3 text-xs whitespace-pre-wrap break-all">
                {JSON.stringify(cp.state, null, 2)}
              </pre>
            </div>
          </CollapsibleContent>
        </Collapsible>
      ))}
    </div>
  );
}

function ReviewTab({ report, events }: { report: TaskReport; events: TaskTimelineEvent[] }) {
  const reviewEvents = events.filter((e) => e.event_type === "review");
  const latestReview = reviewEvents.length > 0 ? reviewEvents[reviewEvents.length - 1] : null;
  const reviewMeta = latestReview?.meta as { score?: number; reviewer_type?: string } | undefined;

  return (
    <div className="space-y-4 text-sm">
      <div className="flex items-center gap-2">
        <Eye className="h-4 w-4 text-muted-foreground" />
        <span className="text-muted-foreground">Review status:</span>
        <Badge variant={report.review_status === "approved" ? "default" : report.review_status === "rejected" ? "destructive" : "outline"}>
          {report.review_status}
        </Badge>
        {reviewMeta?.reviewer_type && (
          <span className="text-xs text-muted-foreground">by {reviewMeta.reviewer_type}</span>
        )}
      </div>
      {reviewMeta?.score !== undefined && (
        <div className="flex items-center gap-2">
          <span className="text-muted-foreground">Score:</span>
          <span className="font-mono text-xs">{reviewMeta.score}/100</span>
        </div>
      )}
      {latestReview?.detail && (
        <div className="rounded border bg-muted/30 p-3">
          <p className="text-xs font-medium text-muted-foreground mb-1">Feedback</p>
          <p className="text-xs">{latestReview.detail}</p>
        </div>
      )}
      {report.review_status === "approved" && !latestReview?.detail && (
        <p className="text-muted-foreground">
          This task was automatically approved and completed.
        </p>
      )}
      {report.status === "failed" && report.error && (
        <div className="rounded border border-destructive/30 bg-destructive/5 p-3">
          <p className="text-destructive font-medium text-xs mb-1">Task failed</p>
          <p className="text-destructive/80 text-xs">{report.error}</p>
        </div>
      )}
    </div>
  );
}

function OutputTab({ report }: { report: TaskReport }) {
  const [copied, setCopied] = useState(false);

  if (!report.result && !report.error) {
    return <p className="text-sm text-muted-foreground py-8 text-center">No output recorded.</p>;
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
    }).catch(() => {});
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

function ContextPreviewTab({ taskId }: { taskId: string }) {
  const [sections, setSections] = useState<TaskContextSection[] | null>(null);
  const [finalPrompt, setFinalPrompt] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [expandedSection, setExpandedSection] = useState<string | null>(null);

  const loadPreview = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await agentsApi.previewTaskContext(taskId);
      setSections(res.sections);
      setFinalPrompt(res.final_prompt);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load context preview");
    } finally {
      setLoading(false);
    }
  }, [taskId]);

  useEffect(() => {
    loadPreview();
  }, [loadPreview]);

  const handleCopy = useCallback(async () => {
    await navigator.clipboard.writeText(finalPrompt);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }, [finalPrompt]);

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center gap-2 py-12 text-center">
        <Eye className="h-8 w-8 text-muted-foreground" />
        <p className="text-sm text-destructive">{error}</p>
        <Button variant="outline" size="sm" onClick={loadPreview}>
          Retry
        </Button>
      </div>
    );
  }

  if (!sections) return null;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold">Context Preview</h3>
          <p className="text-xs text-muted-foreground">
            This is the exact context the agent received when executing this task.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" className="h-7 text-xs" onClick={handleCopy}>
            {copied ? <Check className="mr-1.5 h-3 w-3" /> : <Copy className="mr-1.5 h-3 w-3" />}
            {copied ? "Copied" : "Copy Context"}
          </Button>
          <Button variant="outline" size="sm" className="h-7 text-xs" onClick={loadPreview}>
            Refresh
          </Button>
        </div>
      </div>

      {/* Summary */}
      <div className="rounded-lg border bg-muted/30 px-4 py-3">
        <div className="text-xs text-muted-foreground">
          {sections.length} sections &middot; {finalPrompt.length.toLocaleString()} characters
        </div>
      </div>

      {/* Section list */}
      <div className="space-y-2">
        {sections.map((section) => {
          const isExpanded = expandedSection === section.key;
          const preview =
            section.content.length > 120 ? section.content.slice(0, 120) + "..." : section.content;

          return (
            <div
              key={section.key}
              className="rounded-lg border transition-colors hover:bg-muted/20"
            >
              <button
                className="flex w-full items-center gap-3 px-4 py-3 text-left"
                onClick={() => setExpandedSection(isExpanded ? null : section.key)}
              >
                <ChevronRight
                  className={`h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform ${
                    isExpanded ? "rotate-90" : ""
                  }`}
                />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-medium">{section.title}</span>
                    <Badge variant="outline" className="text-[10px]">
                      {section.source}
                    </Badge>
                  </div>
                  {!isExpanded && (
                    <p className="mt-0.5 text-[11px] text-muted-foreground truncate font-mono">
                      {preview}
                    </p>
                  )}
                </div>
                <span className="text-[10px] text-muted-foreground shrink-0">
                  {section.content.length} chars
                </span>
              </button>
              {isExpanded && (
                <div className="border-t px-4 py-3">
                  <pre className="text-[11px] leading-relaxed whitespace-pre-wrap font-mono text-muted-foreground max-h-80 overflow-y-auto">
                    {section.content}
                  </pre>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="mt-2 h-6 text-[10px]"
                    onClick={async (e) => {
                      e.stopPropagation();
                      await navigator.clipboard.writeText(section.content);
                    }}
                  >
                    <Copy className="mr-1 h-3 w-3" /> Copy section
                  </Button>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ─── Main component ────────────────────────────────────────────────────

interface TaskRunDetailProps {
  taskId: string;
}

export function TaskRunDetail({ taskId }: TaskRunDetailProps) {
  const [report, setReport] = useState<TaskReport | null>(null);
  const [timeline, setTimeline] = useState<TaskTimelineEvent[]>([]);
  const [messages, setMessages] = useState<TimelineItem[]>([]);
  const [checkpoints, setCheckpoints] = useState<TaskCheckpoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [r, t, msgs, cps] = await Promise.all([
        tasksApi.getReport(taskId),
        tasksApi.getTimeline(taskId),
        tasksApi.listMessages(taskId),
        tasksApi.listCheckpoints(taskId),
      ]);
      setReport(r);
      setTimeline(t);
      setMessages(buildTimeline(msgs));
      setCheckpoints(cps);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load task");
    } finally {
      setLoading(false);
    }
  }, [taskId]);

  useEffect(() => {
    load();
  }, [load]);

  // Refresh on task lifecycle events
  useWSEvent(
    "task:completed",
    useCallback(
      (payload: unknown) => {
        if (
          typeof payload === "object" &&
          payload !== null &&
          "task_id" in payload &&
          (payload as { task_id: string }).task_id === taskId
        ) {
          load();
        }
      },
      [taskId, load],
    ),
  );

  useWSEvent(
    "task:failed",
    useCallback(
      (payload: unknown) => {
        if (
          typeof payload === "object" &&
          payload !== null &&
          "task_id" in payload &&
          (payload as { task_id: string }).task_id === taskId
        ) {
          load();
        }
      },
      [taskId, load],
    ),
  );

  if (loading) {
    return (
      <div className="max-w-3xl mx-auto p-6 space-y-4">
        <Skeleton className="h-8 w-1/3" />
        <Skeleton className="h-4 w-1/4" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (error || !report) {
    return (
      <div className="max-w-3xl mx-auto p-6 text-center">
        <p className="text-destructive">{error ?? "Task not found"}</p>
        <Link href="/" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mt-4">
          <ArrowLeft className="h-4 w-4" />
          Back
        </Link>
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto p-6 space-y-6">
      {/* Header */}
      <div className="flex items-start gap-3">
        <Link href={`/issues/${report.issue_id}`} className="mt-1 text-muted-foreground hover:text-foreground">
          <ArrowLeft className="h-4 w-4" />
        </Link>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            {statusIcon(report.status)}
            <h1 className="text-lg font-semibold truncate">
              Task Run <span className="text-muted-foreground font-mono text-sm">#{taskId.slice(0, 8)}</span>
            </h1>
          </div>
          <div className="flex items-center gap-2 mt-1 text-sm text-muted-foreground">
            <Link href={`/issues/${report.issue_id}`} className="underline underline-offset-2 hover:text-foreground">
              {report.issue_title}
            </Link>
            <span>·</span>
            <span>{report.agent_name}</span>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="summary">
        <TabsList>
          <TabsTrigger value="summary">
            <FileText className="h-3.5 w-3.5 mr-1" />
            Summary
          </TabsTrigger>
          <TabsTrigger value="timeline">
            <Clock className="h-3.5 w-3.5 mr-1" />
            Timeline
          </TabsTrigger>
          <TabsTrigger value="messages">
            <MessageSquare className="h-3.5 w-3.5 mr-1" />
            Messages
          </TabsTrigger>
          <TabsTrigger value="checkpoints">
            <GitBranch className="h-3.5 w-3.5 mr-1" />
            Checkpoints
          </TabsTrigger>
          <TabsTrigger value="review">
            <Eye className="h-3.5 w-3.5 mr-1" />
            Review
          </TabsTrigger>
          <TabsTrigger value="output">
            <FileText className="h-3.5 w-3.5 mr-1" />
            Output
          </TabsTrigger>
          <TabsTrigger value="context">
            <Eye className="h-3.5 w-3.5 mr-1" />
            Context Preview
          </TabsTrigger>
        </TabsList>

        <TabsContent value="summary" className="mt-4">
          <SummaryTab report={report} />
        </TabsContent>
        <TabsContent value="timeline" className="mt-4">
          <TimelineTab events={timeline} />
        </TabsContent>
        <TabsContent value="messages" className="mt-4">
          <MessagesTab items={messages} />
        </TabsContent>
        <TabsContent value="checkpoints" className="mt-4">
          <CheckpointsTab checkpoints={checkpoints} />
        </TabsContent>
        <TabsContent value="review" className="mt-4">
          <ReviewTab report={report} events={timeline} />
        </TabsContent>
        <TabsContent value="output" className="mt-4">
          <OutputTab report={report} />
        </TabsContent>
        <TabsContent value="context" className="mt-4">
          <ContextPreviewTab taskId={taskId} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
