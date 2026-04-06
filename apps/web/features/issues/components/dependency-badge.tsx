"use client";

import { useState, useEffect, useMemo } from "react";
import Link from "next/link";
import { GitBranch } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { api } from "@/shared/api";
import { useIssueStore } from "@/features/issues/store";
import type { IssueDependency } from "@/shared/types";

interface DependencyBadgesProps {
  issueId: string;
}

export function DependencyBadges({ issueId }: DependencyBadgesProps) {
  const [dependencies, setDependencies] = useState<IssueDependency[]>([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    api.listIssueDependencies(issueId).then(setDependencies).catch(() => {}).finally(() => setLoaded(true));
  }, [issueId]);

  const depIdentifierMap = useMemo(() => {
    const issues = useIssueStore.getState().issues;
    const map: Record<string, string> = {};
    for (const issue of issues) {
      map[issue.id] = issue.identifier;
    }
    return map;
  }, [dependencies]);

  if (!loaded || dependencies.length === 0) return null;

  return (
    <span className="flex items-center gap-1 shrink-0">
      {dependencies.map((dep) => (
        <Link key={dep.id} href={`/issues/${dep.depends_on_issue_id}`}>
          <Badge
            variant="outline"
            className="h-4 px-1.5 text-[10px] gap-0.5 cursor-pointer hover:bg-muted transition-colors"
          >
            <GitBranch className="h-2.5 w-2.5" aria-hidden="true" />
            {depIdentifierMap[dep.depends_on_issue_id] ?? dep.depends_on_issue_id.slice(0, 8)}
          </Badge>
        </Link>
      ))}
    </span>
  );
}
