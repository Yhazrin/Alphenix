"use client";

import { useCallback } from "react";
import { useWorkspaceStore } from "./store";

export function useActorName() {
  const members = useWorkspaceStore((s) => s.members);
  const agents = useWorkspaceStore((s) => s.agents);

  const getMemberName = useCallback(
    (userId: string) => {
      const m = members.find((m) => m.user_id === userId);
      return m?.name ?? "Unknown";
    },
    [members],
  );

  const getAgentName = useCallback(
    (agentId: string) => {
      const a = agents.find((a) => a.id === agentId);
      return a?.name ?? "Unknown Agent";
    },
    [agents],
  );

  const getActorName = useCallback(
    (type: string, id: string) => {
      if (type === "member") return getMemberName(id);
      if (type === "agent") return getAgentName(id);
      return "System";
    },
    [getMemberName, getAgentName],
  );

  const getActorInitials = useCallback(
    (type: string, id: string) => {
      const name = getActorName(type, id);
      return name
        .split(" ")
        .filter((w) => w.length > 0)
        .map((w) => w[0])
        .join("")
        .toUpperCase()
        .slice(0, 2);
    },
    [getActorName],
  );

  const getActorAvatarUrl = useCallback(
    (type: string, id: string): string | null => {
      if (type === "member") return members.find((m) => m.user_id === id)?.avatar_url ?? null;
      if (type === "agent") return agents.find((a) => a.id === id)?.avatar_url ?? null;
      return null;
    },
    [members, agents],
  );

  return { getMemberName, getAgentName, getActorName, getActorInitials, getActorAvatarUrl };
}
