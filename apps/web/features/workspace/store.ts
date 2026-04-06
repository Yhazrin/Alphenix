"use client";

import { create } from "zustand";
import type { Workspace, MemberWithUser, Agent, Skill } from "@/shared/types";
import { useIssueStore } from "@/features/issues";
import { useInboxStore } from "@/features/inbox";
import { toast } from "sonner";
import { api } from "@/shared/api";
import { configureAgentsApi } from "@/shared/api/agents";
import { configureTasksApi } from "@/shared/api/tasks";
import { configureRuntimesApi } from "@/shared/api/runtimes";
import { createLogger } from "@/shared/logger";
import { resetStores } from "./orchestration";

const logger = createLogger("workspace-store");

interface WorkspaceState {
  workspace: Workspace | null;
  workspaces: Workspace[];
  members: MemberWithUser[];
  agents: Agent[];
  skills: Skill[];
}

interface WorkspaceActions {
  hydrateWorkspace: (
    wsList: Workspace[],
    preferredWorkspaceId?: string | null,
  ) => Promise<Workspace | null>;
  switchWorkspace: (workspaceId: string) => Promise<void>;
  refreshWorkspaces: () => Promise<Workspace[]>;
  refreshMembers: () => Promise<void>;
  updateAgent: (id: string, updates: Partial<Agent>) => void;
  refreshAgents: () => Promise<void>;
  refreshSkills: () => Promise<void>;
  upsertSkill: (skill: Skill) => void;
  removeSkill: (id: string) => void;
  createWorkspace: (data: {
    name: string;
    slug: string;
    description?: string;
  }) => Promise<Workspace>;
  updateWorkspace: (ws: Workspace) => void;
  leaveWorkspace: (workspaceId: string) => Promise<void>;
  deleteWorkspace: (workspaceId: string) => Promise<void>;
  clearWorkspace: () => void;
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
  hydrateWorkspace: async (wsList, preferredWorkspaceId) => {
    set({ workspaces: wsList });

    const nextWorkspace =
      (preferredWorkspaceId
        ? wsList.find((item) => item.id === preferredWorkspaceId)
        : null) ??
      wsList[0] ??
      null;

    if (!nextWorkspace) {
      api.setWorkspaceId(null);
      configureAgentsApi({ workspaceId: null });
      configureTasksApi({ workspaceId: null });
      configureRuntimesApi({ workspaceId: null });
      try { localStorage.removeItem("alphenix_workspace_id"); } catch {}
      set({ workspace: null, members: [], agents: [], skills: [] });
      return null;
    }

    api.setWorkspaceId(nextWorkspace.id);
    configureAgentsApi({ workspaceId: nextWorkspace.id });
    configureTasksApi({ workspaceId: nextWorkspace.id });
    configureRuntimesApi({ workspaceId: nextWorkspace.id });
    try { localStorage.setItem("alphenix_workspace_id", nextWorkspace.id); } catch {}
    set({ workspace: nextWorkspace });

    logger.debug("hydrate workspace", nextWorkspace.name, nextWorkspace.id);
    // Fire-and-forget fetches for other stores
    useIssueStore.getState().fetch().catch((e) => console.error("Failed to fetch issues:", e));
    useInboxStore.getState().fetch().catch((e) => console.error("Failed to fetch inbox:", e));
    const [nextMembers, nextAgents, nextSkills] = await Promise.all([
      api.listMembers(nextWorkspace.id).catch((e) => {
        logger.error("failed to load members", e);
        toast.error("Failed to load members");
        return [] as MemberWithUser[];
      }),
      api.listAgents({ workspace_id: nextWorkspace.id, include_archived: true }).catch((e) => {
        logger.error("failed to load agents", e);
        toast.error("Failed to load agents");
        return [] as Agent[];
      }),
      api.listSkills().catch((e) => {
        logger.error("failed to load skills", e);
        toast.error("Failed to load skills");
        return [] as Skill[];
      }),
    ]);
    logger.info("hydrate complete", "members:", nextMembers.length, "agents:", nextAgents.length);
    set({ members: nextMembers, agents: nextAgents, skills: nextSkills });

    return nextWorkspace;
  },

  switchWorkspace: async (workspaceId) => {
    logger.info("switching to", workspaceId);
    const { workspaces, hydrateWorkspace } = get();
    const ws = workspaces.find((item) => item.id === workspaceId);
    if (!ws) return;

    // Switch identity FIRST — api client, localStorage, and the
    // workspace object in this store — so that any in-flight refetch
    // (e.g. triggered by a WS event during the async gap) already
    // targets the new workspace.
    api.setWorkspaceId(ws.id);
    configureAgentsApi({ workspaceId: ws.id });
    configureTasksApi({ workspaceId: ws.id });
    configureRuntimesApi({ workspaceId: ws.id });
    try { localStorage.setItem("alphenix_workspace_id", ws.id); } catch {}

    // Clear ALL stale data across every store before hydrating.
    resetStores();
    set({ workspace: ws, members: [], agents: [], skills: [] });

    await hydrateWorkspace(workspaces, ws.id);
  },

