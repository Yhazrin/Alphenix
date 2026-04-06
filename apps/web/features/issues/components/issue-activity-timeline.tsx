"use client";

import { useMemo } from "react";
import { Calendar, AlertCircle, MessageSquare } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { ActorAvatar } from "@/components/common/actor-avatar";
import type { TimelineEntry, IssueStatus, IssuePriority } from "@/shared/types";
import { STATUS_CONFIG, PRIORITY_CONFIG } from "@/features/issues/config";
import { StatusIcon, PriorityIcon } from "@/features/issues/components";
import { CommentCard } from "./comment-card";
import { CommentInput } from "./comment-input";
import { timeAgo } from "@/shared/utils";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function statusLabel(status: string): string {
  return STATUS_CONFIG[status as IssueStatus]?.label ?? status;
}

function priorityLabel(priority: string): string {
  return PRIORITY_CONFIG[priority as IssuePriority]?.label ?? priority;
}

export function formatActivity(
  entry: TimelineEntry,
  resolveActorName?: (type: string, id: string) => string,
): string {
  const details = (entry.details ?? {}) as Record<string, string>;
  switch (entry.action) {
    case "created":
      return "created this issue";
    case "status_changed":
      return `changed status from ${statusLabel(details.from ?? "?")} to ${statusLabel(details.to ?? "?")}`;
    case "priority_changed":
      return `changed priority from ${priorityLabel(details.from ?? "?")} to ${priorityLabel(details.to ?? "?")}`;
    case "assignee_changed": {
      const isSelfAssign = details.to_type === entry.actor_type && details.to_id === entry.actor_id;
      if (isSelfAssign) return "self-assigned this issue";
      const toName = details.to_id && details.to_type && resolveActorName
        ? resolveActorName(details.to_type, details.to_id)
        : null;
      if (toName) return `assigned to ${toName}`;
      if (details.from_id && !details.to_id) return "removed assignee";
      return "changed assignee";
    }
    case "due_date_changed": {
      if (!details.to) return "removed due date";
      const d = new Date(details.to);
      if (isNaN(d.getTime())) return "set due date";
      const formatted = d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
      return `set due date to ${formatted}`;
    }
    case "title_changed":
      return `renamed this issue from "${details.from ?? "?"}" to "${details.to ?? "?"}"`;
    case "description_updated":
      return "updated the description";
    case "task_completed":
      return "completed the task";
    case "task_failed":
      return "task failed";
    default:
      return entry.action ?? "";
  }
}

// ---------------------------------------------------------------------------
// Coalescing and grouping
// ---------------------------------------------------------------------------

