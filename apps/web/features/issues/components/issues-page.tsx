"use client";

import { useEffect, useLayoutEffect, useMemo, useRef } from "react";
import { ListTodo, Plus } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { Breadcrumb, BreadcrumbList, BreadcrumbItem, BreadcrumbSeparator, BreadcrumbPage } from "@/components/ui/breadcrumb";
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription } from "@/components/ui/empty";
import { AlertCircle } from "lucide-react";
import { useIssueStore } from "@/features/issues/store";
import { useIssueViewStore, initFilterWorkspaceSync } from "@/features/issues/stores/view-store";
import { useIssuesScopeStore } from "@/features/issues/stores/issues-scope-store";
import { useIssuesMeFilterStore } from "@/features/issues/stores/me-filter-store";
import { useAuthStore } from "@/features/auth";
import { ViewStoreProvider } from "@/features/issues/stores/view-store-context";
import { filterIssues } from "@/features/issues/utils/filter";
import { useWorkspaceStore } from "@/features/workspace";
import { WorkspaceAvatar } from "@/features/workspace";
import { useIssueSelectionStore } from "@/features/issues/stores/selection-store";
import { useModalStore } from "@/features/modals";
import { IssuesHeader } from "./issues-header";
import { IssuesCardGrid } from "./issues-card-grid";
import { BatchActionToolbar } from "./batch-action-toolbar";

function InfiniteScrollSentinel({
  loadingMore,
  onLoadMore,
}: {
  loadingMore: boolean;
  onLoadMore: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (loadingMore) return;
    const el = ref.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) onLoadMore();
      },
      { rootMargin: "200px" },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [loadingMore, onLoadMore]);

  return (
    <div ref={ref} className="flex justify-center py-3" data-testid="infinite-scroll-sentinel">
      {loadingMore && (
        <span className="text-xs text-muted-foreground">Loading…</span>
      )}
    </div>
  );
}

