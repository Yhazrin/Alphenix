"use client";

import { create } from "zustand";
import type { Issue } from "@/shared/types";
import { toast } from "sonner";
import { api } from "@/shared/api";
import { createLogger } from "@/shared/logger";

const logger = createLogger("issue-store");

const PAGE_SIZE = 200;

interface IssueState {
  issues: Issue[];
  loading: boolean;
  loadingMore: boolean;
  error: string | null;
  hasMore: boolean;
  cursor: string | null;
  activeIssueId: string | null;
  fetch: () => Promise<void>;
  loadMore: () => Promise<void>;
  reset: () => void;
  setIssues: (issues: Issue[]) => void;
  addIssue: (issue: Issue) => void;
  updateIssue: (id: string, updates: Partial<Issue>) => void;
  removeIssue: (id: string) => void;
  setActiveIssue: (id: string | null) => void;
}

export const useIssueStore = create<IssueState>((set, get) => ({
  issues: [],
  loading: true,
  loadingMore: false,
  error: null,
  hasMore: true,
  cursor: null,
  activeIssueId: null,

  fetch: async () => {
    logger.debug("fetch start");
    const isInitialLoad = get().issues.length === 0;
    if (isInitialLoad) set({ loading: true, error: null });
    try {
      const res = await api.listIssues({ limit: PAGE_SIZE });
      logger.info("fetched", res.issues.length, "issues");
      set({
        issues: res.issues,
        loading: false,
        error: null,
        hasMore: !!res.next_cursor,
        cursor: res.next_cursor ?? null,
      });
    } catch (err) {
      logger.error("fetch failed", err);
      const message = "Failed to load issues";
      toast.error(message);
      set({ loading: false, error: message });
    }
  },

  loadMore: async () => {
    const { cursor, loadingMore, hasMore } = get();
    if (!cursor || loadingMore || !hasMore) return;
    set({ loadingMore: true });
    try {
      const res = await api.listIssues({ limit: PAGE_SIZE, cursor });
      logger.info("loaded more", res.issues.length, "issues");
      set((s) => ({
        issues: [...s.issues, ...res.issues],
        loadingMore: false,
        hasMore: !!res.next_cursor,
        cursor: res.next_cursor ?? null,
      }));
    } catch (err) {
      logger.error("loadMore failed", err);
      set({ loadingMore: false });
    }
  },

  setIssues: (issues) => set({ issues }),

  reset: () =>
    set({
      issues: [],
      loading: false,
      error: null,
      hasMore: true,
      cursor: null,
      activeIssueId: null,
    }),

  addIssue: (issue) =>
    set((s) => ({
      issues: s.issues.some((i) => i.id === issue.id)
        ? s.issues
        : [...s.issues, issue],
    })),
  updateIssue: (id, updates) =>
    set((s) => ({
      issues: s.issues.map((i) => (i.id === id ? { ...i, ...updates } : i)),
    })),
  removeIssue: (id) =>
    set((s) => ({ issues: s.issues.filter((i) => i.id !== id) })),
  setActiveIssue: (id) => set({ activeIssueId: id }),
}));
