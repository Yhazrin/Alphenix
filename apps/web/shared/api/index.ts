import { createLogger } from "@/shared/logger";
import { ApiClient } from "./client";

export { ApiClient } from "./client";
export { WSClient } from "./ws-client";

// Re-export domain-specific API modules
export { authApi } from "./auth";
export { workspaceApi, configureWorkspaceApi } from "./workspace";
export { issuesApi, configureIssuesApi } from "./issues";
export { tasksApi, configureTasksApi } from "./tasks";
export { agentsApi, configureAgentsApi } from "./agents";
export { runtimesApi, configureRuntimesApi } from "./runtimes";
export { skillsApi, configureSkillsApi } from "./skills";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "";

export const api = new ApiClient(API_BASE_URL, { logger: createLogger("api") });

// Initialize token from localStorage on load
if (typeof window !== "undefined") {
  const token = localStorage.getItem("multica_token");
  if (token) {
    api.setToken(token);
  }
  const wsId = localStorage.getItem("multica_workspace_id");
  if (wsId) {
    api.setWorkspaceId(wsId);
  }
}
