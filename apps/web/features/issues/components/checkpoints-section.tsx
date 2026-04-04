"use client";

import { CheckCircle2, Clock } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { timeAgo } from "@/shared/utils";
import type { TaskCheckpoint } from "@/shared/types";
import { CollapsibleSection } from "./collapsible-section";

interface CheckpointsSectionProps {
  taskId: string;
  checkpoints: TaskCheckpoint[];
  cpsLoading: boolean;
  checkpointsLoaded: boolean;
  onLoadCheckpoints: () => void;
  onSetCheckpointsLoaded: (loaded: boolean) => void;
}

export function CheckpointsSection({
  taskId,
  checkpoints,
  cpsLoading,
  checkpointsLoaded,
  onLoadCheckpoints,
  onSetCheckpointsLoaded,
}: CheckpointsSectionProps) {
  return (
    <CollapsibleSection
      title="Checkpoints"
      icon={<CheckCircle2 className="h-3.5 w-3.5 text-muted-foreground" />}
      count={checkpoints.length}
      onOpen={() => {
        if (!checkpointsLoaded) {
          onSetCheckpointsLoaded(true);
          onLoadCheckpoints();
        }
      }}
    >
      {cpsLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-6 w-full" />
        </div>
      ) : checkpoints.length === 0 ? (
        <p className="text-xs text-muted-foreground py-1">No checkpoints saved.</p>
      ) : (
        <div className="space-y-1.5">
          {checkpoints.map((cp) => (
            <div key={cp.id} className="flex items-center gap-2 text-xs">
              <CheckCircle2 className="h-3 w-3 text-success shrink-0" />
              <span className="font-medium truncate">{cp.label}</span>
              <span className="text-muted-foreground flex items-center gap-1">
                <Clock className="h-2.5 w-2.5" />
                {timeAgo(cp.created_at)}
              </span>
            </div>
          ))}
        </div>
      )}
    </CollapsibleSection>
  );
}
