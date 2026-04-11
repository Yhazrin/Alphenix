/**
 * Persisted workspace selection. Alphenix key is canonical; legacy Multica key is read once for migration.
 */
export const WORKSPACE_ID_STORAGE_KEY = "alphenix_workspace_id";
export const LEGACY_WORKSPACE_ID_STORAGE_KEY = "multica_workspace_id";

export function readStoredWorkspaceId(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return (
      localStorage.getItem(WORKSPACE_ID_STORAGE_KEY) ??
      localStorage.getItem(LEGACY_WORKSPACE_ID_STORAGE_KEY)
    );
  } catch {
    return null;
  }
}

export function persistWorkspaceIdToStorage(workspaceId: string): void {
  try {
    localStorage.setItem(WORKSPACE_ID_STORAGE_KEY, workspaceId);
    localStorage.removeItem(LEGACY_WORKSPACE_ID_STORAGE_KEY);
  } catch {
    /* localStorage unavailable */
  }
}

export function clearWorkspaceIdFromStorage(): void {
  try {
    localStorage.removeItem(WORKSPACE_ID_STORAGE_KEY);
    localStorage.removeItem(LEGACY_WORKSPACE_ID_STORAGE_KEY);
  } catch {
    /* localStorage unavailable */
  }
}
