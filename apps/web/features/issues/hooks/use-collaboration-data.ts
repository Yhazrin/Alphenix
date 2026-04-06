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
  const [messagesError, setMessagesError] = useState<string | null>(null);
  const [dependencies, setDependencies] = useState<TaskDependency[]>([]);
  const [depsLoading, setDepsLoading] = useState(false);
  const [depsError, setDepsError] = useState<string | null>(null);
  const [checkpoints, setCheckpoints] = useState<TaskCheckpoint[]>([]);
  const [cpsLoading, setCpsLoading] = useState(false);
  const [cpsError, setCpsError] = useState<string | null>(null);
  const [memories, setMemories] = useState<AgentMemory[]>([]);
  const [memLoading, setMemLoading] = useState(false);
  const [memError, setMemError] = useState<string | null>(null);
  const [checkpointsLoaded, setCheckpointsLoaded] = useState(false);
  const [memoriesLoaded, setMemoriesLoaded] = useState(false);

  const loadMessages = useCallback(async () => {
    if (!agentId) return;
    let cancelled = false;
    setMessagesLoading(true);
    setMessagesError(null);
    try {
      const data = await api.listAgentMessages(agentId, taskId ? { task_id: taskId } : undefined);
      if (cancelled) return;
      setMessages(data);
      setMessagesError(null);
      api.markAgentMessagesRead(agentId).catch((e) => console.error("markAgentMessagesRead failed:", e));
    } catch (e) {
      if (cancelled) return;
      const message = e instanceof Error ? e.message : "Failed to load messages";
      setMessagesError(message);
      toast.error("Failed to load messages");
    } finally {
      if (!cancelled) setMessagesLoading(false);
    }
    return () => { cancelled = true; };
  }, [agentId, taskId]);

  const loadDependencies = useCallback(async () => {
    if (!taskId) return;
    let cancelled = false;
    setDepsLoading(true);
    setDepsError(null);
    try {
      const data = await api.listTaskDependencies(taskId);
      if (cancelled) return;
      setDependencies(data);
      setDepsError(null);
    } catch (e) {
      if (cancelled) return;
      const message = e instanceof Error ? e.message : "Failed to load dependencies";
      setDepsError(message);
      toast.error("Failed to load dependencies");
    } finally {
      if (!cancelled) setDepsLoading(false);
    }
    return () => { cancelled = true; };
  }, [taskId]);

  const loadCheckpoints = useCallback(async () => {
    if (!taskId) return;
    let cancelled = false;
    setCpsLoading(true);
    setCpsError(null);
    try {
      const data = await api.listTaskCheckpoints(taskId);
      if (cancelled) return;
      setCheckpoints(data);
      setCheckpointsLoaded(true);
      setCpsError(null);
    } catch (e) {
      if (cancelled) return;
      const message = e instanceof Error ? e.message : "Failed to load checkpoints";
      setCpsError(message);
      toast.error("Failed to load checkpoints");
    } finally {
      if (!cancelled) setCpsLoading(false);
    }
    return () => { cancelled = true; };
  }, [taskId]);

  const loadMemories = useCallback(async () => {
    if (!agentId) return;
    let cancelled = false;
    setMemLoading(true);
    setMemError(null);
    try {
      const data = await api.listAgentMemory(agentId);
      if (cancelled) return;
      setMemories(data);
      setMemoriesLoaded(true);
      setMemError(null);
    } catch (e) {
      if (cancelled) return;
      const message = e instanceof Error ? e.message : "Failed to load memories";
      setMemError(message);
      toast.error("Failed to load memories");
    } finally {
      if (!cancelled) setMemLoading(false);
    }
    return () => { cancelled = true; };
  }, [agentId]);

  // Lazy-load: only load messages and dependencies on mount; others on section open
  useEffect(() => {
    const cleanupMessages = loadMessages();
    const cleanupDeps = loadDependencies();
    return () => {
      cleanupMessages?.then((fn) => fn?.());
      cleanupDeps?.then((fn) => fn?.());
    };
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
  };
}
