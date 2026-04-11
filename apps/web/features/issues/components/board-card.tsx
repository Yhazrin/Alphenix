"use client";

import { useCallback, memo } from "react";
import Link from "next/link";
import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { toast } from "sonner";
import { shortDate } from "@/shared/utils";
import type { Issue, UpdateIssueRequest } from "@/shared/types";
import { CalendarDays } from "lucide-react";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { api } from "@/shared/api";
import { useIssueStore } from "@/features/issues/store";
import { PriorityIcon } from "./priority-icon";
import { PriorityPicker, AssigneePicker, DueDatePicker } from "./pickers";
import { PRIORITY_CONFIG } from "@/features/issues/config";
import { AgentStatusDot, getAgentIssueStatus } from "./agent-status-dot";
import { useViewStore } from "@/features/issues/stores/view-store-context";
import { ChannelBadge } from "@/features/channels";
import { cn } from "@/lib/utils";

export const PRIORITY_BORDER: Record<string, string> = {
  urgent: "border-destructive/30",
  high: "border-warning/30",
  medium: "border-warning/20",
  low: "border-info/20",
};

function isOverdue(date: string): boolean {
  const t = new Date(date).getTime();
  return !isNaN(t) && t < Date.now();
}

/** Stops event from bubbling to Link/drag handlers */
function PickerWrapper({ children }: { children: React.ReactNode }) {
  const stop = (e: React.SyntheticEvent) => {
    e.stopPropagation();
    e.preventDefault();
  };
  return (
    <div onClick={stop} onMouseDown={stop} onPointerDown={stop}>
      {children}
    </div>
  );
}

export const BoardCardContent = memo(function BoardCardContent({
  issue,
  editable = false,
}: {
  issue: Issue;
  editable?: boolean;
}) {
  const storeProperties = useViewStore((s) => s.cardProperties);
  const priorityCfg = PRIORITY_CONFIG[issue.priority];

  const handleUpdate = useCallback(
    (updates: Partial<UpdateIssueRequest>) => {
      const current = useIssueStore.getState().issues.find(i => i.id === issue.id);
      const prev = current ? { ...current } : { ...issue };
      useIssueStore.getState().updateIssue(issue.id, updates);
      api.updateIssue(issue.id, updates).catch(() => {
        useIssueStore.getState().updateIssue(issue.id, prev);
        toast.error("Failed to update issue");
      });
    },
    [issue.id],
  );

  const showPriority = storeProperties.priority;
  const showDescription = storeProperties.description && issue.description;
  const showAssignee = storeProperties.assignee && issue.assignee_type && issue.assignee_id;
  const showDueDate = storeProperties.dueDate && issue.due_date;
  const hasFooterRow = showAssignee || showPriority || showDueDate;
  const showFooterStrip = hasFooterRow || editable;

  const borderClass = PRIORITY_BORDER[issue.priority] ?? "border-border/20";
  const agentStatus = issue.assignee_type === "agent" ? getAgentIssueStatus(issue) : "idle";
  const isAgentActive = agentStatus === "working" || agentStatus === "queued";

  return (
    <div
      className={`flex min-h-[10rem] flex-col rounded-xl border bg-card p-3.5 shadow-apple transition-all group-hover:shadow-apple-hover group-hover:-translate-y-0.5 ${borderClass} ${isAgentActive ? "board-agent-active-card" : ""}`}
    >
      {/* Row 1: Identifier + agent activity indicator */}
      <div className="flex min-w-0 flex-wrap items-center gap-1.5">
        <p className="shrink-0 text-xs text-muted-foreground">{issue.identifier}</p>
        <ChannelBadge channelId={issue.channel_id} className="min-w-0 max-w-[min(100%,7.5rem)] shrink font-sans" />
        {issue.assignee_type === "agent" && (
          <AgentStatusDot status={getAgentIssueStatus(issue)} />
        )}
      </div>

      {/* Row 2: Title — two-line slot */}
      <div className="mt-1 min-h-[2.75rem]">
        <p className="text-sm font-medium leading-snug line-clamp-2">{issue.title}</p>
      </div>

      {/* Description — one-line slot */}
      <div className="mt-1 min-h-[1.125rem]">
        {showDescription ? (
          <p className="text-xs text-muted-foreground line-clamp-1">{issue.description}</p>
        ) : null}
      </div>

      {/* Row 3: Assignee, priority badge, due date */}
      <div
        className={cn(
          "mt-auto flex min-h-[44px] flex-wrap items-center gap-2 pt-3",
          showFooterStrip && "border-t border-border/30",
        )}
      >
        {showFooterStrip ? (
          <>
            {editable ? (
              <PickerWrapper>
                <AssigneePicker
                  assigneeType={issue.assignee_type}
                  assigneeId={issue.assignee_id}
                  onUpdate={handleUpdate}
                  trigger={
                    issue.assignee_type && issue.assignee_id ? (
                      <ActorAvatar
                        actorType={issue.assignee_type}
                        actorId={issue.assignee_id}
                        size={22}
                      />
                    ) : (
                      <span className="text-xs text-muted-foreground hover:text-foreground transition-colors">
                        Assign to...
                      </span>
                    )
                  }
                />
              </PickerWrapper>
            ) : showAssignee ? (
              <ActorAvatar
                actorType={issue.assignee_type!}
                actorId={issue.assignee_id!}
                size={22}
              />
            ) : null}
            {showPriority &&
              (editable ? (
                <PickerWrapper>
                  <PriorityPicker
                    priority={issue.priority}
                    onUpdate={handleUpdate}
                    trigger={
                      <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium ${priorityCfg.badgeBg} ${priorityCfg.badgeText}`}>
                        <PriorityIcon priority={issue.priority} className="h-3 w-3" inheritColor />
                        {priorityCfg.label}
                      </span>
                    }
                  />
                </PickerWrapper>
              ) : (
                <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium ${priorityCfg.badgeBg} ${priorityCfg.badgeText}`}>
                  <PriorityIcon priority={issue.priority} className="h-3 w-3" inheritColor />
                  {priorityCfg.label}
                </span>
              ))}
            {showDueDate && (
              <div className="ml-auto">
                {editable ? (
                  <PickerWrapper>
                    <DueDatePicker
                      dueDate={issue.due_date}
                      onUpdate={handleUpdate}
                      trigger={
                        <span
                          className={`flex items-center gap-1 text-xs ${
                            isOverdue(issue.due_date!)
                              ? "text-destructive"
                              : "text-muted-foreground"
                          }`}
                        >
                          <CalendarDays className="size-3" aria-hidden="true" />
                          {shortDate(issue.due_date!)}
                        </span>
                      }
                    />
                  </PickerWrapper>
                ) : (
                  <span
                    className={`flex items-center gap-1 text-xs ${
                      isOverdue(issue.due_date!)
                        ? "text-destructive"
                        : "text-muted-foreground"
                    }`}
                  >
                    <CalendarDays className="size-3" aria-hidden="true" />
                    {shortDate(issue.due_date!)}
                  </span>
                )}
              </div>
            )}
          </>
        ) : null}
      </div>
    </div>
  );
});

export const DraggableBoardCard = memo(function DraggableBoardCard({ issue }: { issue: Issue }) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({
    id: issue.id,
    data: { status: issue.status },
  });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      className={isDragging ? "opacity-30" : ""}
    >
      <Link
        href={`/issues/${issue.id}`}
        className={`group block rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${isDragging ? "pointer-events-none" : ""}`}
      >
        <BoardCardContent issue={issue} editable />
      </Link>
    </div>
  );
});
