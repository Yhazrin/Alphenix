"use client";

import { create } from "zustand";
import { createStore, type StoreApi } from "zustand/vanilla";
import { persist } from "zustand/middleware";
import type { IssueStatus, IssuePriority } from "@/shared/types";
import { ALL_STATUSES } from "@/features/issues/config";

export type SortField =
  | "updated_at"
  | "created_at"
  | "position"
  | "priority"
  | "due_date"
  | "title"
  | "status"
  | "identifier";
export type SortDirection = "asc" | "desc";

export interface CardProperties {
  priority: boolean;
  description: boolean;
  assignee: boolean;
  dueDate: boolean;
}

export interface ActorFilterValue {
  type: "member" | "agent";
  id: string;
}

export const SORT_OPTIONS: { value: SortField; label: string }[] = [
  { value: "updated_at", label: "Last updated" },
  { value: "created_at", label: "Created" },
  { value: "priority", label: "Priority" },
  { value: "due_date", label: "Due date" },
  { value: "status", label: "Status" },
  { value: "title", label: "Title" },
  { value: "identifier", label: "ID" },
  { value: "position", label: "Manual" },
];

const VALID_SORT = new Set(SORT_OPTIONS.map((o) => o.value));

export const CARD_PROPERTY_OPTIONS: { key: keyof CardProperties; label: string }[] = [
  { key: "priority", label: "Priority" },
  { key: "description", label: "Description" },
  { key: "assignee", label: "Assignee" },
  { key: "dueDate", label: "Due date" },
];

export interface IssueViewState {
  statusFilters: IssueStatus[];
  priorityFilters: IssuePriority[];
  assigneeFilters: ActorFilterValue[];
  includeNoAssignee: boolean;
  creatorFilters: ActorFilterValue[];
  sortBy: SortField;
  sortDirection: SortDirection;
  cardProperties: CardProperties;
  toggleStatusFilter: (status: IssueStatus) => void;
  togglePriorityFilter: (priority: IssuePriority) => void;
  toggleAssigneeFilter: (value: ActorFilterValue) => void;
  toggleNoAssignee: () => void;
  toggleCreatorFilter: (value: ActorFilterValue) => void;
  hideStatus: (status: IssueStatus) => void;
  showStatus: (status: IssueStatus) => void;
  clearFilters: () => void;
  setSortBy: (field: SortField) => void;
  setSortDirection: (dir: SortDirection) => void;
  toggleCardProperty: (key: keyof CardProperties) => void;
}

export const viewStoreSlice = (set: StoreApi<IssueViewState>["setState"]): IssueViewState => ({
  statusFilters: [],
  priorityFilters: [],
  assigneeFilters: [],
  includeNoAssignee: false,
  creatorFilters: [],
  sortBy: "updated_at",
  sortDirection: "desc",
  cardProperties: {
    priority: true,
    description: true,
    assignee: true,
    dueDate: true,
  },

  toggleStatusFilter: (status) =>
    set((state) => ({
      statusFilters: state.statusFilters.includes(status)
        ? state.statusFilters.filter((s) => s !== status)
        : [...state.statusFilters, status],
    })),
  togglePriorityFilter: (priority) =>
    set((state) => ({
      priorityFilters: state.priorityFilters.includes(priority)
        ? state.priorityFilters.filter((p) => p !== priority)
        : [...state.priorityFilters, priority],
    })),
  toggleAssigneeFilter: (value) =>
    set((state) => {
      const exists = state.assigneeFilters.some(
        (f) => f.type === value.type && f.id === value.id,
      );
      return {
        assigneeFilters: exists
          ? state.assigneeFilters.filter(
              (f) => !(f.type === value.type && f.id === value.id),
            )
          : [...state.assigneeFilters, value],
      };
    }),
  toggleNoAssignee: () =>
    set((state) => ({ includeNoAssignee: !state.includeNoAssignee })),
  toggleCreatorFilter: (value) =>
    set((state) => {
      const exists = state.creatorFilters.some(
        (f) => f.type === value.type && f.id === value.id,
      );
      return {
        creatorFilters: exists
          ? state.creatorFilters.filter(
              (f) => !(f.type === value.type && f.id === value.id),
            )
          : [...state.creatorFilters, value],
      };
    }),
  hideStatus: (status) =>
    set((state) => {
      if (state.statusFilters.length === 0) {
        return { statusFilters: ALL_STATUSES.filter((s) => s !== status) };
      }
      return {
        statusFilters: state.statusFilters.filter((s) => s !== status),
      };
    }),
  showStatus: (status) =>
    set((state) => {
      if (state.statusFilters.length === 0) return state;
      if (state.statusFilters.includes(status)) return state;
      return { statusFilters: [...state.statusFilters, status] };
    }),
  clearFilters: () =>
    set({
      statusFilters: [],
      priorityFilters: [],
      assigneeFilters: [],
      includeNoAssignee: false,
      creatorFilters: [],
    }),
  setSortBy: (field) => set({ sortBy: field }),
  setSortDirection: (dir) => set({ sortDirection: dir }),
  toggleCardProperty: (key) =>
    set((state) => ({
      cardProperties: {
        ...state.cardProperties,
        [key]: !state.cardProperties[key],
      },
    })),
});

