"use client";

import { useIssueStore } from "@/features/issues";
import { useInboxStore } from "@/features/inbox";
import { useRuntimeStore } from "@/features/runtimes";

/**
 * Orchestrates cross-store state resets when workspace context changes.
 * This centralizes what was previously distributed across store hydrate/switch methods.
 *
 * TODO: When migrating to TanStack Query, this orchestration layer becomes
 * the natural place to manage query invalidation on workspace switch via
 * queryClient.invalidateQueries({ queryKey: ['issues'] })
 */

/** Clears all secondary stores to their empty初始状态. Called BEFORE workspace identity switches. */
export function resetStores() {
  useIssueStore.getState().reset();
  useInboxStore.getState().reset();
  useRuntimeStore.getState().reset();
}

/**
 * Resets stores AND re-fetches their data for the given workspace.
 * Use this when performing a full workspace switch where you want immediate refetch.
 */
export function switchWorkspace(_workspaceId: string) {
  resetStores();

  useIssueStore.getState().fetch();
  useInboxStore.getState().fetch();
  useRuntimeStore.getState().fetchRuntimes();
}
