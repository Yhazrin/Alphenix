import { useState, useEffect } from "react";
import { api } from "@/shared/api";
import type { AgentTask } from "@/shared/types";

export function useTaskAndAgent(issueId: string) {
  const [activeTask, setActiveTask] = useState<AgentTask | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setError(null);
    api
      .getActiveTasksForIssue(issueId)
      .then(({ tasks }) => {
        if (!cancelled) {
          setActiveTask(tasks[0] ?? null);
        }
      })
      .catch((e: unknown) => {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : "Failed to load task");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [issueId]);

  return {
    activeTask,
    taskId: activeTask?.id,
    agentId: activeTask?.agent_id,
    loading,
    error,
  };
}
