"use client";

import { useState, useEffect, useCallback } from "react";
import type { MCPServer } from "@/shared/types";
import { toast } from "sonner";
import { mcpApi } from "@/shared/api";
import { McpServerList } from "./mcp-server-list";

export function McpPage() {
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchServers = useCallback(async () => {
    try {
      const data = await mcpApi.list();
      setServers(data);
    } catch {
      // API not ready yet — show empty state
      setServers([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchServers();
  }, [fetchServers]);

  const handleCreate = async (data: {
    name: string;
    url: string;
    transport: "stdio" | "sse";
    config?: Record<string, unknown>;
  }) => {
    try {
      const server = await mcpApi.create(data);
      setServers((prev) => [...prev, server]);
      toast.success("Server added");
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : "Failed to add server"
      );
      throw e;
    }
  };

  const handleUpdate = async (
    id: string,
    data: Partial<MCPServer>
  ) => {
    try {
      const updated = await mcpApi.update(id, data);
      setServers((prev) =>
        prev.map((s) => (s.id === id ? updated : s))
      );
      toast.success("Server updated");
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : "Failed to update server"
      );
      throw e;
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await mcpApi.delete(id);
      setServers((prev) => prev.filter((s) => s.id !== id));
      toast.success("Server deleted");
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : "Failed to delete server"
      );
      throw e;
    }
  };

  return (
    <div className="flex h-full flex-col">
      <McpServerList
        servers={servers}
        loading={loading}
        onCreate={handleCreate}
        onUpdate={handleUpdate}
        onDelete={handleDelete}
      />
    </div>
  );
}
