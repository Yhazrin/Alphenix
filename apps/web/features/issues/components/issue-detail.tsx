"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useShallow } from "zustand/react/shallow";
import { useDefaultLayout, usePanelRef } from "react-resizable-panels";
import { useRouter } from "next/navigation";
import {
  ChevronDown,
  ChevronLeft,
} from "lucide-react";
import { toast } from "sonner";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { ResizablePanelGroup, ResizablePanel, ResizableHandle } from "@/components/ui/resizable";
import { ContentEditor, type ContentEditorRef } from "@/features/editor";
import { FileUploadButton } from "@/components/common/file-upload-button";
import { TitleEditor } from "@/features/editor";
import type { UpdateIssueRequest } from "@/shared/types";
import { AgentLiveCard, TaskRunHistory } from "./agent-live-card";
import { CollaborationPanel } from "./collaboration-panel";
import { RunTimeline } from "./run-timeline";
import { api } from "@/shared/api";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore, useActorName } from "@/features/workspace";
import { useIssueStore } from "@/features/issues";
import { useIssueTimeline } from "@/features/issues/hooks/use-issue-timeline";
import { useIssueReactions } from "@/features/issues/hooks/use-issue-reactions";
import { useIssueSubscribers } from "@/features/issues/hooks/use-issue-subscribers";
import { ReactionBar } from "@/components/common/reaction-bar";
import { useFileUpload } from "@/shared/hooks/use-file-upload";
import { IssueDetailHeader } from "./issue-detail-header";
import { IssueActivityTimeline } from "./issue-activity-timeline";
import { IssuePropertiesSidebar } from "./issue-properties-sidebar";

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface IssueDetailProps {
  issueId: string;
  onDelete?: () => void;
  defaultSidebarOpen?: boolean;
  layoutId?: string;
  /** When set, the issue detail will auto-scroll to this comment and briefly highlight it. */
  highlightCommentId?: string;
}

// ---------------------------------------------------------------------------
// IssueDetail
// ---------------------------------------------------------------------------