export function IssuesPage() {
  const allIssues = useIssueStore((s) => s.issues);
  const loading = useIssueStore((s) => s.loading);
  const loadingMore = useIssueStore((s) => s.loadingMore);
  const hasMore = useIssueStore((s) => s.hasMore);
  const error = useIssueStore((s) => s.error);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const agents = useWorkspaceStore((s) => s.agents);
  const user = useAuthStore((s) => s.user);
  const scope = useIssuesScopeStore((s) => s.scope);
  const meFilter = useIssuesMeFilterStore((s) => s.meFilter);
  const statusFilters = useIssueViewStore((s) => s.statusFilters);
  const priorityFilters = useIssueViewStore((s) => s.priorityFilters);
  const assigneeFilters = useIssueViewStore((s) => s.assigneeFilters);
  const includeNoAssignee = useIssueViewStore((s) => s.includeNoAssignee);
  const creatorFilters = useIssueViewStore((s) => s.creatorFilters);

  useEffect(() => {
    initFilterWorkspaceSync();
  }, []);

  useLayoutEffect(() => {
    if (!workspace?.id) return;
    void useIssueStore.getState().fetch();
  }, [workspace?.id]);

  useEffect(() => {
    useIssueSelectionStore.getState().clear();
  }, [scope, meFilter]);

  const myAgentIds = useMemo(() => {
    if (!user) return new Set<string>();
    return new Set(
      agents.filter((a) => a.owner_id === user.id).map((a) => a.id),
    );
  }, [agents, user]);

  const meScopedIssues = useMemo(() => {
    if (!user || meFilter === "off") return allIssues;
    if (meFilter === "assigned") {
      return allIssues.filter(
        (i) => i.assignee_type === "member" && i.assignee_id === user.id,
      );
    }
    if (meFilter === "created") {
      return allIssues.filter(
        (i) => i.creator_type === "member" && i.creator_id === user.id,
      );
    }
    return allIssues.filter(
      (i) =>
        i.assignee_type === "agent" &&
        !!i.assignee_id &&
        myAgentIds.has(i.assignee_id),
    );
  }, [allIssues, user, meFilter, myAgentIds]);

  const scopedIssues = useMemo(() => {
    if (scope === "members")
      return meScopedIssues.filter((i) => i.assignee_type === "member");
    if (scope === "agents")
      return meScopedIssues.filter((i) => i.assignee_type === "agent");
    return meScopedIssues;
  }, [meScopedIssues, scope]);

  const issues = useMemo(
    () => filterIssues(scopedIssues, { statusFilters, priorityFilters, assigneeFilters, includeNoAssignee, creatorFilters }),
    [scopedIssues, statusFilters, priorityFilters, assigneeFilters, includeNoAssignee, creatorFilters],
  );

  if (loading) {
    return (
      <div className="flex flex-1 min-h-0 flex-col" role="status" aria-label="Loading issues">
        <div className="flex h-14 min-h-14 shrink-0 items-center gap-3 border-b px-4">
          <Skeleton className="h-6 w-40 shrink-0 rounded-md" />
          <Skeleton className="mx-auto h-9 max-w-xs flex-1 rounded-full" />
          <Skeleton className="h-8 w-28 shrink-0 rounded-full" />
        </div>
        <div className="mx-auto grid w-full max-w-[1680px] flex-1 grid-cols-1 gap-3 p-3 sm:grid-cols-2 sm:gap-4 lg:grid-cols-3 xl:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-40 rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  const breadcrumb = (
    <Breadcrumb>
      <BreadcrumbList>
        <BreadcrumbItem>
          <WorkspaceAvatar name={workspace?.name ?? "W"} size="sm" />
          <span className="ml-1.5 truncate text-sm text-muted-foreground">
            {workspace?.name ?? "Workspace"}
          </span>
        </BreadcrumbItem>
        <BreadcrumbSeparator />
        <BreadcrumbItem>
          <BreadcrumbPage className="text-sm font-medium">Issues</BreadcrumbPage>
        </BreadcrumbItem>
      </BreadcrumbList>
    </Breadcrumb>
  );

  if (error && scopedIssues.length === 0) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        <div className="flex h-14 min-h-14 shrink-0 items-center gap-1.5 border-b px-4">
          {breadcrumb}
        </div>
        <div className="flex flex-1 items-center justify-center">
          <div className="flex flex-col items-center gap-2 text-center">
            <AlertCircle className="size-8 text-destructive" />
            <p className="text-sm font-medium">Failed to load issues</p>
            <p className="text-xs text-muted-foreground">{error}</p>
            <Button
              variant="outline"
              size="sm"
              onClick={() => useIssueStore.getState().fetch()}
            >
              Retry
            </Button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      <IssuesHeader scopedIssues={scopedIssues} leadingSlot={breadcrumb} />

      <ViewStoreProvider store={useIssueViewStore}>
        {scopedIssues.length === 0 ? (
          <Empty className="flex-1 border-0">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <ListTodo aria-hidden="true" />
              </EmptyMedia>
              <EmptyTitle>Create your first task</EmptyTitle>
              <EmptyDescription>Issues track work items. Assign them to teammates or AI Agents to get things done.</EmptyDescription>
            </EmptyHeader>
            <Button
              variant="outline"
              size="sm"
              onClick={() => useModalStore.getState().open("create-issue")}
            >
              <Plus className="size-3.5 mr-1" aria-hidden="true" />
              New issue
            </Button>
          </Empty>
        ) : (
          <div className="flex min-h-0 flex-1 flex-col">
            <IssuesCardGrid issues={issues} />
            {hasMore && (
              <InfiniteScrollSentinel
                loadingMore={loadingMore}
                onLoadMore={() => useIssueStore.getState().loadMore()}
              />
            )}
          </div>
        )}
        <BatchActionToolbar />
      </ViewStoreProvider>
    </div>
  );
}
