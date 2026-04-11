"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";

/** Narrow the issue list to items related to the current user (Issues page). */
export type IssuesMeFilter = "off" | "assigned" | "created" | "my_agents";

interface IssuesMeFilterState {
  meFilter: IssuesMeFilter;
  setMeFilter: (v: IssuesMeFilter) => void;
}

export const useIssuesMeFilterStore = create<IssuesMeFilterState>()(
  persist(
    (set) => ({
      meFilter: "off",
      setMeFilter: (meFilter) => set({ meFilter }),
    }),
    { name: "alphenix_issues_me_filter" },
  ),
);
