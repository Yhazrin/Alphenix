"use client";

import { useState, useCallback, useEffect } from "react";
import { GitBranch } from "lucide-react";
import type { WorkspaceRepo, UpdateIssueRequest } from "@/shared/types";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
import {
  PropertyPicker,
  PickerItem,
  PickerEmpty,
} from "./property-picker";

export function RepoPicker({
  repoId,
  onUpdate,
  align = "start",
}: {
  repoId: string | null;
  onUpdate: (updates: Partial<UpdateIssueRequest>) => void;
  align?: "start" | "center" | "end";
}) {
  const [open, setOpen] = useState(false);
  const [repos, setRepos] = useState<WorkspaceRepo[]>([]);
  const workspace = useWorkspaceStore((s) => s.workspace);

  const loadRepos = useCallback(async () => {
    if (!workspace || !open) return;
    try {
      const data = await api.listWorkspaceRepos(workspace.id);
      setRepos(data);
    } catch { /* silent */ }
  }, [workspace, open]);

  useEffect(() => { loadRepos(); }, [loadRepos]);

  const current = repos.find((r) => r.id === repoId);

  const handleSelect = (id: string | null) => {
    onUpdate({ repo_id: id });
    setOpen(false);
  };

  return (
    <PropertyPicker
      open={open}
      onOpenChange={setOpen}
      width="w-56"
      align={align}
      trigger={
        current ? (
          <>
            <GitBranch className="h-3 w-3 shrink-0 text-muted-foreground" aria-hidden="true" />
            <span className="truncate">{current.name}</span>
          </>
        ) : (
          <>
            <GitBranch className="h-3 w-3 shrink-0 text-muted-foreground" aria-hidden="true" />
            <span className="text-muted-foreground">No repo</span>
          </>
        )
      }
    >
      <PickerItem selected={repoId === null} onClick={() => handleSelect(null)}>
        <span className="text-muted-foreground">No repo</span>
      </PickerItem>
      {repos.length === 0 ? (
        <PickerEmpty />
      ) : (
        repos.map((repo) => (
          <PickerItem
            key={repo.id}
            selected={repoId === repo.id}
            onClick={() => handleSelect(repo.id)}
          >
            <GitBranch className="h-3 w-3 shrink-0 text-muted-foreground" aria-hidden="true" />
            <span className="truncate">{repo.name}</span>
            {repo.is_default && (
              <span className="text-[10px] text-muted-foreground ml-auto">default</span>
            )}
          </PickerItem>
        ))
      )}
    </PropertyPicker>
  );
}
