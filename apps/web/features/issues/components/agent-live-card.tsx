"use client";

import { useState, useEffect, useCallback, useRef, useMemo } from "react";
import Link from "next/link";
import { toast } from "sonner";
import { AlertCircle, Bot, ChevronUp, Loader2, ArrowDown, Square, XCircle, X, RotateCcw, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { useActorName } from "@/features/workspace";
import { formatElapsed } from "./timeline-helpers";
import { TimelineRow } from "./timeline-row";
import { useLiveTask } from "../hooks/use-live-task";
import { api } from "@/shared/api";

// Re-export TaskRunHistory so existing imports from this module keep working.
export { TaskRunHistory } from "./task-run-history";

// ─── AgentLiveCard (real-time view) ────────────────────────────────────────

interface AgentLiveCardProps {
  issueId: string;
  agentName?: string;
  /** Scroll container ref — needed for sticky sentinel detection. */
  scrollContainerRef?: React.RefObject<HTMLDivElement | null>;
}

export function AgentLiveCard({ issueId, agentName, scrollContainerRef }: AgentLiveCardProps) {
  const { getActorName } = useActorName();
  const { activeTask, items, progress, cancelling, error, lastError, clearError, handleCancel } = useLiveTask(issueId);
  const [elapsed, setElapsed] = useState("");
  const [autoScroll, setAutoScroll] = useState(true);
  const [isStuck, setIsStuck] = useState(false);
  const [errorExpanded, setErrorExpanded] = useState(true);
  const [retrying, setRetrying] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);

  // Elapsed time
  useEffect(() => {
    if (!activeTask?.started_at && !activeTask?.dispatched_at) return;
    const startRef = activeTask.started_at ?? activeTask.dispatched_at!;
    setElapsed(formatElapsed(startRef));
    const interval = setInterval(() => setElapsed(formatElapsed(startRef)), 1000);
    return () => clearInterval(interval);
  }, [activeTask?.started_at, activeTask?.dispatched_at]);

  // Sentinel pattern: detect when the card is scrolled past and becomes "stuck"
  useEffect(() => {
    const sentinel = sentinelRef.current;
    const root = scrollContainerRef?.current;
    if (!sentinel || !root || !activeTask) {
      setIsStuck(false);
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        const first = entries[0];
        if (first) setIsStuck(!first.isIntersecting);
      },
      { root, threshold: 0, rootMargin: "-40px 0px 0px 0px" },
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [scrollContainerRef, activeTask]);

  const scrollToCard = useCallback(() => {
    sentinelRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
  }, []);

  // Auto-collapse error banner after 30s
  useEffect(() => {
    if (!lastError) return;
    setErrorExpanded(true);
    const timer = setTimeout(() => setErrorExpanded(false), 30000);
    return () => clearTimeout(timer);
  }, [lastError?.taskId, lastError?.timestamp]);

  // Auto-scroll
  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [items, autoScroll]);

  const handleScroll = useCallback(() => {
    if (!scrollRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = scrollRef.current;
    setAutoScroll(scrollHeight - scrollTop - clientHeight < 40);
  }, []);

  const toolCount = useMemo(
    () => items.filter((i) => i.type === "tool_use").length,
    [items]
  );

  if (error && !activeTask) {
    return (
      <div className="mt-4 flex items-center gap-2 rounded-lg border border-dashed px-3 py-2 text-xs text-destructive">
        <AlertCircle className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        {error}
      </div>
    );
  }

  // Error banner — persistent state after task failure
  if (lastError && !activeTask) {
    if (!errorExpanded) {
      return (
        <div
          className="mt-4 flex items-center gap-2 rounded-lg border border-destructive/20 bg-destructive/5 px-3 py-1.5 text-xs cursor-pointer hover:bg-destructive/10 transition-colors"
          onClick={() => setErrorExpanded(true)}
          onMouseEnter={() => setErrorExpanded(true)}
          role="status"
          data-testid="task-error-banner"
        >
          <XCircle className="h-3 w-3 shrink-0 text-destructive" aria-hidden="true" />
          <span className="text-destructive/80 truncate flex-1">
            Task failed — {lastError.message.length > 60 ? lastError.message.slice(0, 60) + "..." : lastError.message}
          </span>
          <button
            onClick={(e) => { e.stopPropagation(); clearError(); }}
            className="shrink-0 text-muted-foreground hover:text-foreground transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded"
            aria-label="Dismiss"
          >
            <X className="h-3 w-3" />
          </button>
        </div>
      );
    }

    return (
      <div className="mt-4 rounded-lg border border-destructive/30 bg-destructive/5 p-3" data-testid="task-error-banner">
        <div className="flex items-start gap-2">
          <XCircle className="h-4 w-4 shrink-0 text-destructive mt-0.5" aria-hidden="true" />
          <div className="flex-1 min-w-0">
            <p className="text-xs font-medium text-destructive">Task failed</p>
            <p className="text-xs text-destructive/80 mt-0.5 line-clamp-2">
              {lastError.message}
            </p>
          </div>
          <button
            className="shrink-0 text-muted-foreground hover:text-foreground transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded p-0.5"
            onClick={clearError}
            aria-label="Dismiss error"
          >
            <X className="h-3 w-3" />
          </button>
        </div>
        <div className="flex items-center gap-2 mt-2.5 ml-6">
          <button
            className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs font-medium hover:bg-accent hover:text-accent-foreground transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
            disabled={retrying}
            onClick={async () => {
              if (!lastError) return;
              setRetrying(true);
              try {
                await api.retryTask(issueId, lastError.taskId);
                clearError();
                toast.success("Task queued for retry");
              } catch (err) {
                toast.error("Failed to retry task", {
                  description: err instanceof Error ? err.message : "Unknown error",
                });
              } finally {
                setRetrying(false);
              }
            }}
          >
            {retrying ? (
              <Loader2 className="h-3 w-3 animate-spin" aria-hidden="true" />
            ) : (
              <RotateCcw className="h-3 w-3" aria-hidden="true" />
            )}
            {retrying ? "Retrying..." : "Retry"}
          </button>
          <Link
            href={`?task=${lastError.taskId}`}
            className="inline-flex items-center gap-0.5 text-xs text-muted-foreground hover:text-foreground transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded px-1 py-0.5"
          >
            View details
            <ChevronRight className="h-3 w-3" aria-hidden="true" />
          </Link>
        </div>
      </div>
    );
  }

  if (!activeTask) return null;
  const name = (activeTask.agent_id ? getActorName("agent", activeTask.agent_id) : agentName) ?? "Agent";

  return (
    <>
      {/* Sentinel — zero-height element that IntersectionObserver watches */}
      <div ref={sentinelRef} className="mt-4 h-0 pointer-events-none" aria-hidden />

      <div
        className={cn(
          "rounded-lg border transition-all duration-200",
          isStuck
            ? "sticky top-4 z-10 shadow-md border-brand/30 bg-brand/10 backdrop-blur-md"
            : "border-info/20 bg-info/5",
        )}
      >
        {/* Header */}
        <div className="flex items-center gap-2 px-3 py-2">
          {activeTask.agent_id ? (
            <ActorAvatar actorType="agent" actorId={activeTask.agent_id} size={20} />
          ) : (
            <div className={cn(
              "flex items-center justify-center h-5 w-5 rounded-full shrink-0",
              isStuck ? "bg-brand/15 text-brand" : "bg-info/10 text-info",
            )}>
              <Bot className="h-3 w-3" aria-hidden="true" />
            </div>
          )}
          <div className="flex items-center gap-1.5 text-xs font-medium min-w-0">
            <Loader2 className={cn("h-3 w-3 animate-spin shrink-0", isStuck ? "text-brand" : "text-info")} />
            <span className="truncate">{name} is working</span>
          </div>
          <span className="ml-auto text-xs text-muted-foreground tabular-nums shrink-0">{elapsed}</span>
          {!isStuck && toolCount > 0 && (
            <span className="text-xs text-muted-foreground shrink-0">
              {toolCount} tool {toolCount === 1 ? "call" : "calls"}
            </span>
          )}
          {isStuck ? (
            <button
              onClick={scrollToCard}
              className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-muted-foreground hover:text-foreground transition-colors shrink-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              aria-label="Scroll to live card"
              title="Scroll to live card"
            >
              <ChevronUp className="h-3.5 w-3.5" aria-hidden="true" />
            </button>
          ) : (
            <button
              onClick={handleCancel}
              disabled={cancelling}
              className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-50 shrink-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              aria-label="Stop agent"
              title="Stop agent"
            >
              {cancelling ? (
                <Loader2 className="h-3 w-3 animate-spin" aria-hidden="true" />
              ) : (
                <Square className="h-3 w-3" aria-hidden="true" />
              )}
              <span>Stop</span>
            </button>
          )}
        </div>

        {/* Progress bar */}
        {progress && progress.total > 0 && (
          <div className="px-3 pb-2">
            <div className="flex items-center gap-2 text-xs text-muted-foreground mb-1">
              <span className="truncate">{progress.summary}</span>
              <span className="shrink-0 tabular-nums">{progress.step}/{progress.total}</span>
            </div>
            <div className="h-1 rounded-full bg-muted overflow-hidden">
              <div
                className={cn(
                  "h-full rounded-full transition-all duration-500",
                  isStuck ? "bg-brand" : "bg-info",
                )}
                style={{ width: `${Math.min(100, (progress.step / progress.total) * 100)}%` }}
              />
            </div>
          </div>
        )}

        {/* Timeline content — collapses when stuck */}
        <div
          className={cn(
            "overflow-hidden transition-all duration-200",
            isStuck ? "max-h-0 opacity-0" : "max-h-[20rem] opacity-100",
          )}
        >
          {items.length > 0 && (
            <div
              ref={scrollRef}
              onScroll={handleScroll}
              className="relative max-h-80 overflow-y-auto border-t border-info/10 px-3 py-2 space-y-0.5"
            >
              {items.map((item, idx) => (
                <TimelineRow key={`${item.seq}-${idx}`} item={item} />
              ))}

              {!autoScroll && (
                <button
                  onClick={() => {
                    if (scrollRef.current) {
                      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
                      setAutoScroll(true);
                    }
                  }}
                  aria-label="Scroll to latest"
                  className="sticky bottom-0 left-1/2 -translate-x-1/2 flex items-center gap-1 rounded-full bg-background border px-2 py-0.5 text-xs text-muted-foreground hover:text-foreground shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  <ArrowDown className="h-3 w-3" aria-hidden="true" />
                  Latest
                </button>
              )}
            </div>
          )}
        </div>
      </div>
    </>
  );
}