  refreshWorkspaces: async () => {
    const { workspace, hydrateWorkspace } = get();
    let storedWorkspaceId: string | null = null;
    try { storedWorkspaceId = localStorage.getItem("alphenix_workspace_id"); } catch {}
    try {
      const wsList = await api.listWorkspaces();
      await hydrateWorkspace(wsList, workspace?.id ?? storedWorkspaceId);
      return wsList;
    } catch (e) {
      logger.error("failed to refresh workspaces", e);
      toast.error("Failed to refresh workspaces");
      return get().workspaces;
    }
  },

  refreshMembers: async () => {
    const { workspace } = get();
    if (!workspace) return;
    try {
      const members = await api.listMembers(workspace.id);
      set({ members });
    } catch (e) {
      logger.error("failed to refresh members", e);
      toast.error("Failed to refresh members");
    }
  },

  updateAgent: (id, updates) =>
    set((s) => ({
      agents: s.agents.map((a) => (a.id === id ? { ...a, ...updates } : a)),
    })),

  refreshAgents: async () => {
    const { workspace } = get();
    if (!workspace) return;
    try {
      const agents = await api.listAgents({ workspace_id: workspace.id, include_archived: true });
      set({ agents });
    } catch (e) {
      logger.error("failed to refresh agents", e);
      toast.error("Failed to refresh agents");
    }
  },

  refreshSkills: async () => {
    const { workspace, skills: existing } = get();
    if (!workspace) return;
    try {
      const fetched = await api.listSkills();
      // listSkills doesn't include files — preserve files from existing entries
      const filesById = new Map(
        existing.filter((s) => s.files?.length).map((s) => [s.id, s.files]),
      );
      const merged = fetched.map((s) => ({
        ...s,
        files: s.files ?? filesById.get(s.id) ?? [],
      }));
      set({ skills: merged });
    } catch (e) {
      logger.error("failed to refresh skills", e);
      toast.error("Failed to refresh skills");
    }
  },

  upsertSkill: (skill) => {
    set((state) => {
      const idx = state.skills.findIndex((s) => s.id === skill.id);
      if (idx >= 0) {
        const next = [...state.skills];
        next[idx] = skill;
        return { skills: next };
      }
      return { skills: [...state.skills, skill] };
    });
  },

  removeSkill: (id) => {
    set((state) => ({ skills: state.skills.filter((s) => s.id !== id) }));
  },

  createWorkspace: async (data) => {
    try {
      const ws = await api.createWorkspace(data);
      set((state) => ({ workspaces: [...state.workspaces, ws] }));
      return ws;
    } catch (e) {
      logger.error("failed to create workspace", e);
      toast.error("Failed to create workspace");
      throw e;
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

  leaveWorkspace: async (workspaceId) => {
    try {
      await api.leaveWorkspace(workspaceId);
      const { workspace, hydrateWorkspace } = get();
      const wsList = await api.listWorkspaces();
      const preferredWorkspaceId =
        workspace?.id === workspaceId ? null : (workspace?.id ?? null);
      await hydrateWorkspace(wsList, preferredWorkspaceId);
    } catch (e) {
      logger.error("failed to leave workspace", e);
      toast.error("Failed to leave workspace");
      throw e;
    }
  },

  deleteWorkspace: async (workspaceId) => {
    try {
      await api.deleteWorkspace(workspaceId);
      const { workspace, hydrateWorkspace } = get();
      const wsList = await api.listWorkspaces();
      const preferredWorkspaceId =
        workspace?.id === workspaceId ? null : (workspace?.id ?? null);
      await hydrateWorkspace(wsList, preferredWorkspaceId);
    } catch (e) {
      logger.error("failed to delete workspace", e);
      toast.error("Failed to delete workspace");
      throw e;
    }
  },

  clearWorkspace: () => {
    api.setWorkspaceId(null);
    configureAgentsApi({ workspaceId: null });
    configureTasksApi({ workspaceId: null });
    configureRuntimesApi({ workspaceId: null });
    try { localStorage.removeItem("alphenix_workspace_id"); } catch {}
    set({ workspace: null, workspaces: [], members: [], agents: [], skills: [] });
  },
}));
