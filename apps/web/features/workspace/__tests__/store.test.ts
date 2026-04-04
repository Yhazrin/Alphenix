import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { useWorkspaceStore } from "../store";

// Mock dependencies
vi.mock("@/shared/api", () => {
  const mockApi = {
    setWorkspaceId: vi.fn(),
    listWorkspaces: vi.fn(),
    listMembers: vi.fn(),
    listAgents: vi.fn(),
    listSkills: vi.fn(),
    createWorkspace: vi.fn(),
    leaveWorkspace: vi.fn(),
    deleteWorkspace: vi.fn(),
  };
  return { api: mockApi };
});

vi.mock("@/features/issues", () => ({
  useIssueStore: { getState: () => ({ fetch: vi.fn().mockResolvedValue(undefined), setIssues: vi.fn(), reset: vi.fn() }) },
}));

vi.mock("@/features/inbox", () => ({
  useInboxStore: { getState: () => ({ fetch: vi.fn().mockResolvedValue(undefined), setItems: vi.fn(), reset: vi.fn() }) },
}));

vi.mock("@/features/runtimes", () => ({
  useRuntimeStore: { getState: () => ({ setRuntimes: vi.fn(), reset: vi.fn() }) },
}));

vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));

import { api } from "@/shared/api";

describe("workspace store", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    useWorkspaceStore.setState({
      workspace: null,
      workspaces: [],
      members: [],
      agents: [],
      skills: [],
    });
  });

  describe("hydrateWorkspace", () => {
    it("returns null when workspace list is empty", async () => {
      const result = await useWorkspaceStore.getState().hydrateWorkspace([]);
      expect(result).toBeNull();
      expect(useWorkspaceStore.getState().workspace).toBeNull();
      expect(api.setWorkspaceId).toHaveBeenCalledWith(null);
    });

    it("selects preferred workspace when available", async () => {
      const workspaces = [
        { id: "ws-1", name: "First", slug: "first" },
        { id: "ws-2", name: "Second", slug: "second" },
      ] as any[];

      vi.mocked(api.listMembers).mockResolvedValueOnce([]);
      vi.mocked(api.listAgents).mockResolvedValueOnce([]);
      vi.mocked(api.listSkills).mockResolvedValueOnce([]);

      const result = await useWorkspaceStore.getState().hydrateWorkspace(workspaces, "ws-2");

      expect(result?.id).toBe("ws-2");
      expect(api.setWorkspaceId).toHaveBeenCalledWith("ws-2");
      expect(localStorage.getItem("multicode_workspace_id")).toBe("ws-2");
    });

    it("falls back to first workspace when preferred not found", async () => {
      const workspaces = [
        { id: "ws-1", name: "First", slug: "first" },
      ] as any[];

      vi.mocked(api.listMembers).mockResolvedValueOnce([]);
      vi.mocked(api.listAgents).mockResolvedValueOnce([]);
      vi.mocked(api.listSkills).mockResolvedValueOnce([]);

      const result = await useWorkspaceStore.getState().hydrateWorkspace(workspaces, "nonexistent");

      expect(result?.id).toBe("ws-1");
    });

    it("loads members, agents, and skills in parallel", async () => {
      const workspaces = [{ id: "ws-1", name: "WS", slug: "ws" }] as any[];
      const members = [{ id: "m-1" }] as any[];
      const agents = [{ id: "a-1" }] as any[];
      const skills = [{ id: "s-1" }] as any[];

      vi.mocked(api.listMembers).mockResolvedValueOnce(members);
      vi.mocked(api.listAgents).mockResolvedValueOnce(agents);
      vi.mocked(api.listSkills).mockResolvedValueOnce(skills);

      await useWorkspaceStore.getState().hydrateWorkspace(workspaces);

      expect(api.listMembers).toHaveBeenCalledWith("ws-1");
      expect(api.listAgents).toHaveBeenCalledWith({ workspace_id: "ws-1", include_archived: true });
      expect(useWorkspaceStore.getState().members).toEqual(members);
      expect(useWorkspaceStore.getState().agents).toEqual(agents);
      expect(useWorkspaceStore.getState().skills).toEqual(skills);
    });
  });

  describe("switchWorkspace", () => {
    it("clears stale data and re-hydrates", async () => {
      const workspaces = [
        { id: "ws-1", name: "First", slug: "first" },
        { id: "ws-2", name: "Second", slug: "second" },
      ] as any[];

      useWorkspaceStore.setState({ workspaces });

      vi.mocked(api.listMembers).mockResolvedValue([]);
      vi.mocked(api.listAgents).mockResolvedValue([]);
      vi.mocked(api.listSkills).mockResolvedValue([]);

      await useWorkspaceStore.getState().switchWorkspace("ws-2");

      expect(api.setWorkspaceId).toHaveBeenCalledWith("ws-2");
      expect(localStorage.getItem("multicode_workspace_id")).toBe("ws-2");
      expect(useWorkspaceStore.getState().workspace?.id).toBe("ws-2");
    });

    it("does nothing for unknown workspace ID", async () => {
      useWorkspaceStore.setState({
        workspaces: [{ id: "ws-1", name: "First", slug: "first" }] as any[],
        workspace: { id: "ws-1" } as any,
      });

      await useWorkspaceStore.getState().switchWorkspace("unknown");

      // Should not have changed
      expect(useWorkspaceStore.getState().workspace?.id).toBe("ws-1");
    });
  });

  describe("clearWorkspace", () => {
    it("resets all state and clears localStorage", () => {
      localStorage.setItem("multicode_workspace_id", "ws-1");
      useWorkspaceStore.setState({
        workspace: { id: "ws-1" } as any,
        workspaces: [{ id: "ws-1" }] as any[],
        members: [{ id: "m-1" }] as any[],
        agents: [{ id: "a-1" }] as any[],
        skills: [{ id: "s-1" }] as any[],
      });

      useWorkspaceStore.getState().clearWorkspace();

      expect(api.setWorkspaceId).toHaveBeenCalledWith(null);
      expect(localStorage.getItem("multicode_workspace_id")).toBeNull();
      expect(useWorkspaceStore.getState().workspace).toBeNull();
      expect(useWorkspaceStore.getState().workspaces).toEqual([]);
      expect(useWorkspaceStore.getState().members).toEqual([]);
    });
  });

  describe("updateAgent", () => {
    it("updates agent by ID", () => {
      useWorkspaceStore.setState({
        agents: [
          { id: "a-1", name: "Old" },
          { id: "a-2", name: "Other" },
        ] as any[],
      });

      useWorkspaceStore.getState().updateAgent("a-1", { name: "New" } as any);

      const agents = useWorkspaceStore.getState().agents;
      expect(agents[0]!.name).toBe("New");
      expect(agents[1]!.name).toBe("Other");
    });
  });

  describe("upsertSkill", () => {
    it("inserts new skill when not found", () => {
      useWorkspaceStore.setState({ skills: [{ id: "s-1" }] as any[] });
      useWorkspaceStore.getState().upsertSkill({ id: "s-2" } as any);
      expect(useWorkspaceStore.getState().skills).toHaveLength(2);
    });

    it("updates existing skill", () => {
      useWorkspaceStore.setState({ skills: [{ id: "s-1", name: "Old" }] as any[] });
      useWorkspaceStore.getState().upsertSkill({ id: "s-1", name: "New" } as any);
      expect(useWorkspaceStore.getState().skills).toHaveLength(1);
      expect(useWorkspaceStore.getState().skills[0]!.name).toBe("New");
    });
  });

  describe("removeSkill", () => {
    it("removes skill by ID", () => {
      useWorkspaceStore.setState({
        skills: [{ id: "s-1" }, { id: "s-2" }] as any[],
      });
      useWorkspaceStore.getState().removeSkill("s-1");
      expect(useWorkspaceStore.getState().skills).toHaveLength(1);
      expect(useWorkspaceStore.getState().skills[0]!.id).toBe("s-2");
    });
  });
});
