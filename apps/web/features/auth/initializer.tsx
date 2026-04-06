"use client";

import { useEffect, type ReactNode } from "react";
import { useAuthStore } from "./store";
import { useWorkspaceStore } from "@/features/workspace";
import { api, configureIssuesApi, configureAgentsApi, configureTasksApi, configureRuntimesApi } from "@/shared/api";
import { createLogger } from "@/shared/logger";
import { setLoggedInCookie, clearLoggedInCookie } from "./auth-cookie";

const logger = createLogger("auth");

/**
 * Initializes auth + workspace state on mount using HttpOnly cookie.
 * Fires getMe() and listWorkspaces() in parallel.
 * If the cookie is missing/expired, the user stays on the login page.
 */
export function AuthInitializer({ children }: { children: ReactNode }) {
  useEffect(() => {
    // Sync workspaceId from localStorage BEFORE any async ops.
    // This ensures child components can make API calls immediately on mount.
    let wsId: string | null = null;
    try { wsId = localStorage.getItem("alphenix_workspace_id"); } catch {}
    if (wsId) {
      api.setWorkspaceId(wsId);
      // Also sync workspaceId for domain-specific API modules before any async ops.
      configureIssuesApi({ workspaceId: wsId });
      configureAgentsApi({ workspaceId: wsId });
      configureTasksApi({ workspaceId: wsId });
      configureRuntimesApi({ workspaceId: wsId });
    }

    // Fire getMe and listWorkspaces in parallel — cookie is sent automatically
    const mePromise = api.getMe();
    const wsPromise = api.listWorkspaces();

    Promise.all([mePromise, wsPromise])
      .then(([user, wsList]) => {
        setLoggedInCookie();
        useAuthStore.setState({ user, isLoading: false });
        useWorkspaceStore.getState().hydrateWorkspace(wsList, wsId);
      })
      .catch((err) => {
        logger.error("auth init failed", err);
        api.setWorkspaceId(null);
        try { localStorage.removeItem("alphenix_workspace_id"); } catch {}
        clearLoggedInCookie();
        useAuthStore.setState({ user: null, isLoading: false });
      });
  }, []);

  return <>{children}</>;
}
