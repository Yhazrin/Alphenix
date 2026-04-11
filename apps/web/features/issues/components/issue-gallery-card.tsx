"use client";

import { memo } from "react";
import Link from "next/link";
import { CalendarDays } from "lucide-react";
import type { Issue } from "@/shared/types";
import { PRIORITY_CONFIG, STATUS_CONFIG } from "@/features/issues/config";
import { shortDate } from "@/shared/utils";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { useIssueSelectionStore } from "@/features/issues/stores/selection-store";
import { useViewStore } from "@/features/issues/stores/view-store-context";
import { cn } from "@/lib/utils";
import { PriorityIcon } from "./priority-icon";
import { StatusIcon } from "./status-icon";
import { AgentStatusDot, getAgentIssueStatus } from "./agent-status-dot";
import { PRIORITY_BORDER } from "./board-card";
import { ChannelBadge } from "@/features/channels";

function isOverdue(date: string): boolean {
  const t = new Date(date).getTime();
  return !isNaN(t) && t < Date.now();
}

/**
 * Gallery tile: large radius, status chip, glassy card — optimized for grid scanning (no columns / no accordion).
 */
export const IssueGalleryCard = memo(function IssueGalleryCard({ issue }: { issue: Issue }) {
  const selected = useIssueSelectionStore((s) => s.selectedIds.has(issue.id));
  const toggle = useIssueSelectionStore((s) => s.toggle);
  const storeProperties = useViewStore((s) => s.cardProperties);

  const statusCfg = STATUS_CONFIG[issue.status];
  const priorityCfg = PRIORITY_CONFIG[issue.priority];
  const borderClass = PRIORITY_BORDER[issue.priority] ?? "border-border/30";
  const agentStatus = issue.assignee_type === "agent" ? getAgentIssueStatus(issue) : "idle";
  const isAgentActive = agentStatus === "working" || agentStatus === "queued";

  const showPriority = storeProperties.priority;
  const showDescription = storeProperties.description && issue.description;
  const showAssignee = storeProperties.assignee && issue.assignee_type && issue.assignee_id;
  const showDueDate = storeProperties.dueDate && issue.due_date;
  const hasFooterRow = showAssignee || showPriority || showDueDate;

  return (
    <div
      className={cn(
        "group/gallery relative flex h-full min-h-[13.5rem] flex-col overflow-hidden rounded-2xl border bg-card/90 p-4 shadow-sm ring-1 ring-border/40 backdrop-blur-sm transition-all duration-200 ease-out sm:min-h-[14rem]",
        "hover:-translate-y-1 hover:shadow-md hover:ring-primary/15",
        borderClass,
        selected && "bg-primary/[0.06] ring-2 ring-primary/30",
        isAgentActive && "board-agent-active-card",
      )}
    >
      <div className="absolute left-3 top-3 z-10">
        <input
          type="checkbox"
          checked={selected}
          aria-label={`Select ${issue.identifier}`}
          onClick={(e) => e.stopPropagation()}
          onChange={() => toggle(issue.id)}
          className="size-4 cursor-pointer accent-primary"
        />
      </div>

      <div className="flex items-start justify-between gap-2 pl-8">
        <Link
          href={`/issues/${issue.id}`}
          className="min-w-0 flex-1 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <div className="flex flex-wrap items-center gap-1.5 font-mono text-[11px] tabular-nums text-muted-foreground">
            <span>{issue.identifier}</span>
            <ChannelBadge channelId={issue.channel_id} className="font-sans" />
            {issue.assignee_type === "agent" && (
              <AgentStatusDot status={getAgentIssueStatus(issue)} />
            )}
          </div>
          <h3 className="mt-1.5 line-clamp-2 text-[15px] font-semibold leading-snug tracking-tight text-foreground">
            {issue.title}
          </h3>
        </Link>
        <Link
          href={`/issues/${issue.id}`}
          className={cn(
            "shrink-0 rounded-full px-2 py-1 text-[10px] font-semibold uppercase tracking-wide",
            statusCfg.badgeBg,
            statusCfg.badgeText,
          )}
          title={statusCfg.label}
        >
          <span className="flex items-center gap-1">
            <StatusIcon status={issue.status} className="size-3" inheritColor aria-hidden="true" />
            <span className="max-w-[5rem] truncate sm:max-w-[6.5rem]">{statusCfg.label}</span>
          </span>
        </Link>
      </div>

      {/* Fixed slot for two lines of description so card heights stay aligned */}
      <div className="mt-2 min-h-[2.625rem] pl-8">
        {showDescription ? (
          <Link
            href={`/issues/${issue.id}`}
            className="block text-xs leading-relaxed text-muted-foreground line-clamp-2"
          >
            {issue.description}
          </Link>
        ) : null}
      </div>

      <div
        className={cn(
          "mt-auto flex min-h-[52px] flex-wrap content-end items-center gap-2 pl-8 pt-3",
          hasFooterRow && "border-t border-border/30",
        )}
      >
        {hasFooterRow ? (
          <>
            {showAssignee && (
              <ActorAvatar
                actorType={issue.assignee_type!}
                actorId={issue.assignee_id!}
                size={24}
              />
            )}
            {showPriority && issue.priority !== "none" && (
              <span
                className={cn(
                  "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium",
                  priorityCfg.badgeBg,
                  priorityCfg.badgeText,
                )}
              >
                <PriorityIcon priority={issue.priority} className="size-3" inheritColor />
                {priorityCfg.label}
              </span>
            )}
            {showDueDate && issue.due_date && (
              <span
                className={cn(
                  "ml-auto inline-flex items-center gap-1 text-xs",
                  isOverdue(issue.due_date) ? "text-destructive" : "text-muted-foreground",
                )}
              >
                <CalendarDays className="size-3.5 shrink-0 opacity-70" aria-hidden="true" />
                {shortDate(issue.due_date)}
              </span>
            )}
          </>
        ) : null}
      </div>
    </div>
  );
});
