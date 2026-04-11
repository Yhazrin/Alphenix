"use client";

import { useMemo } from "react";
import type { Issue } from "@/shared/types";
import { useViewStore } from "@/features/issues/stores/view-store-context";
import { sortIssues } from "@/features/issues/utils/sort";
import { IssueGalleryCard } from "./issue-gallery-card";

export function IssuesCardGrid({ issues }: { issues: Issue[] }) {
  const sortBy = useViewStore((s) => s.sortBy);
  const sortDirection = useViewStore((s) => s.sortDirection);

  const sorted = useMemo(
    () => sortIssues(issues, sortBy, sortDirection),
    [issues, sortBy, sortDirection],
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
      <div className="mx-auto w-full max-w-[1680px] px-3 pb-6 pt-2 sm:px-5">
        {sorted.length > 0 ? (
          <div
            className="grid grid-cols-1 items-stretch gap-3 sm:grid-cols-2 sm:gap-4 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5"
            data-testid="issues-card-grid"
          >
            {sorted.map((issue) => (
              <IssueGalleryCard key={issue.id} issue={issue} />
            ))}
          </div>
        ) : (
          <p className="py-20 text-center text-sm text-muted-foreground">
            No issues match your filters
          </p>
        )}
      </div>
    </div>
  );
}
