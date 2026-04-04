"use client";

import { memo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Calendar,
  ChevronLeft,
  ChevronRight,
  Link2,
  MoreHorizontal,
  PanelRight,
  Trash2,
  UserMinus,
} from "lucide-react";
import { toast } from "sonner";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
} from "@/components/ui/dropdown-menu";
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { ActorAvatar } from "@/components/common/actor-avatar";
import type { UpdateIssueRequest } from "@/shared/types";
import { ALL_STATUSES, STATUS_CONFIG, PRIORITY_ORDER, PRIORITY_CONFIG } from "@/features/issues/config";
import { StatusIcon, PriorityIcon, canAssignAgent } from "@/features/issues/components";
import type { Issue, MemberWithUser, Agent } from "@/shared/types";

export interface IssueDetailHeaderProps {
  issue: Issue;
  workspaceName: string | undefined;
  allIssues: { id: string }[];
  prevIssueId: string | null;
  nextIssueId: string | null;
  currentIndex: number;
  members: MemberWithUser[];
  agents: Agent[];
  userId: string | undefined;
  currentMemberRole: string | undefined;
  sidebarOpen: boolean;
  sidebarRef: { current: { isCollapsed: () => boolean; expand: () => void; collapse: () => void } | null };
  onUpdateField: (updates: Partial<UpdateIssueRequest>) => void;
  onDelete: () => Promise<void>;
}