function useGroupedTimeline(timeline: TimelineEntry[]) {
  return useMemo(() => {
    const topLevel = timeline.filter((e) => e.type === "activity" || !e.parent_id);
    const repliesMap = new Map<string, TimelineEntry[]>();
    for (const e of timeline) {
      if (e.type === "comment" && e.parent_id) {
        const list = repliesMap.get(e.parent_id) ?? [];
        list.push(e);
        repliesMap.set(e.parent_id, list);
      }
    }

    // Coalesce: same actor + same action within 2 min -> keep last only
    const COALESCE_MS = 2 * 60 * 1000;
    const coalesced: TimelineEntry[] = [];
    for (const entry of topLevel) {
      if (entry.type === "activity") {
        const prev = coalesced[coalesced.length - 1];
        if (
          prev?.type === "activity" &&
          prev.action === entry.action &&
          prev.actor_type === entry.actor_type &&
          prev.actor_id === entry.actor_id &&
          Math.abs(new Date(entry.created_at).getTime() - new Date(prev.created_at).getTime()) <= COALESCE_MS
        ) {
          coalesced[coalesced.length - 1] = entry;
          continue;
        }
      }
      coalesced.push(entry);
    }

    // Group consecutive activities together so the connector line works
    const grouped: { type: "activities" | "comment"; entries: TimelineEntry[] }[] = [];
    for (const entry of coalesced) {
      if (entry.type === "activity") {
        const last = grouped[grouped.length - 1];
        if (last?.type === "activities") {
          last.entries.push(entry);
        } else {
          grouped.push({ type: "activities", entries: [entry] });
        }
      } else {
        grouped.push({ type: "comment", entries: [entry] });
      }
    }

    return { repliesByParent: repliesMap, groups: grouped };
  }, [timeline]);
}

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface IssueActivityTimelineProps {
  issueId: string;
  timeline: TimelineEntry[];
  loading: boolean;
  error?: string | null;
  currentUserId: string | undefined;
  getActorName: (type: string, id: string) => string;
  submitReply: (parentId: string, content: string, attachmentIds?: string[]) => Promise<void>;
  editComment: (commentId: string, content: string) => Promise<void>;
  deleteComment: (commentId: string) => void;
  handleToggleReaction: (commentId: string, emoji: string) => void;
  submitComment: (content: string, attachmentIds?: string[]) => Promise<void>;
  highlightedId: string | null;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function IssueActivityTimeline({
  issueId,
  timeline,
  loading,
  error,
  currentUserId,
  getActorName,
  submitReply,
  editComment,
  deleteComment,
  handleToggleReaction,
  submitComment,
  highlightedId,
}: IssueActivityTimelineProps) {
  const { repliesByParent, groups } = useGroupedTimeline(timeline);

  if (loading) {
    return (
      <div className="space-y-4">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="flex items-start gap-3 px-4">
            <Skeleton className="h-8 w-8 rounded-full shrink-0" />
            <div className="flex-1 space-y-2">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-16 w-full rounded-lg" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center gap-2 py-8 text-sm text-muted-foreground">
        <AlertCircle className="h-5 w-5 text-destructive" aria-hidden="true" />
        <span>Failed to load activity</span>
        <span className="text-xs text-destructive">{error}</span>
      </div>
    );
  }

  if (groups.length === 0) {
    return (
      <div className="flex flex-col items-center gap-2 py-8 text-sm text-muted-foreground">
        <MessageSquare className="h-5 w-5" aria-hidden="true" />
        <span>No activity yet</span>
        <div className="mt-4 w-full">
          <CommentInput issueId={issueId} onSubmit={submitComment} />
        </div>
      </div>
    );
  }

  return (
    <div className="mt-4 flex flex-col gap-3">
      {groups.map((group) => {
        if (group.type === "comment") {
          const entry = group.entries[0]!;
          return (
            <div key={entry.id} id={`comment-${entry.id}`}>
              <CommentCard
                issueId={issueId}
                entry={entry}
                allReplies={repliesByParent}
                currentUserId={currentUserId}
                onReply={submitReply}
                onEdit={editComment}
                onDelete={deleteComment}
                onToggleReaction={handleToggleReaction}
                highlightedCommentId={highlightedId}
              />
            </div>
          );
        }

        return (
          <div key={group.entries[0]!.id} className="px-4 flex flex-col gap-3">
            {group.entries.map((entry) => {
              const details = (entry.details ?? {}) as Record<string, string>;
              const isStatusChange = entry.action === "status_changed";
              const isPriorityChange = entry.action === "priority_changed";
              const isDueDateChange = entry.action === "due_date_changed";

              let leadIcon: React.ReactNode;
              if (isStatusChange && details.to) {
                leadIcon = <StatusIcon status={details.to as IssueStatus} className="h-4 w-4 shrink-0" />;
              } else if (isPriorityChange && details.to) {
                leadIcon = <PriorityIcon priority={details.to as IssuePriority} className="h-4 w-4 shrink-0" />;
              } else if (isDueDateChange) {
                leadIcon = <Calendar className="h-4 w-4 shrink-0 text-muted-foreground" />;
              } else {
                leadIcon = <ActorAvatar actorType={entry.actor_type} actorId={entry.actor_id} size={16} />;
              }

              return (
                <div key={entry.id} className="flex items-center text-xs text-muted-foreground">
                  <div className="mr-2 flex w-4 shrink-0 justify-center">
                    {leadIcon}
                  </div>
                  <div className="flex min-w-0 flex-1 items-center gap-1">
                    <span className="shrink-0 font-medium">{getActorName(entry.actor_type, entry.actor_id)}</span>
                    <span className="truncate">{formatActivity(entry, getActorName)}</span>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <span className="ml-auto shrink-0 cursor-default">
                            {timeAgo(entry.created_at)}
                          </span>
                        }
                      />
                      <TooltipContent side="top">
                        {new Date(entry.created_at).toLocaleString()}
                      </TooltipContent>
                    </Tooltip>
                  </div>
                </div>
              );
            })}
          </div>
        );
      })}

      {/* Bottom comment input */}
      <div className="mt-4">
        <CommentInput issueId={issueId} onSubmit={submitComment} />
      </div>
    </div>
  );
}
