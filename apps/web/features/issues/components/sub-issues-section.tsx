"use client";

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { ListTree, Sparkles } from "lucide-react";
import { Progress, ProgressTrack, ProgressIndicator } from "@/components/ui/progress";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusIcon } from "./status-icon";
import { CollapsibleSection } from "./collapsible-section";
import { DecomposeDialog } from "./decompose-dialog";
import { DependencyBadges } from "./dependency-badge";
import { api } from "@/shared/api";
import { useIssueStore } from "@/features/issues/store";
import type { Issue } from "@/shared/types";

interface SubIssuesSectionProps {
  issueId: string;
  issueKind: string;
}

export function SubIssuesSection({ issueId, issueKind }: SubIssuesSectionProps) {
  const [subIssues, setSubIssues] = useState<Issue[]>([]);
  const [loading, setLoading] = useState(true);
  const [decomposeOpen, setDecomposeOpen] = useState(false);

  const fetchSubIssues = useCallback(async () => {
    let cancelled = false;
    try {
      const issues = await api.listSubIssues(issueId);
      if (cancelled) return;
      setSubIssues(issues);
      // Upsert into global store so identifier lookups work elsewhere.
      const store = useIssueStore.getState();
      for (const issue of issues) {
        store.addIssue(issue);
      }
    } catch {
      if (cancelled) return;
      // Silently fail — section will show empty state.
    } finally {
      if (!cancelled) setLoading(false);
    }
    return () => { cancelled = true; };
  }, [issueId]);

  useEffect(() => {
    const cleanup = fetchSubIssues();
    return () => { cleanup?.then((fn) => fn?.()); };
  }, [fetchSubIssues]);

  const handleDecomposeComplete = useCallback((issues: Issue[]) => {
    setSubIssues(issues);
    const store = useIssueStore.getState();
    for (const issue of issues) {
      store.addIssue(issue);
    }
  }, []);

  if (issueKind !== "goal") return null;

  const completedCount = subIssues.filter((i) => i.status === "done" || i.status === "cancelled").length;
  const totalCount = subIssues.length;

  return (
    <>
      <CollapsibleSection
        title="Sub-Issues"
        icon={<ListTree className="h-3.5 w-3.5 text-muted-foreground" aria-hidden="true" />}
        count={totalCount}
        defaultOpen={totalCount > 0}
      >
        {loading ? (
          <div className="space-y-2">
            <Skeleton className="h-7 w-full" />
            <Skeleton className="h-7 w-full" />
          </div>
        ) : totalCount === 0 ? (
          <div className="flex flex-col items-center gap-2 py-3">
            <p className="text-xs text-muted-foreground">No sub-issues yet.</p>
            <Button
              variant="outline"
              size="sm"
              className="h-7 text-xs"
              onClick={() => setDecomposeOpen(true)}
            >
              <Sparkles className="mr-1.5 h-3 w-3" aria-hidden="true" />
              Decompose with AI
            </Button>
          </div>
        ) : (
          <div className="space-y-2">
            {/* Progress bar */}
            <div className="flex items-center gap-2 text-xs text-muted-foreground mb-1">
              <Progress value={totalCount > 0 ? (completedCount / totalCount) * 100 : 0} className="flex-1 h-1">
                <ProgressTrack>
                  <ProgressIndicator />
                </ProgressTrack>
              </Progress>
              <span className="shrink-0 tabular-nums">{completedCount}/{totalCount}</span>
            </div>

            {/* Sub-issue list */}
            <div className="space-y-0.5">
              {subIssues.map((sub) => (
                <Link
                  key={sub.id}
                  href={`/issues/${sub.id}`}
                  className="flex items-center gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-muted/50 transition-colors group"
                >
                  <StatusIcon status={sub.status} className="h-3.5 w-3.5 shrink-0" />
                  <span className="font-mono text-muted-foreground shrink-0">{sub.identifier}</span>
                  <span className="truncate flex-1">{sub.title}</span>
                  <DependencyBadges issueId={sub.id} />
                </Link>
              ))}
            </div>

            <Button
              variant="ghost"
              size="sm"
              className="h-6 text-xs text-muted-foreground w-full"
              onClick={() => setDecomposeOpen(true)}
            >
              <Sparkles className="mr-1.5 h-3 w-3" aria-hidden="true" />
              Re-decompose
            </Button>
          </div>
        )}
      </CollapsibleSection>

      <DecomposeDialog
        issueId={issueId}
        open={decomposeOpen}
        onOpenChange={setDecomposeOpen}
        onComplete={handleDecomposeComplete}
      />
    </>
  );
}
