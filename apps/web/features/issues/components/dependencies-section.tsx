"use client";

import { useState, useMemo } from "react";
import { GitBranch, Plus, X, Link2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { api } from "@/shared/api";
import { useIssueStore } from "@/features/issues/store";
import { toast } from "sonner";
import { timeAgo } from "@/shared/utils";
import type { TaskDependency } from "@/shared/types";
import { CollapsibleSection } from "./collapsible-section";

interface DependenciesSectionProps {
  taskId: string;
  dependencies: TaskDependency[];
  depsLoading: boolean;
  depStatuses: Record<string, string>;
  onDependencyAdded: (dep: TaskDependency) => void;
  onDependencyRemoved: (dependsOnId: string) => void;
}

export function DependenciesSection({
  taskId,
  dependencies,
  depsLoading,
  depStatuses,
  onDependencyAdded,
  onDependencyRemoved,
}: DependenciesSectionProps) {
  const [showAddDep, setShowAddDep] = useState(false);
  const [addDepTaskId, setAddDepTaskId] = useState("");

  const depIdentifierMap = useMemo(() => {
    const issues = useIssueStore.getState().issues;
    const map: Record<string, string> = {};
    for (const issue of issues) {
      map[issue.id] = issue.identifier;
    }
    return map;
  }, [dependencies]);

  const handleAddDependency = async () => {
    if (!taskId || !addDepTaskId.trim()) return;
    try {
      const dep = await api.addTaskDependency(taskId, {
        depends_on_task_id: addDepTaskId.trim(),
      });
      onDependencyAdded(dep);
      setAddDepTaskId("");
      setShowAddDep(false);
      toast.success("Dependency added");
    } catch {
      toast.error("Failed to add dependency");
    }
  };

  const handleRemoveDependency = async (dependsOnId: string) => {
    if (!taskId) return;
    try {
      await api.removeTaskDependency(taskId, { depends_on_task_id: dependsOnId });
      onDependencyRemoved(dependsOnId);
      toast.success("Dependency removed");
    } catch {
      toast.error("Failed to remove dependency");
    }
  };

  return (
    <CollapsibleSection
      title="Dependencies"
      icon={<GitBranch className="h-3.5 w-3.5 text-muted-foreground" />}
      count={dependencies.length}
    >
      {depsLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-6 w-full" />
        </div>
      ) : dependencies.length === 0 && !showAddDep ? (
        <p className="text-xs text-muted-foreground py-1">No dependencies.</p>
      ) : (
        <div className="space-y-1.5">
          {dependencies.map((dep) => (
            <div key={`${dep.task_id}-${dep.depends_on_id}`} className="flex items-center gap-2 text-xs group">
              <GitBranch className="h-3 w-3 text-muted-foreground shrink-0" />
              <span className="font-mono truncate">
                {depIdentifierMap[dep.depends_on_id] ?? dep.depends_on_id.slice(0, 8)}
              </span>
              {depStatuses[dep.depends_on_id] && (
                <Badge
                  variant="outline"
                  className={cn(
                    "h-4 px-1.5 text-[10px] shrink-0",
                    depStatuses[dep.depends_on_id] === "completed" && "border-success/30 text-success",
                    depStatuses[dep.depends_on_id] === "running" && "border-info/30 text-info",
                    depStatuses[dep.depends_on_id] === "failed" && "border-destructive/30 text-destructive",
                    depStatuses[dep.depends_on_id] === "in_review" && "border-warning/30 text-warning",
                  )}
                >
                  {depStatuses[dep.depends_on_id]}
                </Badge>
              )}
              <span className="text-muted-foreground ml-auto">
                {timeAgo(dep.created_at)}
              </span>
              <Button
                variant="ghost"
                size="icon-sm"
                className="h-5 w-5 p-0 opacity-0 group-hover:opacity-100 transition-opacity"
                onClick={() => handleRemoveDependency(dep.depends_on_id)}
              >
                <X className="h-3 w-3" />
              </Button>
            </div>
          ))}
        </div>
      )}
      {showAddDep ? (
        <div className="mt-2 flex gap-1.5">
          <Input
            value={addDepTaskId}
            onChange={(e) => setAddDepTaskId(e.target.value)}
            placeholder="Task ID..."
            className="h-8 text-xs"
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                handleAddDependency();
              }
              if (e.key === "Escape") {
                setShowAddDep(false);
                setAddDepTaskId("");
              }
            }}
            autoFocus
          />
          <Button
            size="sm"
            variant="ghost"
            className="h-8 w-8 p-0 shrink-0"
            onClick={handleAddDependency}
            disabled={!addDepTaskId.trim()}
          >
            <Link2 className="h-3.5 w-3.5" />
          </Button>
          <Button
            size="sm"
            variant="ghost"
            className="h-8 w-8 p-0 shrink-0"
            onClick={() => {
              setShowAddDep(false);
              setAddDepTaskId("");
            }}
          >
            <X className="h-3.5 w-3.5" />
          </Button>
        </div>
      ) : (
        <Button
          variant="ghost"
          size="sm"
          className="mt-1.5 h-6 text-xs text-muted-foreground w-full"
          onClick={() => setShowAddDep(true)}
        >
          <Plus className="h-3 w-3 mr-1" />
          Add dependency
        </Button>
      )}
    </CollapsibleSection>
  );
}
