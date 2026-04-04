import { useState, useCallback, useEffect, useRef } from "react";
import { api } from "@/shared/api";
import { useWSEvent } from "@/features/realtime";
import { toast } from "sonner";
import type {
  AgentMessage,
  TaskDependency,
  TaskCheckpoint,
  AgentMemory,
} from "@/shared/types";

function useDebouncedCallback(cb: () => void, delay: number) {
  const timer = useRef<ReturnType<typeof setTimeout>>(null);
  const savedCb = useRef(cb);
  savedCb.current = cb;

  useEffect(() => () => { if (timer.current) clearTimeout(timer.current); }, []);

  return useCallback(() => {
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => savedCb.current(), delay);
  }, [delay]);
}

export function useCollaborationData(
  agentId: string | undefined,
  taskId: string | undefined,
) {
  const [messages, setMessages] = useState<AgentMessage[]>([]);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [dependencies, setDependencies] = useState<TaskDependency[]>([]);
  const [depsLoading, setDepsLoading] = useState(false);
  const [checkpoints, setCheckpoints] = useState<TaskCheckpoint[]>([]);
  const [cpsLoading, setCpsLoading] = useState(false);
  const [memories, setMemories] = useState<AgentMemory[]>([]);
  const [memLoading, setMemLoading] = useState(false);
  const [checkpointsLoaded, setCheckpointsLoaded] = useState(false);
  const [memoriesLoaded, setMemoriesLoaded] = useState(false);

  const loadMessages = useCallback(async () => {
    if (!agentId) return;
    setMessagesLoading(true);
    try {
      const data = await api.listAgentMessages(agentId, taskId ? { task_id: taskId } : undefined);
      setMessages(data);
    } catch {
      toast.error("Failed to load messages");
    } finally {
      setMessagesLoading(false);
    }
  }, [agentId, taskId]);

  const loadDependencies = useCallback(async () => {
    if (!taskId) return;
    setDepsLoading(true);
    try {
      const data = await api.listTaskDependencies(taskId);
      setDependencies(data);
    } catch {
      toast.error("Failed to load dependencies");
    } finally {
      setDepsLoading(false);
    }
  }, [taskId]);

  const loadCheckpoints = useCallback(async () => {
    if (!taskId) return;
    setCpsLoading(true);
    try {
      const data = await api.listTaskCheckpoints(taskId);
      setCheckpoints(data);
    } catch {
      toast.error("Failed to load checkpoints");
    } finally {
      setCpsLoading(false);
    }
  }, [taskId]);

  const loadMemories = useCallback(async () => {
    if (!agentId) return;
    setMemLoading(true);
    try {
      const data = await api.listAgentMemory(agentId);
      setMemories(data);
    } catch {
      toast.error("Failed to load memories");
    } finally {
      setMemLoading(false);
    }
  }, [agentId]);

  // Lazy-load: only load messages and dependencies on mount; others on section open
  useEffect(() => {
    loadMessages();
    loadDependencies();
  }, [loadMessages, loadDependencies]);

  // Debounced WS handlers — 200ms delay to avoid firehose reload
  const debouncedMessages = useDebouncedCallback(loadMessages, 200);
  const debouncedDeps = useDebouncedCallback(loadDependencies, 200);
  const debouncedCheckpoints = useDebouncedCallback(loadCheckpoints, 200);
  const debouncedMemories = useDebouncedCallback(loadMemories, 200);

  useWSEvent("agent:message", debouncedMessages);
  useWSEvent("task_dep:created", debouncedDeps);
  useWSEvent("task_dep:deleted", debouncedDeps);
  useWSEvent("task:checkpoint", debouncedCheckpoints);
  useWSEvent("memory:stored", debouncedMemories);

  return {
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
  };
}