function IssueDetailHeaderImpl({
  issue,
  workspaceName,
  allIssues,
  prevIssueId,
  nextIssueId,
  currentIndex,
  members,
  agents,
  userId,
  currentMemberRole,
  sidebarOpen,
  sidebarRef,
  onUpdateField,
  onDelete,
}: IssueDetailHeaderProps) {
  const router = useRouter();
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const handleConfirmDelete = async () => {
    setDeleting(true);
    try {
      await onDelete();
    } catch {
      toast.error("Failed to delete issue");
      setDeleting(false);
    }
  };

  return (
    <div className="flex h-12 shrink-0 items-center justify-between border-b bg-background px-4 text-sm">
      <div className="flex items-center gap-1.5 min-w-0">
        {workspaceName && (
          <>
            <Link
              href="/issues"
              className="text-muted-foreground hover:text-foreground transition-colors truncate shrink-0"
            >
              {workspaceName}
            </Link>
            <ChevronRight className="h-3 w-3 text-muted-foreground/50 shrink-0" />
          </>
        )}
        <span className="truncate text-muted-foreground">
          {issue.identifier}
        </span>
        <ChevronRight className="h-3 w-3 text-muted-foreground/50 shrink-0" />
        <span className="truncate">{issue.title}</span>
      </div>
      <div className="flex items-center gap-1 shrink-0">
        {/* Issue navigation */}
        {allIssues.length > 1 && (
          <div className="flex items-center gap-0.5 mr-1">
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    className="text-muted-foreground"
                    disabled={!prevIssueId}
                    onClick={() => prevIssueId && router.push(`/issues/${prevIssueId}`)}
                    aria-label="Previous issue"
                  >
                    <ChevronLeft className="h-4 w-4" />
                  </Button>
                }
              />
              <TooltipContent side="bottom">Previous issue</TooltipContent>
            </Tooltip>
            <span className="text-xs text-muted-foreground tabular-nums px-0.5">
              {currentIndex >= 0 ? currentIndex + 1 : "?"} / {allIssues.length}
            </span>
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    className="text-muted-foreground"
                    disabled={!nextIssueId}
                    onClick={() => nextIssueId && router.push(`/issues/${nextIssueId}`)}
                    aria-label="Next issue"
                  >
                    <ChevronRight className="h-4 w-4" />
                  </Button>
                }
              />
              <TooltipContent side="bottom">Next issue</TooltipContent>
            </Tooltip>
          </div>
        )}
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button variant="ghost" size="icon-xs" className="text-muted-foreground" aria-label="Issue actions">
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            }
          />
          <DropdownMenuContent align="end" className="w-auto">
            {/* Status */}
            <DropdownMenuSub>
              <DropdownMenuSubTrigger>
                <StatusIcon status={issue.status} className="h-3.5 w-3.5" />
                Status
              </DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                {ALL_STATUSES.map((s) => (
                  <DropdownMenuItem
                    key={s}
                    onClick={() => onUpdateField({ status: s })}
                  >
                    <StatusIcon status={s} className="h-3.5 w-3.5" />
                    {STATUS_CONFIG[s].label}
                    {issue.status === s && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuSubContent>
            </DropdownMenuSub>

            {/* Priority */}
            <DropdownMenuSub>
              <DropdownMenuSubTrigger>
                <PriorityIcon priority={issue.priority} />
                Priority
              </DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                {PRIORITY_ORDER.map((p) => (
                  <DropdownMenuItem
                    key={p}
                    onClick={() => onUpdateField({ priority: p })}
                  >
                    <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium ${PRIORITY_CONFIG[p].badgeBg} ${PRIORITY_CONFIG[p].badgeText}`}>
                      <PriorityIcon priority={p} className="h-3 w-3" inheritColor />
                      {PRIORITY_CONFIG[p].label}
                    </span>
                    {issue.priority === p && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuSubContent>
            </DropdownMenuSub>

            {/* Assignee */}
            <DropdownMenuSub>
              <DropdownMenuSubTrigger>
                <UserMinus className="h-3.5 w-3.5" />
                Assignee
              </DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                <DropdownMenuItem
                  onClick={() => onUpdateField({ assignee_type: null, assignee_id: null })}
                >
                  <UserMinus className="h-3.5 w-3.5 text-muted-foreground" />
                  Unassigned
                  {!issue.assignee_type && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
                </DropdownMenuItem>
                {members.map((m) => (
                  <DropdownMenuItem
                    key={m.user_id}
                    onClick={() => onUpdateField({ assignee_type: "member", assignee_id: m.user_id })}
                  >
                    <ActorAvatar actorType="member" actorId={m.user_id} size={16} />
                    {m.name}
                    {issue.assignee_type === "member" && issue.assignee_id === m.user_id && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
                  </DropdownMenuItem>
                ))}
                {agents.filter((a) => !a.archived_at && canAssignAgent(a, userId, currentMemberRole)).map((a) => (
                  <DropdownMenuItem
                    key={a.id}
                    onClick={() => onUpdateField({ assignee_type: "agent", assignee_id: a.id })}
                  >
                    <ActorAvatar actorType="agent" actorId={a.id} size={16} />
                    {a.name}
                    {issue.assignee_type === "agent" && issue.assignee_id === a.id && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuSubContent>
            </DropdownMenuSub>

            {/* Due date */}
            <DropdownMenuSub>
              <DropdownMenuSubTrigger>
                <Calendar className="h-3.5 w-3.5" />
                Due date
              </DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                <DropdownMenuItem onClick={() => onUpdateField({ due_date: new Date().toISOString() })}>
                  Today
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => {
                  const d = new Date(); d.setDate(d.getDate() + 1);
                  onUpdateField({ due_date: d.toISOString() });
                }}>
                  Tomorrow
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => {
                  const d = new Date(); d.setDate(d.getDate() + 7);
                  onUpdateField({ due_date: d.toISOString() });
                }}>
                  Next week
                </DropdownMenuItem>
                {issue.due_date && (
                  <>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={() => onUpdateField({ due_date: null })}>
                      Clear date
                    </DropdownMenuItem>
                  </>
                )}
              </DropdownMenuSubContent>
            </DropdownMenuSub>

            <DropdownMenuSeparator />

            {/* Copy link */}
            <DropdownMenuItem onClick={() => {
              navigator.clipboard.writeText(window.location.href);
              toast.success("Link copied");
            }}>
              <Link2 className="h-3.5 w-3.5" />
              Copy link
            </DropdownMenuItem>

            <DropdownMenuSeparator />

            {/* Delete */}
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setDeleteDialogOpen(true)}
            >
              <Trash2 className="h-3.5 w-3.5" />
              Delete issue
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant={sidebarOpen ? "secondary" : "ghost"}
                size="icon-xs"
                className={sidebarOpen ? "" : "text-muted-foreground"}
                aria-label="Toggle sidebar"
                onClick={() => {
                  const panel = sidebarRef.current;
                  if (!panel) return;
                  if (panel.isCollapsed()) panel.expand();
                  else panel.collapse();
                }}
              >
                <PanelRight className="h-4 w-4" />
              </Button>
            }
          />
          <TooltipContent side="bottom">Toggle sidebar</TooltipContent>
        </Tooltip>
      </div>

      {/* Delete confirmation dialog (controlled by state) */}
      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete issue</AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete this issue and all its comments. This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleConfirmDelete}
              disabled={deleting}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {deleting ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

export const IssueDetailHeader = memo(IssueDetailHeaderImpl);
