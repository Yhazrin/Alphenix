"use client";

import { memo } from "react";
import Link from "next/link";
import type { Issue } from "@/shared/types";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { useIssueSelectionStore } from "@/features/issues/stores/selection-store";
import { shortDate } from "@/shared/utils";
import { PriorityIcon } from "./priority-icon";
import { PRIORITY_CONFIG } from "@/features/issues/config";
import { AgentStatusDot, getAgentIssueStatus } from "./agent-status-dot";
import { CalendarDays } from "lucide-react";

function isOverdue(date: string): boolean {
  const t = new Date(date).getTime();
  return !isNaN(t) && t < Date.now();
}

export const ListRow = memo(function ListRow({ issue }: { issue: Issue }) {
  const selected = useIssueSelectionStore((s) => s.selectedIds.has(issue.id));
  const toggle = useIssueSelectionStore((s) => s.toggle);

  return (
    <div
      className={`group/row flex h-10 items-center gap-2 border-b border-border/40 px-4 text-sm transition-colors hover:bg-accent/50 ${
        selected ? "bg-accent/30" : ""
      }`}
    >
      <div className="relative flex shrink-0 items-center justify-center w-4 h-4">
        <PriorityIcon
          priority={issue.priority}
          className={selected ? "hidden" : "group-hover/row:hidden"}
        />
        <input
          type="checkbox"
          checked={selected}
          aria-label={`Select issue ${issue.identifier}`}
          onChange={() => toggle(issue.id)}
          className={`absolute inset-0 cursor-pointer accent-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
            selected ? "opacity-100" : "opacity-0 group-hover/row:opacity-100"
          }`}
        />
      </div>
      <Link
        href={`/issues/${issue.id}`}
        className="flex flex-1 items-center gap-2 min-w-0"
      >
        <span className="w-16 shrink-0 text-xs text-muted-foreground">
          {issue.identifier}
        </span>
        <span className="min-w-0 flex-1 truncate">{issue.title}</span>
        {issue.priority !== "none" && (
          <span className={`shrink-0 inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium ${PRIORITY_CONFIG[issue.priority].badgeBg} ${PRIORITY_CONFIG[issue.priority].badgeText}`}>
            {PRIORITY_CONFIG[issue.priority].label}
          </span>
        )}
        {issue.due_date && (
          <span className={`shrink-0 flex items-center gap-1 text-xs ${isOverdue(issue.due_date) ? "text-destructive" : "text-muted-foreground"}`}>
            <CalendarDays className="size-3" aria-hidden="true" />
            {shortDate(issue.due_date)}
          </span>
        )}
        {issue.assignee_type && issue.assignee_id && (
          <span className="flex items-center gap-1">
            <ActorAvatar
              actorType={issue.assignee_type}
              actorId={issue.assignee_id}
              size={20}
            />
            {issue.assignee_type === "agent" && (
              <AgentStatusDot status={getAgentIssueStatus(issue)} />
            )}
          </span>
        )}
      </Link>
    </div>
  );
});
