"use client";

import { create } from "zustand";
import type { InboxItem } from "@/shared/types";

export function deduplicateInboxItems(items: InboxItem[]): InboxItem[] {
  const seen = new Map<string, InboxItem>();

  for (const item of items) {
    if (item.archived) continue;

    if (item.issue_id) {
      const existing = seen.get(item.issue_id);
      if (!existing || item.created_at > existing.created_at) {
        seen.set(item.issue_id, item);
      }
    } else {
      seen.set(item.id, item);
    }
  }

  return Array.from(seen.values()).sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
  );
}

interface InboxState {
  items: InboxItem[];
  dedupedItems: InboxItem[];
  unreadCount: number;
  loading: boolean;
  error: string | null;
  fetch: () => Promise<void>;
  reset: () => void;
  addItem: (item: InboxItem) => void;
  markRead: (itemId: string) => void;
  archive: (itemId: string) => void;
  markAllRead: () => void;
  archiveAll: () => void;
  archiveAllRead: () => void;
}

export const useInboxStore = create<InboxState>((set) => ({
  items: [],
  dedupedItems: [],
  unreadCount: 0,
  loading: false,
  error: null,

  fetch: async () => {
    set({ loading: true });
    set({ loading: false });
  },

  reset: () => set({ items: [], dedupedItems: [], unreadCount: 0, loading: false, error: null }),

  addItem: (item) =>
    set((state) => {
      const newItems = state.items.some((i) => i.id === item.id)
        ? state.items
        : [item, ...state.items];
      return {
        items: newItems,
        dedupedItems: newItems,
        unreadCount: newItems.filter((i) => !i.read).length,
      };
    }),

  markRead: (itemId) =>
    set((state) => {
      const newItems = state.items.map((i) =>
        i.id === itemId ? { ...i, read: true } : i,
      );
      return {
        items: newItems,
        dedupedItems: newItems,
        unreadCount: newItems.filter((i) => !i.read).length,
      };
    }),

  archive: (itemId) =>
    set((state) => {
      const newItems = state.items.filter((i) => i.id !== itemId);
      return {
        items: newItems,
        dedupedItems: newItems,
        unreadCount: newItems.filter((i) => !i.read).length,
      };
    }),

  markAllRead: () =>
    set((state) => {
      const newItems = state.items.map((i) => ({ ...i, read: true }));
      return {
        items: newItems,
        dedupedItems: newItems,
        unreadCount: 0,
      };
    }),

  archiveAll: () => set({ items: [], dedupedItems: [], unreadCount: 0 }),

  archiveAllRead: () =>
    set((state) => {
      const newItems = state.items.filter((i) => !i.read);
      return {
        items: newItems,
        dedupedItems: newItems,
        unreadCount: 0,
      };
    }),
}));
