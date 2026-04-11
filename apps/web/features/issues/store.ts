"use client";

import { create } from "zustand";
import type { Issue } from "@/shared/types";
import { api } from "@/shared/api";
import { useWorkspaceStore } from "@/features/workspace";
import { CLOSED_PAGE_SIZE } from "@core/issues/queries";

interface IssueClientState {
  issues: Issue[];
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  error: string | null;
  activeIssueId: string | null;
  setActiveIssue: (id: string | null) => void;
  setIssues: (issues: Issue[]) => void;
  addIssue: (issue: Issue) => void;
  updateIssue: (id: string, updates: Partial<Issue>) => void;
  removeIssue: (id: string) => void;
  fetch: () => Promise<void>;
  loadMore: () => Promise<void>;
}

export const useIssueStore = create<IssueClientState>((set, get) => ({
  issues: [],
  loading: false,
  loadingMore: false,
  hasMore: false,
  error: null,
  activeIssueId: null,

  setActiveIssue: (id) => set({ activeIssueId: id }),

  setIssues: (issues) => set({ issues }),

  addIssue: (issue) =>
    set((state) => ({
      issues: state.issues.some((i) => i.id === issue.id)
        ? state.issues
        : [...state.issues, issue],
    })),

  updateIssue: (id, updates) =>
    set((state) => ({
      issues: state.issues.map((i) =>
        i.id === id ? { ...i, ...updates } : i,
      ),
    })),

  removeIssue: (id) =>
    set((state) => ({
      issues: state.issues.filter((i) => i.id !== id),
    })),

  fetch: async () => {
    set({ loading: true, error: null });
    const wsId = useWorkspaceStore.getState().workspace?.id;
    if (!wsId) {
      set({ loading: false, issues: [], hasMore: false, loadingMore: false });
      return;
    }
    try {
      const { useChannelStore } = await import("@/features/channels/store");
      const channelId = useChannelStore.getState().filterChannelId ?? undefined;
      const chOpt = channelId ? { channel_id: channelId } : {};
      const [openRes, closedRes] = await Promise.all([
        api.listIssues({ open_only: true, ...chOpt }),
        api.listIssues({ status: "done", limit: CLOSED_PAGE_SIZE, offset: 0, ...chOpt }),
      ]);
      const issues = [...openRes.issues, ...closedRes.issues];
      const doneLoaded = closedRes.issues.length;
      const doneTotal = closedRes.total;
      set({
        issues,
        loading: false,
        error: null,
        hasMore: doneLoaded < doneTotal,
        loadingMore: false,
      });
    } catch (e) {
      set({
        loading: false,
        error: e instanceof Error ? e.message : "Failed to load issues",
      });
    }
  },

  loadMore: async () => {
    const { loadingMore, hasMore, issues } = get();
    if (!hasMore || loadingMore) return;
    const wsId = useWorkspaceStore.getState().workspace?.id;
    if (!wsId) return;

    const doneLoaded = issues.filter((i) => i.status === "done").length;
    set({ loadingMore: true });
    try {
      const { useChannelStore } = await import("@/features/channels/store");
      const channelId = useChannelStore.getState().filterChannelId ?? undefined;
      const chOpt = channelId ? { channel_id: channelId } : {};
      const res = await api.listIssues({
        status: "done",
        limit: CLOSED_PAGE_SIZE,
        offset: doneLoaded,
        ...chOpt,
      });
      const existingIds = new Set(issues.map((i) => i.id));
      const appended = res.issues.filter((i) => !existingIds.has(i.id));
      const next = [...issues, ...appended];
      const nextDoneCount = next.filter((i) => i.status === "done").length;
      set({
        issues: next,
        loadingMore: false,
        hasMore: nextDoneCount < res.total,
      });
    } catch {
      set({ loadingMore: false });
    }
  },
}));