export function IssueDetail({ issueId, onDelete, defaultSidebarOpen = true, layoutId = "alphenix_issue_detail_layout", highlightCommentId }: IssueDetailProps) {
  const id = issueId;
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const members = useWorkspaceStore((s) => s.members);
  const agents = useWorkspaceStore((s) => s.agents);
  const currentMemberRole = members.find((m) => m.user_id === user?.id)?.role;

  // Issue navigation
  const allIssues = useIssueStore((s) => s.issues ?? []);
  const currentIndex = allIssues.findIndex((i) => i.id === id);
  const prevIssue = currentIndex > 0 ? allIssues[currentIndex - 1] : null;
  const nextIssue = currentIndex < allIssues.length - 1 ? allIssues[currentIndex + 1] : null;
  const { getActorName } = useActorName();
  const { uploadWithToast } = useFileUpload();
  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id: layoutId,
  });
  const sidebarRef = usePanelRef();
  const [sidebarOpen, setSidebarOpen] = useState(defaultSidebarOpen);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const [showScrollBottom, setShowScrollBottom] = useState(false);
  const [highlightedId, setHighlightedId] = useState<string | null>(null);
  const didHighlightRef = useRef<string | null>(null);

  // Single source of truth: read issue directly from global store
  const issue = useIssueStore(useShallow((s) => s.issues.find((i) => i.id === id))) ?? null;
  const [issueLoading, setIssueLoading] = useState(!issue);

  // If issue isn't in the store yet, fetch and upsert it
  useEffect(() => {
    if (issue) {
      setIssueLoading(false);
      return;
    }
    let cancelled = false;
    setIssueLoading(true);
    api
      .getIssue(id)
      .then((iss) => {
        if (!cancelled) useIssueStore.getState().addIssue(iss);
      })
      .catch((e) => {
        if (cancelled) return;
        console.error(e instanceof Error ? e : String(e));
        toast.error("Failed to load issue");
      })
      .finally(() => { if (!cancelled) setIssueLoading(false); });
    return () => { cancelled = true; };
  }, [id, issue?.id]);

  // Custom hooks — encapsulate timeline, reactions, subscribers
  const {
    timeline, loading: timelineLoading, error: timelineError, submitComment, submitReply,
    editComment, deleteComment, toggleReaction: handleToggleReaction,
  } = useIssueTimeline(id, user?.id, workspace?.id);

  const {
    reactions: issueReactions, loading: reactionsLoading,
    toggleReaction: handleToggleIssueReaction,
  } = useIssueReactions(id, user?.id);

  const {
    subscribers, loading: subscribersLoading, isSubscribed, toggleSubscribe: handleToggleSubscribe, toggleSubscriber,
  } = useIssueSubscribers(id, user?.id);

  const loading = issueLoading;

  // Scroll to highlighted comment once timeline loads (fire only once per highlightCommentId)
  useEffect(() => {
    if (!highlightCommentId || timeline.length === 0) return;
    if (didHighlightRef.current === highlightCommentId) return;
    const el = document.getElementById(`comment-${highlightCommentId}`);
    if (el) {
      didHighlightRef.current = highlightCommentId;
      let timer: ReturnType<typeof setTimeout> | undefined;
      const raf = requestAnimationFrame(() => {
        el.scrollIntoView({ behavior: "smooth", block: "center" });
        setHighlightedId(highlightCommentId);
        timer = setTimeout(() => setHighlightedId(null), 2000);
      });
      return () => {
        cancelAnimationFrame(raf);
        clearTimeout(timer);
      };
    }
  }, [highlightCommentId, timeline.length]);

  // Track scroll position for jump-to-bottom button (throttled via rAF)
  const rafIdRef = useRef<number>(0);
  useEffect(() => {
    const container = scrollContainerRef.current;
    if (!container) return;
    const onScroll = () => {
      if (rafIdRef.current) return; // already scheduled
      rafIdRef.current = requestAnimationFrame(() => {
        rafIdRef.current = 0;
        const { scrollTop, scrollHeight, clientHeight } = container;
        setShowScrollBottom(scrollHeight - scrollTop - clientHeight > 200);
      });
    };
    container.addEventListener("scroll", onScroll, { passive: true });
    onScroll();
    return () => {
      container.removeEventListener("scroll", onScroll);
      if (rafIdRef.current) cancelAnimationFrame(rafIdRef.current);
    };
  }, []);

  const scrollToBottom = useCallback(() => {
    scrollContainerRef.current?.scrollTo({ top: scrollContainerRef.current.scrollHeight, behavior: "smooth" });
  }, []);

  const scrollToAgentCollab = useCallback(() => {
    document.getElementById("issue-agent-collab-anchor")?.scrollIntoView({ behavior: "smooth", block: "start" });
  }, []);

  // Issue field updates — write directly to the global store (single source of truth)
  const handleUpdateField = useCallback(
    (updates: Partial<UpdateIssueRequest>) => {
      if (!issue) return;
      const prev = { ...issue };
      useIssueStore.getState().updateIssue(id, updates);
      api.updateIssue(id, updates).catch(() => {
        useIssueStore.getState().updateIssue(id, prev);
        toast.error("Failed to update issue");
      });
    },
    [issue, id],
  );

  const descEditorRef = useRef<ContentEditorRef>(null);
  const handleDescriptionUpload = useCallback(
    (file: File) => uploadWithToast(file, { issueId: id }),
    [uploadWithToast, id],
  );

  const handleDelete = useCallback(async () => {
    if (!issue) return;
    try {
      await api.deleteIssue(issue.id);
      useIssueStore.getState().removeIssue(issue.id);
      toast.success("Issue deleted");
      if (onDelete) onDelete();
      else router.push("/issues");
    } catch {
      toast.error("Failed to delete issue");
    }
  }, [issue, onDelete, router]);

  if (loading) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        {/* Header skeleton */}
        <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
          <Skeleton className="h-4 w-16" />
          <Skeleton className="h-4 w-4" />
          <Skeleton className="h-4 w-24" />
        </div>
        <div className="flex flex-1 min-h-0">
          {/* Content skeleton */}
          <div className="flex-1 p-8 space-y-6">
            <Skeleton className="h-8 w-3/4" />
            <div className="space-y-2">
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-5/6" />
              <Skeleton className="h-4 w-2/3" />
            </div>
            <Skeleton className="h-px w-full" />
            <div className="space-y-3">
              <Skeleton className="h-4 w-20" />
              <div className="flex items-start gap-3">
                <Skeleton className="h-8 w-8 rounded-full" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-4 w-32" />
                  <Skeleton className="h-16 w-full rounded-lg" />
                </div>
              </div>
            </div>
          </div>
          {/* Sidebar skeleton */}
          <div className="w-64 border-l p-4 space-y-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="flex items-center justify-between">
                <Skeleton className="h-3 w-16" />
                <Skeleton className="h-5 w-24" />
              </div>
            ))}
            <Skeleton className="h-px w-full" />
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="flex items-center justify-between">
                <Skeleton className="h-3 w-16" />
                <Skeleton className="h-4 w-28" />
              </div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (!issue) {
    return (
      <div className="flex flex-1 min-h-0 flex-col items-center justify-center gap-3 text-sm text-muted-foreground">
        <p>This issue does not exist or has been deleted in this workspace.</p>
        {!onDelete && (
          <Button variant="outline" size="sm" onClick={() => router.push("/issues")}>
            <ChevronLeft className="mr-1 h-3.5 w-3.5" aria-hidden="true" />
            Back to Issues
          </Button>
        )}
      </div>
    );
  }

  return (
    <ResizablePanelGroup orientation="horizontal" className="flex-1 min-h-0" defaultLayout={defaultLayout} onLayoutChanged={onLayoutChanged}>
      <ResizablePanel id="content" minSize="50%">
      {/* LEFT: Content area */}
      <div className="flex h-full flex-col">
        <IssueDetailHeader
          issue={issue}
          workspaceName={workspace?.name}
          allIssues={allIssues}
          prevIssueId={prevIssue?.id ?? null}
          nextIssueId={nextIssue?.id ?? null}
          currentIndex={currentIndex}
          members={members}
          agents={agents}
          userId={user?.id}
          currentMemberRole={currentMemberRole}
          sidebarOpen={sidebarOpen}
          sidebarRef={sidebarRef}
          onUpdateField={handleUpdateField}
          onDelete={handleDelete}
        />

        {/* Content — scrollable */}
        <div ref={scrollContainerRef} className="relative flex-1 overflow-y-auto">
        <div className="mx-auto w-full max-w-4xl space-y-8 px-6 py-6 sm:px-8 sm:py-8">
          <section className="rounded-2xl border border-border/60 bg-card/50 p-6 shadow-sm sm:p-8">
            <TitleEditor
              key={`title-${id}`}
              defaultValue={issue.title}
              placeholder="Issue title"
              className="w-full text-balance text-2xl font-bold leading-snug tracking-tight"
              onBlur={(value) => {
                const trimmed = value.trim();
                if (trimmed && trimmed !== issue.title) handleUpdateField({ title: trimmed });
              }}
            />

            <ContentEditor
              ref={descEditorRef}
              key={id}
              defaultValue={issue.description || ""}
              placeholder="Add description..."
              onUpdate={(md) => handleUpdateField({ description: md || undefined })}
              onUploadFile={handleDescriptionUpload}
              debounceMs={1500}
              className="mt-5"
            />

            <div className="mt-3 flex items-center gap-1">
              {reactionsLoading ? (
                <div className="flex items-center gap-1">
                  <Skeleton className="h-7 w-14 rounded-full" />
                  <Skeleton className="h-7 w-14 rounded-full" />
                </div>
              ) : (
                <ReactionBar
                  reactions={issueReactions}
                  currentUserId={user?.id}
                  onToggle={handleToggleIssueReaction}
                />
              )}
              <FileUploadButton
                size="sm"
                onSelect={(file) => descEditorRef.current?.uploadFile(file)}
              />
            </div>
          </section>

          {/* Activity / Comments */}
          <div>
            <div className="flex items-center justify-between gap-3">
              <h2 className="text-lg font-semibold tracking-tight">Activity</h2>
            </div>

            <section
              id="issue-agent-collab-anchor"
              className="mt-5 space-y-3 rounded-2xl border border-border/60 bg-muted/20 p-4 sm:p-5"
              aria-label="Agent task and collaboration"
            >
              <div className="flex items-center justify-between gap-2 border-b border-border/50 pb-3">
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  Live task & collaboration
                </h3>
              </div>
              <AgentLiveCard
                issueId={id}
                agentName={issue.assignee_type === "agent" && issue.assignee_id ? getActorName("agent", issue.assignee_id) : undefined}
                scrollContainerRef={scrollContainerRef}
              />
              <TaskRunHistory issueId={id} />
              <RunTimeline issueId={id} />
              <CollaborationPanel issueId={id} />
            </section>

            {/* Timeline entries */}
            <IssueActivityTimeline
              issueId={id}
              timeline={timeline}
              loading={timelineLoading}
              error={timelineError}
              currentUserId={user?.id}
              getActorName={getActorName}
              submitReply={submitReply}
              editComment={editComment}
              deleteComment={deleteComment}
              handleToggleReaction={handleToggleReaction}
              submitComment={submitComment}
              highlightedId={highlightedId}
            />
          </div>
        </div>
        {/* Jump to bottom button */}
        {showScrollBottom && (
          <div className="sticky bottom-4 flex justify-center pointer-events-none">
            <Button
              variant="secondary"
              size="sm"
              className="pointer-events-auto shadow-md dark:shadow-none"
              onClick={scrollToBottom}
            >
              <ChevronDown className="mr-1 h-3.5 w-3.5" aria-hidden="true" />
              Jump to bottom
            </Button>
          </div>
        )}
        </div>
      </div>
      </ResizablePanel>
      <ResizableHandle />
      <ResizablePanel
        id="sidebar"
        defaultSize={defaultSidebarOpen ? 340 : 0}
        minSize={280}
        maxSize={460}
        collapsible
        groupResizeBehavior="preserve-pixel-size"
        panelRef={sidebarRef}
        onResize={(size) => setSidebarOpen(size.inPixels > 0)}
      >
        <IssuePropertiesSidebar
          issue={issue}
          getActorName={getActorName}
          onUpdateField={handleUpdateField}
          subscribers={subscribers}
          subscribersLoading={subscribersLoading}
          isSubscribed={isSubscribed}
          onToggleSubscribe={handleToggleSubscribe}
          toggleSubscriber={toggleSubscriber}
          members={members}
          agents={agents}
          onScrollToAgentSection={scrollToAgentCollab}
        />
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}
