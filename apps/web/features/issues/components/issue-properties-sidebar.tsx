"use client";

import { useState } from "react";
import { Check, ChevronRight } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { ActorAvatar } from "@/components/common/actor-avatar";
import type { UpdateIssueRequest } from "@/shared/types";
import { ALL_STATUSES, STATUS_CONFIG, PRIORITY_ORDER, PRIORITY_CONFIG } from "@/features/issues/config";
import { StatusIcon, PriorityIcon, AssigneePicker, DueDatePicker } from "@/features/issues/components";
import type { Issue } from "@/shared/types";
import { shortDate } from "@/shared/utils";

// ---------------------------------------------------------------------------
// Property row
// ---------------------------------------------------------------------------

function PropRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex min-h-8 items-center gap-2 rounded-md px-2 -mx-2 hover:bg-accent/50 transition-colors">
      <span className="w-16 shrink-0 text-xs text-muted-foreground">{label}</span>
      <div className="flex min-w-0 flex-1 items-center gap-1.5 text-xs truncate">
        {children}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface IssuePropertiesSidebarProps {
  issue: Issue;
  getActorName: (type: string, id: string) => string;
  onUpdateField: (updates: Partial<UpdateIssueRequest>) => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function IssuePropertiesSidebar({
  issue,
  getActorName,
  onUpdateField,
}: IssuePropertiesSidebarProps) {
  const [propertiesOpen, setPropertiesOpen] = useState(true);
  const [detailsOpen, setDetailsOpen] = useState(true);

  return (
    <div className="overflow-y-auto border-l h-full">
      <div className="p-4 space-y-5">
        {/* Properties section */}
        <div>
          <button
            className={`flex w-full items-center gap-1 text-xs font-medium transition-colors mb-2 ${propertiesOpen ? "" : "text-muted-foreground hover:text-foreground"}`}
            onClick={() => setPropertiesOpen(!propertiesOpen)}
          >
            <ChevronRight className={`h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform ${propertiesOpen ? "rotate-90" : ""}`} />
            Properties
          </button>

          {propertiesOpen && <div className="space-y-0.5 pl-2">
            {/* Status */}
            <PropRow label="Status">
              <DropdownMenu>
                <DropdownMenuTrigger className="flex items-center gap-1.5 cursor-pointer rounded px-1 -mx-1 hover:bg-accent/30 transition-colors overflow-hidden">
                  <StatusIcon status={issue.status} className="h-3.5 w-3.5 shrink-0" />
                  <span className="truncate">{STATUS_CONFIG[issue.status].label}</span>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start" className="w-44">
                  {ALL_STATUSES.map((s) => (
                    <DropdownMenuItem key={s} onClick={() => onUpdateField({ status: s })}>
                      <StatusIcon status={s} className="h-3.5 w-3.5" />
                      {STATUS_CONFIG[s].label}
                      {s === issue.status && <Check className="ml-auto h-3.5 w-3.5" />}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            </PropRow>

            {/* Priority */}
            <PropRow label="Priority">
              <DropdownMenu>
                <DropdownMenuTrigger className="flex items-center gap-1.5 cursor-pointer rounded px-1 -mx-1 hover:bg-accent/30 transition-colors overflow-hidden">
                  <PriorityIcon priority={issue.priority} className="shrink-0" />
                  <span className="truncate">{PRIORITY_CONFIG[issue.priority].label}</span>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start" className="w-44">
                  {PRIORITY_ORDER.map((p) => (
                    <DropdownMenuItem key={p} onClick={() => onUpdateField({ priority: p })}>
                      <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium ${PRIORITY_CONFIG[p].badgeBg} ${PRIORITY_CONFIG[p].badgeText}`}>
                        <PriorityIcon priority={p} className="h-3 w-3" inheritColor />
                        {PRIORITY_CONFIG[p].label}
                      </span>
                      {p === issue.priority && <Check className="ml-auto h-3.5 w-3.5" />}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            </PropRow>

            {/* Assignee */}
            <PropRow label="Assignee">
              <AssigneePicker
                assigneeType={issue.assignee_type}
                assigneeId={issue.assignee_id}
                onUpdate={onUpdateField}
                align="start"
              />
            </PropRow>

            {/* Due date */}
            <PropRow label="Due date">
              <DueDatePicker
                dueDate={issue.due_date}
                onUpdate={onUpdateField}
              />
            </PropRow>
          </div>}
        </div>

        {/* Details section */}
        <div>
          <button
            className={`flex w-full items-center gap-1 text-xs font-medium transition-colors mb-2 ${detailsOpen ? "" : "text-muted-foreground hover:text-foreground"}`}
            onClick={() => setDetailsOpen(!detailsOpen)}
          >
            <ChevronRight className={`h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform ${detailsOpen ? "rotate-90" : ""}`} />
            Details
          </button>

          {detailsOpen && <div className="space-y-0.5 pl-2">
            <PropRow label="Created by">
              <ActorAvatar
                actorType={issue.creator_type}
                actorId={issue.creator_id}
                size={18}
              />
              <span className="truncate">{getActorName(issue.creator_type, issue.creator_id)}</span>
            </PropRow>
            <PropRow label="Created">
              <span className="text-muted-foreground">{shortDate(issue.created_at)}</span>
            </PropRow>
            <PropRow label="Updated">
              <span className="text-muted-foreground">{shortDate(issue.updated_at)}</span>
            </PropRow>
          </div>}
        </div>

      </div>
    </div>
  );
}
