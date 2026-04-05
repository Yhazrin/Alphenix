"use client";

import { useState } from "react";
import {
  Plus,
  Trash2,
  Server,
  AlertCircle,
  Loader2,
  Globe,
  Terminal,
  Pause,
  Play,
} from "lucide-react";
import type { MCPServer } from "@/shared/types";
import { timeAgo } from "@/shared/utils";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { toast } from "sonner";
import { EmptyState } from "@/components/common/empty-state";
import { McpServerForm } from "./mcp-server-form";

const STATUS_BADGE: Record<
  MCPServer["status"],
  { label: string; variant: "default" | "secondary" | "destructive" }
> = {
  active: { label: "Active", variant: "default" },
  paused: { label: "Paused", variant: "secondary" },
  error: { label: "Error", variant: "destructive" },
};

export function McpServerList({
  servers,
  loading,
  onCreate,
  onUpdate,
  onDelete,
}: {
  servers: MCPServer[];
  loading: boolean;
  onCreate: (data: {
    name: string;
    url: string;
    transport: "stdio" | "sse";
    config?: Record<string, unknown>;
  }) => Promise<void>;
  onUpdate: (
    id: string,
    data: Partial<MCPServer>
  ) => Promise<void>;
  onDelete: (id: string) => Promise<void>;
}) {
  const [showCreate, setShowCreate] = useState(false);
  const [editingServer, setEditingServer] = useState<MCPServer | null>(null);
  const [deletingServer, setDeletingServer] = useState<MCPServer | null>(null);
  const [saving, setSaving] = useState(false);

  const handleDelete = async () => {
    if (!deletingServer) return;
    setSaving(true);
    try {
      await onDelete(deletingServer.id);
      setDeletingServer(null);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to delete server");
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="flex flex-1 items-center justify-center py-12">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (servers.length === 0) {
    return (
      <>
        <EmptyState
          icon={Server}
          title="No MCP servers configured"
          description="Add an MCP server to give your agents access to external tools and resources."
          actions={[
            {
              label: "Add Server",
              onClick: () => setShowCreate(true),
              icon: Plus,
            },
          ]}
        />
        {showCreate && (
          <McpServerForm
            onClose={() => setShowCreate(false)}
            onSubmit={async (data) => {
              await onCreate(data);
              setShowCreate(false);
            }}
          />
        )}
      </>
    );
  }

  return (
    <>
      <div className="flex flex-1 flex-col overflow-hidden">
        <div className="flex items-center justify-between px-4 py-3">
          <h2 className="text-sm font-semibold">MCP Servers</h2>
          <Button size="xs" onClick={() => setShowCreate(true)}>
            <Plus className="mr-1 h-3 w-3" aria-hidden="true" />
            Add Server
          </Button>
        </div>

        <div className="flex-1 overflow-y-auto px-4">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="pb-2 pr-4 font-medium">Name</th>
                <th className="pb-2 pr-4 font-medium">URL</th>
                <th className="pb-2 pr-4 font-medium">Transport</th>
                <th className="pb-2 pr-4 font-medium">Status</th>
                <th className="pb-2 font-medium">Created</th>
              </tr>
            </thead>
            <tbody>
              {servers.map((server) => {
                const badge = STATUS_BADGE[server.status];
                return (
                  <tr
                    key={server.id}
                    className="cursor-pointer border-b transition-colors hover:bg-accent/30"
                    onClick={() => setEditingServer(server)}
                  >
                    <td className="py-2.5 pr-4">
                      <div className="flex items-center gap-2">
                        <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-muted">
                          <Server className="h-3.5 w-3.5 text-muted-foreground" aria-hidden="true" />
                        </div>
                        <span className="font-medium">{server.name}</span>
                      </div>
                    </td>
                    <td className="py-2.5 pr-4 font-mono text-xs text-muted-foreground">
                      {server.url}
                    </td>
                    <td className="py-2.5 pr-4">
                      <div className="flex items-center gap-1 text-xs text-muted-foreground">
                        {server.transport === "sse" ? (
                          <Globe className="h-3 w-3" aria-hidden="true" />
                        ) : (
                          <Terminal className="h-3 w-3" aria-hidden="true" />
                        )}
                        {server.transport}
                      </div>
                    </td>
                    <td className="py-2.5 pr-4">
                      <Badge variant={badge.variant} className="text-xs">
                        {badge.label}
                      </Badge>
                    </td>
                    <td className="py-2.5 text-xs text-muted-foreground">
                      {timeAgo(server.created_at)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      {showCreate && (
        <McpServerForm
          onClose={() => setShowCreate(false)}
          onSubmit={async (data) => {
            await onCreate(data);
            setShowCreate(false);
          }}
        />
      )}

      {editingServer && (
        <McpServerForm
          server={editingServer}
          onClose={() => setEditingServer(null)}
          onSubmit={async (data) => {
            await onUpdate(editingServer.id, data);
            setEditingServer(null);
          }}
          onDelete={() => {
            setDeletingServer(editingServer);
            setEditingServer(null);
          }}
        />
      )}

      {deletingServer && (
        <Dialog
          open
          onOpenChange={(v) => {
            if (!v) setDeletingServer(null);
          }}
        >
          <DialogContent className="max-w-sm">
            <DialogHeader>
              <DialogTitle className="text-sm">Delete Server</DialogTitle>
              <DialogDescription className="text-xs">
                Are you sure you want to delete &quot;{deletingServer.name}&quot;?
                This cannot be undone.
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button
                variant="ghost"
                onClick={() => setDeletingServer(null)}
              >
                Cancel
              </Button>
              <Button
                variant="destructive"
                onClick={handleDelete}
                disabled={saving}
              >
                {saving ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                )}
                Delete
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}
