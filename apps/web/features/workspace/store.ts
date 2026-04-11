"use client";

import { create } from "zustand";
import type { Workspace, Agent, Skill, MemberWithUser } from "@/shared/types";
import { toast } from "sonner";
import { api } from "@/shared/api";
import { createLogger } from "@/shared/logger";
import {
  clearWorkspaceIdFromStorage,
  persistWorkspaceIdToStorage,
  readStoredWorkspaceId,
} from "@/shared/constants/workspace-storage";

const logger = createLogger("workspace-store");

interface WorkspaceState {
  workspace: Workspace | null;
  workspaces: Workspace[];
  // Stub properties for backwards compatibility
  // Actual data is managed by TanStack Query in @core/workspace
  members: MemberWithUser[];
  agents: Agent[];
  skills: Skill[];
}

interface WorkspaceActions {
  hydrateWorkspace: (
    wsList: Workspace[],
    preferredWorkspaceId?: string | null,
  ) => Workspace | null;
  switchWorkspace: (workspaceId: string) => void;
  refreshWorkspaces: () => Promise<Workspace[]>;
  updateWorkspace: (ws: Workspace) => void;
  createWorkspace: (data: {
    name: string;
    slug: string;
    description?: string;
  }) => Promise<Workspace>;
  leaveWorkspace: (workspaceId: string) => Promise<void>;
  deleteWorkspace: (workspaceId: string) => Promise<void>;
  clearWorkspace: () => void;
  // Stub actions for backwards compatibility
  refreshAgents: () => Promise<void>;
  refreshSkills: () => Promise<void>;
  refreshMembers: () => Promise<void>;
  updateAgent: (id: string, updates: Partial<Agent>) => void;
  upsertSkill: (skill: Skill) => void;
  removeSkill: (id: string) => void;
}

type WorkspaceStore = WorkspaceState & WorkspaceActions;

export const useWorkspaceStore = create<WorkspaceStore>((set, get) => ({
  // State
  workspace: null,
  workspaces: [],
  members: [],
  agents: [],
  skills: [],

  // Actions
  hydrateWorkspace: (wsList, preferredWorkspaceId) => {
    set({ workspaces: wsList });

    const nextWorkspace =
      (preferredWorkspaceId
        ? wsList.find((item) => item.id === preferredWorkspaceId)
        : null) ??
      wsList[0] ??
      null;

    if (!nextWorkspace) {
      api.setWorkspaceId(null);
      clearWorkspaceIdFromStorage();
      set({ workspace: null, members: [], agents: [], skills: [] });
      return null;
    }

    api.setWorkspaceId(nextWorkspace.id);
    persistWorkspaceIdToStorage(nextWorkspace.id);
    set({ workspace: nextWorkspace });
    logger.debug("hydrate workspace", nextWorkspace.name, nextWorkspace.id);

    // Members, agents, skills, issues, inbox are all managed by TanStack Query.
    // They auto-fetch when components mount with the workspace ID in their query key.

    return nextWorkspace;
  },

  switchWorkspace: (workspaceId) => {
    logger.info("switching to", workspaceId);
    const { workspaces, hydrateWorkspace } = get();
    const ws = workspaces.find((item) => item.id === workspaceId);
    if (!ws) return;

    api.setWorkspaceId(ws.id);
    persistWorkspaceIdToStorage(ws.id);

    // All data caches (issues, inbox, members, agents, skills, runtimes)
    // are managed by TanStack Query, keyed by wsId — auto-refetch on switch.
    set({ workspace: ws, members: [], agents: [], skills: [] });

    hydrateWorkspace(workspaces, ws.id);
  },

  refreshWorkspaces: async () => {
    const { workspace, hydrateWorkspace } = get();
    const storedWorkspaceId = readStoredWorkspaceId();
    try {
      const wsList = await api.listWorkspaces();
      hydrateWorkspace(wsList, workspace?.id ?? storedWorkspaceId);
      return wsList;
    } catch (e) {
      logger.error("failed to refresh workspaces", e);
      toast.error("Failed to refresh workspaces");
      return get().workspaces;
    }
  },

  updateWorkspace: (ws) => {
    set((state) => ({
      workspace: state.workspace?.id === ws.id ? ws : state.workspace,
      workspaces: state.workspaces.map((item) =>
        item.id === ws.id ? ws : item,
      ),
    }));
  },

  createWorkspace: async (data) => {
    const ws = await api.createWorkspace(data);
    set((state) => ({ workspaces: [...state.workspaces, ws] }));
    return ws;
  },

  leaveWorkspace: async (workspaceId) => {
    await api.leaveWorkspace(workspaceId);
    const { workspace, hydrateWorkspace } = get();
    const wsList = await api.listWorkspaces();
    const preferredWorkspaceId =
      workspace?.id === workspaceId ? null : (workspace?.id ?? null);
    hydrateWorkspace(wsList, preferredWorkspaceId);
  },

  deleteWorkspace: async (workspaceId) => {
    await api.deleteWorkspace(workspaceId);
    const { workspace, hydrateWorkspace } = get();
    const wsList = await api.listWorkspaces();
    const preferredWorkspaceId =
      workspace?.id === workspaceId ? null : (workspace?.id ?? null);
    hydrateWorkspace(wsList, preferredWorkspaceId);
  },

  clearWorkspace: () => {
    api.setWorkspaceId(null);
    clearWorkspaceIdFromStorage();
    set({ workspace: null, workspaces: [], members: [], agents: [], skills: [] });
  },

  // Stub actions for backwards compatibility
  refreshAgents: async () => {
    // Actual implementation uses TanStack Query
  },

  refreshSkills: async () => {
    // Actual implementation uses TanStack Query
  },

  refreshMembers: async () => {
    // Actual implementation uses TanStack Query
  },

  updateAgent: (id, updates) =>
    set((state) => ({
      agents: state.agents.map((a) =>
        a.id === id ? { ...a, ...updates } : a,
      ),
    })),

  upsertSkill: (skill) =>
    set((state) => {
      const idx = state.skills.findIndex((s) => s.id === skill.id);
      if (idx >= 0) {
        const next = [...state.skills];
        next[idx] = skill;
        return { skills: next };
      }
      return { skills: [...state.skills, skill] };
    }),

  removeSkill: (id) =>
    set((state) => ({
      skills: state.skills.filter((s) => s.id !== id),
    })),
}));
