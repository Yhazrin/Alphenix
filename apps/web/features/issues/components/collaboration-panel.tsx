"use client";

import { AlertCircle, Bot } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { useTaskAndAgent } from "../hooks/use-task-and-agent";
import { useDependencyStatuses } from "../hooks/use-dependency-statuses";
import { useCollaborationData } from "../hooks/use-collaboration-data";
import { AgentMessagesSection } from "./agent-messages-section";
import { DependenciesSection } from "./dependencies-section";
import { CheckpointsSection } from "./checkpoints-section";
import { ReviewSection } from "./review-section";
import { ChainTaskSection } from "./chain-task-section";
import { MemorySection } from "./memory-section";

interface CollaborationPanelProps {
  issueId: string;
}

export function CollaborationPanel({ issueId }: CollaborationPanelProps) {
  const { taskId, agentId, error, loading: taskContextLoading } = useTaskAndAgent(issueId);
  const {
    messages,
    setMessages,
    messagesLoading,
    messagesError,
    dependencies,
    setDependencies,
    depsLoading,
    depsError,
    checkpoints,
    cpsLoading,
    cpsError,
    memories,
    setMemories,
    memLoading,
    memError,
    checkpointsLoaded,
    setCheckpointsLoaded,
    memoriesLoaded,
    setMemoriesLoaded,
    loadCheckpoints,
    loadMemories,
  } = useCollaborationData(agentId, taskId);

  const depStatuses = useDependencyStatuses(dependencies);

  if (taskContextLoading) {
    return (
      <div
        className="flex flex-col gap-3 py-1"
        aria-busy="true"
        aria-label="Loading collaboration"
      >
        <Skeleton className="h-20 w-full rounded-xl" />
        <Skeleton className="h-12 w-full rounded-lg" />
      </div>
    );
  }

  // Show empty state if no task context
  if (!taskId && !agentId) {
    if (error) {
      return (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-destructive/25 bg-destructive/5 px-4 py-10 text-center">
          <AlertCircle className="mb-3 h-8 w-8 text-destructive/60" aria-hidden="true" />
          <p className="text-sm font-medium text-destructive">
            Could not load agent task context
          </p>
          <p className="mt-1 max-w-sm text-xs text-muted-foreground">
            Comments and activity below still work. Try refreshing the page if this continues.
          </p>
          <p className="mt-2 font-mono text-[10px] text-destructive/70 break-all">
            {error}
          </p>
        </div>
      );
    }

    return (
      <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-muted-foreground/20 bg-muted/20 px-4 py-10 text-center">
        <Bot className="mb-3 h-8 w-8 text-muted-foreground/40" aria-hidden="true" />
        <p className="text-sm font-medium text-muted-foreground">
          No active agent task
        </p>
        <p className="mt-1 max-w-xs text-xs text-muted-foreground/80">
          When an agent is working on this issue, messages, checkpoints, and dependencies appear here.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2.5">
      {agentId && (
        <AgentMessagesSection
          agentId={agentId}
          taskId={taskId}
          messages={messages}
          messagesLoading={messagesLoading}
          messagesError={messagesError}
          onMessageSent={(msg) => setMessages((prev) => [...prev, msg])}
        />
      )}

      {taskId && (
        <DependenciesSection
          taskId={taskId}
          dependencies={dependencies}
          depsLoading={depsLoading}
          depsError={depsError}
          depStatuses={depStatuses}
          onDependencyAdded={(dep) => setDependencies((prev) => [...prev, dep])}
          onDependencyRemoved={(dependsOnId) =>
            setDependencies((prev) => prev.filter((d) => d.depends_on_id !== dependsOnId))
          }
        />
      )}

      {taskId && (
        <CheckpointsSection
          taskId={taskId}
          checkpoints={checkpoints}
          cpsLoading={cpsLoading}
          cpsError={cpsError}
          checkpointsLoaded={checkpointsLoaded}
          onLoadCheckpoints={loadCheckpoints}
          onSetCheckpointsLoaded={setCheckpointsLoaded}
        />
      )}

      {taskId && <ReviewSection taskId={taskId} />}

      {taskId && <ChainTaskSection taskId={taskId} />}

      {agentId && (
        <MemorySection
          agentId={agentId}
          memories={memories}
          memLoading={memLoading}
          memError={memError}
          memoriesLoaded={memoriesLoaded}
          onLoadMemories={loadMemories}
          onSetMemoriesLoaded={setMemoriesLoaded}
          onMemoryStored={(mem) => setMemories((prev) => [...prev, mem])}
          onMemoryDeleted={(memoryId) =>
            setMemories((prev) => prev.filter((m) => m.id !== memoryId))
          }
        />
      )}
    </div>
  );
}
