"use client";

import { create } from "zustand";
import type { Channel } from "@/shared/types";
import { api } from "@/shared/api";
import { useWorkspaceStore } from "@/features/workspace";

const FILTER_STORAGE_KEY = "alphenix.channelFilter.v1";

function readFilterForWorkspace(wsId: string): string | null | undefined {
  if (typeof window === "undefined") return undefined;
  try {
    const raw = localStorage.getItem(FILTER_STORAGE_KEY);
    if (!raw) return undefined;
    const map = JSON.parse(raw) as Record<string, string | null>;
    if (!(wsId in map)) return undefined;
    return map[wsId];
  } catch {
    return undefined;
  }
}

function writeFilterForWorkspace(wsId: string, channelId: string | null) {
  if (typeof window === "undefined") return;
  try {
    const raw = localStorage.getItem(FILTER_STORAGE_KEY);
    const map = raw ? (JSON.parse(raw) as Record<string, string | null>) : {};
    map[wsId] = channelId;
    localStorage.setItem(FILTER_STORAGE_KEY, JSON.stringify(map));
  } catch {
    /* ignore */
  }
}

interface ChannelState {
  channels: Channel[];
  loading: boolean;
  error: string | null;
  /** null = show issues from all channels */
  filterChannelId: string | null;
  _hydratedWorkspaceId: string | null;
  fetchChannels: () => Promise<void>;
  setFilterChannelId: (id: string | null) => void;
  createChannel: (name: string) => Promise<Channel>;
}

export const useChannelStore = create<ChannelState>((set, get) => ({
  channels: [],
  loading: false,
  error: null,
  filterChannelId: null,
  _hydratedWorkspaceId: null,

  fetchChannels: async () => {
    const wsId = useWorkspaceStore.getState().workspace?.id;
    if (!wsId) {
      set({ channels: [], loading: false, error: null });
      return;
    }
    set({ loading: true, error: null });
    try {
      const { channels } = await api.listChannels();
      const { _hydratedWorkspaceId, filterChannelId: prevFilter } = get();

      let nextFilter: string | null;
      if (_hydratedWorkspaceId !== wsId) {
        const persisted = readFilterForWorkspace(wsId);
        nextFilter = persisted === undefined ? null : persisted;
        const valid =
          nextFilter === null ||
          channels.some((c) => c.id === nextFilter);
        if (!valid) nextFilter = null;
      } else {
        nextFilter = prevFilter;
      }

      set({
        channels,
        loading: false,
        error: null,
        filterChannelId: nextFilter,
        _hydratedWorkspaceId: wsId,
      });
      if (prevFilter !== nextFilter) {
        const { useIssueStore } = await import("@/features/issues/store");
        void useIssueStore.getState().fetch();
      }
    } catch (e) {
      set({
        loading: false,
        error: e instanceof Error ? e.message : "Failed to load channels",
      });
    }
  },

  setFilterChannelId: (id) => {
    const wsId = useWorkspaceStore.getState().workspace?.id;
    if (wsId) writeFilterForWorkspace(wsId, id);
    const prev = get().filterChannelId;
    set({ filterChannelId: id });
    if (prev !== id) {
      void import("@/features/issues/store").then((m) => {
        void m.useIssueStore.getState().fetch();
      });
    }
  },

  createChannel: async (name) => {
    const trimmed = name.trim();
    if (!trimmed) throw new Error("Channel name is required");
    const ch = await api.createChannel({ name: trimmed });
    await get().fetchChannels();
    get().setFilterChannelId(ch.id);
    return ch;
  },
}));

if (typeof window !== "undefined" && typeof useWorkspaceStore.subscribe === "function") {
  useWorkspaceStore.subscribe((state, prev) => {
    const a = state.workspace?.id ?? null;
    const b = prev.workspace?.id ?? null;
    if (a === b) return;
    useChannelStore.setState({
      channels: [],
      _hydratedWorkspaceId: null,
      filterChannelId: null,
      error: null,
      loading: false,
    });
    if (a) void useChannelStore.getState().fetchChannels();
  });
}
