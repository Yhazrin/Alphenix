"use client";

import { useState, useCallback, useRef, useEffect } from "react";
import { useRouter } from "next/navigation";
import { ArrowRight } from "lucide-react";
import { api } from "@/shared/api";
import type { Issue, IssueStatus } from "@/shared/types";
import {
  CommandDialog,
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandGroup,
  CommandItem,
} from "@/components/ui/command";
import { StatusIcon } from "@/features/issues/components";

export function SearchModal({ onClose }: { onClose: () => void }) {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Issue[]>([]);
  const [loading, setLoading] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const doSearch = useCallback(async (q: string) => {
    if (!q.trim()) {
      setResults([]);
      return;
    }
    setLoading(true);
    try {
      const res = await api.searchIssues(q.trim(), 20);
      setResults(res.issues ?? []);
    } catch {
      setResults([]);
    } finally {
      setLoading(false);
    }
  }, []);

  const handleChange = useCallback(
    (value: string) => {
      setQuery(value);
      if (debounceRef.current) clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(() => doSearch(value), 200);
    },
    [doSearch],
  );

  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, []);

  const goToIssue = (issueId: string) => {
    onClose();
    router.push(`/issues/${issueId}`);
  };

  return (
    <CommandDialog open onOpenChange={(v) => { if (!v) onClose(); }} title="Search issues" description="Search issues by title, description, or number">
      <CommandInput
        placeholder="Search issues by title, description, or number..."
        value={query}
        onValueChange={handleChange}
        autoFocus
      />
      <CommandList>
        {!query.trim() && !loading && (
          <CommandEmpty>Type to search issues</CommandEmpty>
        )}
        {query.trim() && !loading && results.length === 0 && (
          <CommandEmpty>No issues found</CommandEmpty>
        )}
        {loading && query.trim() && (
          <CommandEmpty>Searching...</CommandEmpty>
        )}
        {results.length > 0 && (
          <CommandGroup heading="Issues">
            {results.map((issue) => (
              <CommandItem
                key={issue.id}
                value={`${issue.identifier} ${issue.title}`}
                onSelect={() => goToIssue(issue.id)}
                className="flex items-center gap-2 cursor-pointer"
              >
                <StatusIcon
                  status={issue.status as IssueStatus}
                  className="size-4 shrink-0"
                />
                <span className="text-muted-foreground font-mono text-xs shrink-0">
                  {issue.identifier}
                </span>
                <span className="truncate">{issue.title}</span>
                <ArrowRight className="ml-auto size-3.5 text-muted-foreground opacity-0 group-data-[selected=true]/command-item:opacity-100" />
              </CommandItem>
            ))}
          </CommandGroup>
        )}
      </CommandList>
    </CommandDialog>
  );
}
