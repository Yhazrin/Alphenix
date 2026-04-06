"use client";

import { useState } from "react";
import { Loader2, Plus, Save, Trash2, Plug } from "lucide-react";
import type { MCPServer } from "@/shared/types";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export function McpServerForm({
  server,
  onClose,
  onSubmit,
  onDelete,
}: {
  server?: MCPServer;
  onClose: () => void;
  onSubmit: (data: {
    name: string;
    url: string;
    transport: "stdio" | "sse";
    config?: Record<string, unknown>;
  }) => Promise<void>;
  onDelete?: () => void;
}) {
  const isEdit = !!server;

  const [name, setName] = useState(server?.name ?? "");
  const [url, setUrl] = useState(server?.url ?? "");
  const [transport, setTransport] = useState<"stdio" | "sse">(
    server?.transport ?? "sse"
  );
  const [configText, setConfigText] = useState(
    server?.config ? JSON.stringify(server.config, null, 2) : ""
  );
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async () => {
    if (!name.trim() || !url.trim()) return;

    let config: Record<string, unknown> | undefined;
    if (configText.trim()) {
      try {
        config = JSON.parse(configText);
        if (typeof config !== "object" || config === null || Array.isArray(config)) {
          setError("Config must be a JSON object");
          return;
        }
      } catch {
        setError("Invalid JSON in config field");
        return;
      }
    }

    setLoading(true);
    setError("");
    try {
      await onSubmit({
        name: name.trim(),
        url: url.trim(),
        transport,
        config,
      });
      onClose();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : `Failed to ${isEdit ? "update" : "create"} server`
      );
      setLoading(false);
    }
  };

  return (
    <Dialog
      open
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="text-sm">
            {isEdit ? "Edit Server" : "Add MCP Server"}
          </DialogTitle>
          <DialogDescription className="text-xs">
            {isEdit
              ? "Update the server configuration."
              : "Connect an MCP server to give your agents access to external tools."}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div>
            <Label
              htmlFor="mcp-name"
              className="text-xs text-muted-foreground"
            >
              Name
            </Label>
            <Input
              id="mcp-name"
              autoFocus
              placeholder="My MCP Server"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSubmit()}
              className="mt-1"
            />
          </div>

          <div>
            <Label
              htmlFor="mcp-url"
              className="text-xs text-muted-foreground"
            >
              URL
            </Label>
            <Input
              id="mcp-url"
              placeholder="http://localhost:3001/sse"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSubmit()}
              className="mt-1 font-mono text-sm"
            />
          </div>

          <div>
            <Label className="text-xs text-muted-foreground">
              Transport
            </Label>
            <Select
              value={transport}
              onValueChange={(v) => setTransport(v as "stdio" | "sse")}
            >
              <SelectTrigger className="mt-1">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="sse">SSE (Server-Sent Events)</SelectItem>
                <SelectItem value="stdio">stdio</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div>
            <Label
              htmlFor="mcp-config"
              className="text-xs text-muted-foreground"
            >
              Config (JSON, optional)
            </Label>
            <textarea
              id="mcp-config"
              placeholder='{"apiKey": "..."}'
              value={configText}
              onChange={(e) => setConfigText(e.target.value)}
              className="mt-1 h-20 w-full rounded-md border border-input bg-transparent px-3 py-2 font-mono text-xs shadow-xs placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
          </div>

          {error && (
            <div className="flex items-center gap-2 rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
              <Plug className="h-3.5 w-3.5 shrink-0" />
              {error}
            </div>
          )}
        </div>

        <DialogFooter>
          <div className="flex w-full items-center justify-between">
            <div>
              {onDelete && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={onDelete}
                  className="text-muted-foreground hover:text-destructive"
                >
                  <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                  Delete
                </Button>
              )}
            </div>
            <div className="flex items-center gap-2">
              <Button variant="ghost" onClick={onClose}>
                Cancel
              </Button>
              <Button
                onClick={handleSubmit}
                disabled={loading || !name.trim() || !url.trim()}
              >
                {loading ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : isEdit ? (
                  <Save className="mr-1.5 h-3.5 w-3.5" />
                ) : (
                  <Plus className="mr-1.5 h-3.5 w-3.5" />
                )}
                {loading
                  ? isEdit
                    ? "Saving..."
                    : "Creating..."
                  : isEdit
                    ? "Save"
                    : "Add Server"}
              </Button>
            </div>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
