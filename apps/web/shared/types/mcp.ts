export type MCPTransport = "stdio" | "sse";
export type MCPServerStatus = "active" | "paused" | "error";

export interface MCPServer {
  id: string;
  name: string;
  url: string;
  transport: MCPTransport;
  status: MCPServerStatus;
  config: Record<string, unknown>;
  workspace_id: string;
  created_at: string;
  updated_at: string;
}

export interface CreateMCPServerRequest {
  name: string;
  url: string;
  transport?: MCPTransport;
  config?: Record<string, unknown>;
}

export interface UpdateMCPServerRequest {
  name?: string;
  url?: string;
  transport?: MCPTransport;
  status?: MCPServerStatus;
  config?: Record<string, unknown>;
}
