import { useState, useEffect } from "react";
import { api } from "@/shared/api";
import type { AgentTask } from "@/shared/types";

export function useTaskAndAgent(issueId: string) {
  const [activeTask, setActiveTask] = useState<AgentTask | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    api
      .getActiveTaskForIssue(issueId)
      .then(({ task }) => {
        if (!cancelled) {
          setActiveTask(task);
        }
      })
      .catch(() => {})
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
  };
}
