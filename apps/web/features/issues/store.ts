"use client";

import { create } from "zustand";
import type { Issue } from "@/shared/types";
import { toast } from "sonner";
import { api } from "@/shared/api";
import { createLogger } from "@/shared/logger";

const logger = createLogger("issue-store");

interface IssueState {
  issues: Issue[];
  loading: boolean;
  error: string | null;
  activeIssueId: string | null;
  fetch: () => Promise<void>;
  setIssues: (issues: Issue[]) => void;
  addIssue: (issue: Issue) => void;
  updateIssue: (id: string, updates: Partial<Issue>) => void;
  removeIssue: (id: string) => void;
  setActiveIssue: (id: string | null) => void;
}

export const useIssueStore = create<IssueState>((set, get) => ({
  issues: [],
  loading: true,
  error: null,
  activeIssueId: null,

  fetch: async () => {
    logger.debug("fetch start");
    const isInitialLoad = get().issues.length === 0;
    if (isInitialLoad) set({ loading: true, error: null });
    try {
      const res = await api.listIssues({ limit: 200 });
      logger.info("fetched", res.issues.length, "issues");
      set({ issues: res.issues, loading: false, error: null });
    } catch (err) {
      logger.error("fetch failed", err);
      const message = "Failed to load issues";
      toast.error(message);
      set({ loading: false, error: message });
    }
  },

  setIssues: (issues) => set({ issues }),
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