export type PersistedIssueView = Pick<
  IssueViewState,
  | "statusFilters"
  | "priorityFilters"
  | "assigneeFilters"
  | "includeNoAssignee"
  | "creatorFilters"
  | "sortBy"
  | "sortDirection"
  | "cardProperties"
>;

export const viewStorePersistOptions = (name: string) => ({
  name,
  version: 3,
  migrate: (persisted: unknown, _fromVersion: number) => {
    if (!persisted || typeof persisted !== "object") return persisted;
    const wrap = persisted as { state?: Record<string, unknown> };
    if (wrap.state) {
      delete wrap.state.viewMode;
      delete wrap.state.listCollapsedStatuses;
      const sb = wrap.state.sortBy;
      if (typeof sb !== "string" || !VALID_SORT.has(sb as SortField)) {
        wrap.state.sortBy = "updated_at";
        wrap.state.sortDirection = "desc";
      }
    }
    return persisted;
  },
  merge: (persisted: unknown, current: IssueViewState): IssueViewState => {
    if (!persisted || typeof persisted !== "object") return current;
    const p = { ...(persisted as Record<string, unknown>) };
    delete p.viewMode;
    delete p.listCollapsedStatuses;
    delete p.setViewMode;
    delete p.toggleListCollapsed;
    const next = { ...current, ...p } as IssueViewState;
    if (!VALID_SORT.has(next.sortBy)) {
      next.sortBy = "updated_at";
      next.sortDirection = "desc";
    }
    return next;
  },
  partialize: (state: IssueViewState): PersistedIssueView => ({
    statusFilters: state.statusFilters,
    priorityFilters: state.priorityFilters,
    assigneeFilters: state.assigneeFilters,
    includeNoAssignee: state.includeNoAssignee,
    creatorFilters: state.creatorFilters,
    sortBy: state.sortBy,
    sortDirection: state.sortDirection,
    cardProperties: state.cardProperties,
  }),
});

/** Factory: creates a vanilla StoreApi for use with React Context. */
export function createIssueViewStore(persistKey: string): StoreApi<IssueViewState> {
  return createStore<IssueViewState>()(
    persist(viewStoreSlice, viewStorePersistOptions(persistKey))
  );
}

/** Global singleton for the /issues page. */
export const useIssueViewStore = create<IssueViewState>()(
  persist(viewStoreSlice, viewStorePersistOptions("alphenix_issues_view"))
);

const _syncedStores = new Set<StoreApi<IssueViewState>>();
let _workspaceSyncInitialized = false;

export function registerViewStoreForWorkspaceSync(store: StoreApi<IssueViewState>) {
  _syncedStores.add(store);
  if (_workspaceSyncInitialized) return;
  _workspaceSyncInitialized = true;

  import("@/features/workspace")
    .then(({ useWorkspaceStore }) => {
      let prevId: string | undefined;
      useWorkspaceStore.subscribe((state) => {
        const id = state.workspace?.id;
        if (prevId && id !== prevId) {
          for (const s of _syncedStores) s.getState().clearFilters();
        }
        prevId = id;
      });
    })
    .catch((e) => {
      console.error("Failed to initialize workspace sync:", e);
    });
}

export const initFilterWorkspaceSync = () =>
  registerViewStoreForWorkspaceSync(useIssueViewStore);
