"use client";

import { useEffect, type ReactNode } from "react";
import { useAuthStore } from "./store";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
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
    const wsId = localStorage.getItem("multicode_workspace_id");

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
        localStorage.removeItem("multicode_workspace_id");
        clearLoggedInCookie();
        useAuthStore.setState({ user: null, isLoading: false });
      });
  }, []);

  return <>{children}</>;
}
