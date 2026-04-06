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
export { runsApi, configureRunsApi } from "./runs";
export { mcpApi, configureMcpApi } from "./mcp";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "";

export const api = new ApiClient(API_BASE_URL, { logger: createLogger("api") });

// Initialize workspace_id from localStorage on load.
// Auth uses HttpOnly cookie — no token stored in localStorage.
if (typeof window !== "undefined") {
  const wsId = localStorage.getItem("alphenix_workspace_id");
  if (wsId) {
    api.setWorkspaceId(wsId);
  }
}
