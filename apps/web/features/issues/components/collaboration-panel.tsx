"use client";

import { Bot } from "lucide-react";
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
  const { taskId, agentId } = useTaskAndAgent(issueId);
  const {
    messages,
    setMessages,
    messagesLoading,
    dependencies,
    setDependencies,
    depsLoading,
    checkpoints,
    cpsLoading,
    memories,
    setMemories,
    memLoading,
    checkpointsLoaded,
    setCheckpointsLoaded,
    memoriesLoaded,
    setMemoriesLoaded,
    loadCheckpoints,
    loadMemories,
  } = useCollaborationData(agentId, taskId);

  const depStatuses = useDependencyStatuses(dependencies);

  // Show empty state if no task context
  if (!taskId && !agentId) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <Bot className="h-8 w-8 text-muted-foreground/40 mb-3" />
        <p className="text-sm font-medium text-muted-foreground">
          No active collaboration session
        </p>
        <p className="text-xs text-muted-foreground/70 mt-1">
          Assign an agent to this issue to see collaboration activity.
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
          onMessageSent={(msg) => setMessages((prev) => [...prev, msg])}
        />
      )}

      {taskId && (
        <DependenciesSection
          taskId={taskId}
          dependencies={dependencies}
          depsLoading={depsLoading}
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
