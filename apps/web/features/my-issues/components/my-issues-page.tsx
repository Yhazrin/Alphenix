"use client";

import { useEffect, useLayoutEffect, useMemo } from "react";
import { useStore } from "zustand";
import { ListTodo } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore, WorkspaceAvatar } from "@/features/workspace";
import { Breadcrumb, BreadcrumbList, BreadcrumbItem, BreadcrumbSeparator, BreadcrumbPage } from "@/components/ui/breadcrumb";
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription } from "@/components/ui/empty";
import { useIssueStore } from "@/features/issues/store";
import { filterIssues } from "@/features/issues/utils/filter";
import { ViewStoreProvider } from "@/features/issues/stores/view-store-context";
import { useIssueSelectionStore } from "@/features/issues/stores/selection-store";
import { IssuesCardGrid } from "@/features/issues/components/issues-card-grid";
import { BatchActionToolbar } from "@/features/issues/components/batch-action-toolbar";
import { registerViewStoreForWorkspaceSync } from "@/features/issues/stores/view-store";
import { myIssuesViewStore } from "../stores/my-issues-view-store";
import { MyIssuesHeader } from "./my-issues-header";

export function MyIssuesPage() {
  const user = useAuthStore((s) => s.user);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const agents = useWorkspaceStore((s) => s.agents);
  const allIssues = useIssueStore((s) => s.issues);
  const loading = useIssueStore((s) => s.loading);

  const statusFilters = useStore(myIssuesViewStore, (s) => s.statusFilters);
  const priorityFilters = useStore(myIssuesViewStore, (s) => s.priorityFilters);
  const scope = useStore(myIssuesViewStore, (s) => s.scope);

  useEffect(() => {
    registerViewStoreForWorkspaceSync(myIssuesViewStore);
  }, []);

  useLayoutEffect(() => {
    if (!workspace?.id) return;
    void useIssueStore.getState().fetch();
  }, [workspace?.id]);

  useEffect(() => {
    useIssueSelectionStore.getState().clear();
  }, [scope]);

  const myAgentIds = useMemo(() => {
    if (!user) return new Set<string>();
    return new Set(
      agents.filter((a) => a.owner_id === user.id).map((a) => a.id),
    );
  }, [agents, user]);

  const assignedToMe = useMemo(() => {
    if (!user) return [];
    return allIssues.filter(
      (i) => i.assignee_type === "member" && i.assignee_id === user.id,
    );
  }, [allIssues, user]);

  const myAgentIssues = useMemo(() => {
    if (!user) return [];
    return allIssues.filter(
      (i) =>
        i.assignee_type === "agent" &&
        i.assignee_id &&
        myAgentIds.has(i.assignee_id),
    );
  }, [allIssues, user, myAgentIds]);

  const createdByMe = useMemo(() => {
    if (!user) return [];
    return allIssues.filter(
      (i) => i.creator_type === "member" && i.creator_id === user.id,
    );
  }, [allIssues, user]);

  const myIssues = useMemo(() => {
    switch (scope) {
      case "assigned": return assignedToMe;
      case "agents": return myAgentIssues;
      case "created": return createdByMe;
      default: return assignedToMe;
    }
  }, [scope, assignedToMe, myAgentIssues, createdByMe]);

  const issues = useMemo(
    () =>
      filterIssues(myIssues, {
        statusFilters,
        priorityFilters,
        assigneeFilters: [],
        includeNoAssignee: false,
        creatorFilters: [],
      }),
    [myIssues, statusFilters, priorityFilters],
  );

  if (loading) {
    return (
      <div className="flex flex-1 min-h-0 flex-col" role="status" aria-label="Loading issues">
        <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
          <Skeleton className="h-5 w-5 rounded" />
          <Skeleton className="h-4 w-32" />
        </div>
        <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
          <Skeleton className="h-5 w-24" />
          <Skeleton className="h-8 w-24" />
        </div>
        <div className="mx-auto grid w-full max-w-[1680px] flex-1 grid-cols-1 gap-3 p-3 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-40 rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      <div className="flex h-12 shrink-0 items-center gap-1.5 border-b px-4">
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <WorkspaceAvatar name={workspace?.name ?? "W"} size="sm" />
              <span className="ml-1.5 text-sm text-muted-foreground">
                {workspace?.name ?? "Workspace"}
              </span>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage className="text-sm font-medium">My Issues</BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
      </div>

      <MyIssuesHeader allIssues={myIssues} />

      <ViewStoreProvider store={myIssuesViewStore}>
        {myIssues.length === 0 ? (
          <Empty className="flex-1 border-0">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <ListTodo aria-hidden="true" />
              </EmptyMedia>
              <EmptyTitle>No issues assigned to you</EmptyTitle>
              <EmptyDescription>Issues assigned to you by teammates or AI Agents will appear here.</EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <div className="flex min-h-0 flex-1 flex-col">
            <IssuesCardGrid issues={issues} />
          </div>
        )}
        <BatchActionToolbar />
      </ViewStoreProvider>
    </div>
  );
}
