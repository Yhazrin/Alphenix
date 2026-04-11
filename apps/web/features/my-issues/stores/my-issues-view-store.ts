"use client";

import { createStore, type StoreApi } from "zustand/vanilla";
import { persist } from "zustand/middleware";
import {
  type IssueViewState,
  type PersistedIssueView,
  viewStoreSlice,
  viewStorePersistOptions,
} from "@/features/issues/stores/view-store";

export type MyIssuesScope = "assigned" | "created" | "agents";

export interface MyIssuesViewState extends IssueViewState {
  scope: MyIssuesScope;
  setScope: (scope: MyIssuesScope) => void;
}

type PersistedMyIssuesView = PersistedIssueView & { scope: MyIssuesScope };

const basePersist = viewStorePersistOptions("alphenix_my_issues_view");

export const myIssuesViewStore: StoreApi<MyIssuesViewState> = createStore<MyIssuesViewState>()(
  persist<MyIssuesViewState, [], [], PersistedMyIssuesView>(
    (set) => ({
      ...viewStoreSlice(set as unknown as StoreApi<IssueViewState>["setState"]),
      scope: "assigned" as MyIssuesScope,
      setScope: (scope: MyIssuesScope) => set({ scope }),
    }),
    {
      name: basePersist.name,
      version: basePersist.version,
      migrate: (persisted, version) =>
        basePersist.migrate(persisted, version) as
          | PersistedMyIssuesView
          | Promise<PersistedMyIssuesView>,
      merge: (persisted, current): MyIssuesViewState => {
        const merged = basePersist.merge(persisted, current);
        let scope: MyIssuesScope = current.scope;
        if (persisted && typeof persisted === "object") {
          const s = (persisted as { scope?: unknown }).scope;
          if (s === "assigned" || s === "created" || s === "agents") {
            scope = s;
          }
        }
        return { ...current, ...merged, scope };
      },
      partialize: (state: MyIssuesViewState): PersistedMyIssuesView => ({
        ...basePersist.partialize(state),
        scope: state.scope,
      }),
    },
  ),
);
