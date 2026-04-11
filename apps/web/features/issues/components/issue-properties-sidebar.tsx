"use client";

import Link from "next/link";
import { Check, ChevronRight, Copy, Link2, Users } from "lucide-react";
import { toast } from "sonner";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover";
import { Checkbox } from "@/components/ui/checkbox";
import { Command, CommandInput, CommandList, CommandEmpty, CommandGroup, CommandItem } from "@/components/ui/command";
import { AvatarGroup, AvatarGroupCount } from "@/components/ui/avatar";
import type { UpdateIssueRequest, Issue, IssueSubscriber, MemberWithUser, Agent } from "@/shared/types";
import { ALL_STATUSES, STATUS_CONFIG, PRIORITY_ORDER, PRIORITY_CONFIG } from "@/features/issues/config";
import { StatusIcon } from "./status-icon";
import { PriorityIcon } from "./priority-icon";
import { AssigneePicker, DueDatePicker, RepoPicker } from "./pickers";
import { shortDate } from "@/shared/utils";
import { cn } from "@/lib/utils";
import { getAgentIssueStatus, AgentStatusDot } from "./agent-status-dot";
import { ChannelPicker } from "@/features/channels";

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface IssuePropertiesSidebarProps {
  issue: Issue;
  getActorName: (type: string, id: string) => string;
  onUpdateField: (updates: Partial<UpdateIssueRequest>) => void;
  subscribers: IssueSubscriber[];
  subscribersLoading: boolean;
  isSubscribed: boolean;
  onToggleSubscribe: () => void;
  toggleSubscriber: (userId: string, userType: "member" | "agent", isSubbed: boolean) => void;
  members: MemberWithUser[];
  agents: Agent[];
  onScrollToAgentSection?: () => void;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function agentTaskHint(issue: Issue): string | null {
  const s = issue.latest_task_status;
  if (s === "failed") return "Last agent run failed";
  if (s === "queued") return "Agent task queued";
  if (s === "running" || s === "claimed" || s === "dispatched") return "Agent task in progress";
  if (issue.assignee_type === "agent" && issue.assignee_id) return "Assigned to agent";
  return null;
}

function copyToClipboard(text: string, success: string) {
  void navigator.clipboard.writeText(text).then(
    () => toast.success(success),
    () => toast.error("Could not copy"),
  );
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function IssuePropertiesSidebar({
  issue,
  getActorName,
  onUpdateField,
  subscribers,
  subscribersLoading,
  isSubscribed,
  onToggleSubscribe,
  toggleSubscriber,
  members,
  agents,
  onScrollToAgentSection,
}: IssuePropertiesSidebarProps) {
  const agentStatus = getAgentIssueStatus(issue);
  const taskHint = agentTaskHint(issue);
  const showAgentStrip =
    issue.assignee_type === "agent" || agentStatus !== "idle" || issue.latest_task_status;

  return (
    <div
      data-testid="issue-detail-sidebar"
      className="flex h-full min-h-0 flex-col border-l border-border/80 bg-muted/10"
    >
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="space-y-4 p-4">
          {/* Summary — primary fields at a glance */}
          <div className="rounded-xl border border-border/70 bg-card p-3 shadow-sm">
            <div className="flex items-start justify-between gap-2">
              <span className="font-mono text-[11px] font-medium text-muted-foreground tabular-nums">
                {issue.identifier}
              </span>
              <div className="flex flex-wrap justify-end gap-1">
                <DropdownMenu>
                  <DropdownMenuTrigger
                    className="inline-flex items-center gap-1 rounded-md border border-border/80 bg-background px-2 py-0.5 text-[11px] font-medium hover:bg-accent/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    <StatusIcon status={issue.status} className="h-3 w-3 shrink-0" />
                    {STATUS_CONFIG[issue.status].label}
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-44">
                    {ALL_STATUSES.map((s) => (
                      <DropdownMenuItem key={s} onClick={() => onUpdateField({ status: s })}>
                        <StatusIcon status={s} className="h-3.5 w-3.5" aria-hidden="true" />
                        {STATUS_CONFIG[s].label}
                        {s === issue.status && <Check className="ml-auto h-3.5 w-3.5" aria-hidden="true" />}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
                <DropdownMenu>
                  <DropdownMenuTrigger
                    className="inline-flex items-center gap-1 rounded-md border border-border/80 bg-background px-2 py-0.5 text-[11px] font-medium hover:bg-accent/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    <PriorityIcon priority={issue.priority} className="h-3 w-3 shrink-0" />
                    {PRIORITY_CONFIG[issue.priority].label}
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-44">
                    {PRIORITY_ORDER.map((p) => (
                      <DropdownMenuItem key={p} onClick={() => onUpdateField({ priority: p })}>
                        <span
                          className={cn(
                            "inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium",
                            PRIORITY_CONFIG[p].badgeBg,
                            PRIORITY_CONFIG[p].badgeText,
                          )}
                        >
                          <PriorityIcon priority={p} className="h-3 w-3" inheritColor />
                          {PRIORITY_CONFIG[p].label}
                        </span>
                        {p === issue.priority && <Check className="ml-auto h-3.5 w-3.5" aria-hidden="true" />}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>

            <div className="mt-3 border-t border-border/50 pt-3">
              <p className="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
                Assignee
              </p>
              <AssigneePicker
                assigneeType={issue.assignee_type}
                assigneeId={issue.assignee_id}
                onUpdate={onUpdateField}
                align="end"
              />
            </div>

            <div className="mt-3 border-t border-border/50 pt-3">
              <p className="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
                Due date
              </p>
              <DueDatePicker dueDate={issue.due_date} onUpdate={onUpdateField} />
            </div>

            <div className="mt-3 border-t border-border/50 pt-3">
              <p className="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
                Channel
              </p>
              <ChannelPicker
                channelId={issue.channel_id}
                onSelect={(nextId) => onUpdateField({ channel_id: nextId })}
              />
              <Link
                href={`/channels/${issue.channel_id}`}
                className="mt-2 inline-block text-xs text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
              >
                Manage channel access
              </Link>
            </div>
          </div>

          {/* Agent / task context — ties sidebar to execution, not only static metadata */}
          {showAgentStrip && (
            <div className="rounded-xl border border-border/60 bg-card/80 p-3">
              <div className="flex items-center gap-2">
                <AgentStatusDot status={agentStatus} />
                <span className="min-w-0 flex-1 text-xs font-medium leading-snug text-foreground">
                  {taskHint ?? "Agent"}
                </span>
              </div>
              {onScrollToAgentSection && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="mt-2 h-8 w-full justify-between px-2 text-xs text-muted-foreground hover:text-foreground"
                  onClick={onScrollToAgentSection}
                >
                  Open live output & collaboration
                  <ChevronRight className="h-3.5 w-3.5 shrink-0 opacity-60" aria-hidden="true" />
                </Button>
              )}
            </div>
          )}

          {/* Quick actions */}
          <div className="flex gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-8 flex-1 gap-1.5 text-xs"
              onClick={() => {
                if (typeof window === "undefined") return;
                copyToClipboard(window.location.href, "Issue link copied");
              }}
            >
              <Link2 className="h-3.5 w-3.5" aria-hidden="true" />
              Copy link
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-8 flex-1 gap-1.5 text-xs"
              onClick={() => copyToClipboard(issue.identifier, "ID copied")}
            >
              <Copy className="h-3.5 w-3.5" aria-hidden="true" />
              Copy ID
            </Button>
          </div>

          {/* Subscribers — moved from main column for a denser activity stream */}
          <div className="rounded-xl border border-border/60 bg-card/50 p-3">
            <div className="mb-2 flex items-center justify-between gap-2">
              <span className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
                Subscribers
              </span>
              <Badge variant="secondary" className="h-5 px-1.5 text-[10px] font-normal tabular-nums">
                {subscribers.length}
              </Badge>
            </div>
            {subscribersLoading ? (
              <div className="flex items-center gap-2">
                <Skeleton className="h-8 w-8 rounded-full" />
                <Skeleton className="h-8 w-8 rounded-full" />
              </div>
            ) : (
              <div className="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={onToggleSubscribe}
                  className="text-xs text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
                >
                  {isSubscribed ? "Unsubscribe me" : "Subscribe me"}
                </button>
                <Popover>
                  <PopoverTrigger
                    className="flex cursor-pointer items-center rounded-md p-0.5 hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    aria-label="Manage subscribers"
                  >
                    {subscribers.length > 0 ? (
                      <AvatarGroup>
                        {subscribers.slice(0, 6).map((sub) => (
                          <ActorAvatar
                            key={`${sub.user_type}-${sub.user_id}`}
                            actorType={sub.user_type}
                            actorId={sub.user_id}
                            size={26}
                          />
                        ))}
                        {subscribers.length > 6 && (
                          <AvatarGroupCount>+{subscribers.length - 6}</AvatarGroupCount>
                        )}
                      </AvatarGroup>
                    ) : (
                      <span className="flex h-8 w-8 items-center justify-center rounded-full border border-dashed border-muted-foreground/35 text-muted-foreground">
                        <Users className="h-3.5 w-3.5" aria-hidden="true" />
                      </span>
                    )}
                  </PopoverTrigger>
                  <PopoverContent align="start" className="w-64 p-0">
                    <Command>
                      <CommandInput placeholder="Add or remove…" />
                      <CommandList className="max-h-64">
                        <CommandEmpty>No results</CommandEmpty>
                        {members.length > 0 && (
                          <CommandGroup heading="Members">
                            {members
                              .filter((m, i, arr) => arr.findIndex((x) => x.user_id === m.user_id) === i)
                              .map((m) => {
                                const sub = subscribers.find(
                                  (s) => s.user_type === "member" && s.user_id === m.user_id,
                                );
                                const isSubbed = !!sub;
                                return (
                                  <CommandItem
                                    key={`member-${m.user_id}`}
                                    onSelect={() => toggleSubscriber(m.user_id, "member", isSubbed)}
                                    className="flex items-center gap-2.5"
                                  >
                                    <Checkbox checked={isSubbed} className="pointer-events-none" />
                                    <ActorAvatar actorType="member" actorId={m.user_id} size={22} />
                                    <span className="flex-1 truncate">{m.name}</span>
                                  </CommandItem>
                                );
                              })}
                          </CommandGroup>
                        )}
                        {(() => {
                          const activeAgents = agents.filter((a) => !a.archived_at);
                          return activeAgents.length > 0 ? (
                            <CommandGroup heading="Agents">
                              {activeAgents.map((a) => {
                                const sub = subscribers.find(
                                  (s) => s.user_type === "agent" && s.user_id === a.id,
                                );
                                const isSubbed = !!sub;
                                return (
                                  <CommandItem
                                    key={`agent-${a.id}`}
                                    onSelect={() => toggleSubscriber(a.id, "agent", isSubbed)}
                                    className="flex items-center gap-2.5"
                                  >
                                    <Checkbox checked={isSubbed} className="pointer-events-none" />
                                    <ActorAvatar actorType="agent" actorId={a.id} size={22} />
                                    <span className="flex-1 truncate">{a.name}</span>
                                  </CommandItem>
                                );
                              })}
                            </CommandGroup>
                          ) : null;
                        })()}
                      </CommandList>
                    </Command>
                  </PopoverContent>
                </Popover>
              </div>
            )}
          </div>

          {/* Repository */}
          <div className="rounded-xl border border-border/60 bg-card/30 p-3">
            <p className="mb-2 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
              Repository
            </p>
            <RepoPicker repoId={issue.repo_id} onUpdate={onUpdateField} align="end" />
          </div>

          {/* Audit — compact, always visible (no accordion) */}
          <div className="space-y-2 border-t border-border/50 pt-3 text-[11px] text-muted-foreground">
            <div className="flex items-center gap-2">
              <ActorAvatar
                actorType={issue.creator_type}
                actorId={issue.creator_id}
                size={18}
              />
              <span className="min-w-0 truncate">
                <span className="text-muted-foreground/80">Created by </span>
                <span className="text-foreground/90">{getActorName(issue.creator_type, issue.creator_id)}</span>
              </span>
            </div>
            <div className="flex justify-between gap-2 tabular-nums">
              <span>Created</span>
              <span>{shortDate(issue.created_at)}</span>
            </div>
            <div className="flex justify-between gap-2 tabular-nums">
              <span>Updated</span>
              <span>{shortDate(issue.updated_at)}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
