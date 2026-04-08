"use client";

import { create } from "zustand";
import type { Issue } from "@/shared/types";

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

export const useIssueStore = create<IssueClientState>((set) => ({
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
    set({ loading: false });
  },

  loadMore: async () => {
    set({ loadingMore: true });
    set({ loadingMore: false });
  },
}));
