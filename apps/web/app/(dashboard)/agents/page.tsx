"use client";

import { useState, useEffect, useMemo } from "react";
import { useDefaultLayout } from "react-resizable-panels";
import {
  Bot,
  Plus,
  Archive,
} from "lucide-react";
import type {
  Agent,
  CreateAgentRequest,
  UpdateAgentRequest,
} from "@/shared/types";
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@/components/ui/resizable";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { Skeleton } from "@/components/ui/skeleton";
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription } from "@/components/ui/empty";
import { api } from "@/shared/api";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { useRuntimeStore } from "@/features/runtimes";
import { CreateAgentDialog } from "./_components/create-agent-dialog";
import { AgentListItem } from "./_components/agent-list-panel";
import { AgentDetail } from "./_components/agent-detail-panel";

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function AgentsPage() {
  const isLoading = useAuthStore((s) => s.isLoading);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const agents = useWorkspaceStore((s) => s.agents);
  const refreshAgents = useWorkspaceStore((s) => s.refreshAgents);
  const [selectedId, setSelectedId] = useState<string>("");
  const [showArchived, setShowArchived] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const runtimes = useRuntimeStore((s) => s.runtimes);
  const fetchRuntimes = useRuntimeStore((s) => s.fetchRuntimes);
  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id: "multicode_agents_layout",
  });

  useEffect(() => {
    if (workspace) fetchRuntimes();
  }, [workspace, fetchRuntimes]);

  const filteredAgents = useMemo(
    () => showArchived ? agents.filter((a) => !!a.archived_at) : agents.filter((a) => !a.archived_at),
    [agents, showArchived],
  );

  const archivedCount = useMemo(() => agents.filter((a) => !!a.archived_at).length, [agents]);

  // Select first agent on initial load or when filter changes
  useEffect(() => {
    if (filteredAgents.length > 0 && !filteredAgents.some((a) => a.id === selectedId)) {
      setSelectedId(filteredAgents[0]!.id);
    }
  }, [filteredAgents, selectedId]);

  const handleCreate = async (data: CreateAgentRequest) => {
    try {
      const agent = await api.createAgent(data);
      await refreshAgents();
      setSelectedId(agent.id);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to create agent");
      throw e;
    }
  };

  const handleUpdate = async (id: string, data: Record<string, unknown>) => {
    try {
      await api.updateAgent(id, data as UpdateAgentRequest);
      await refreshAgents();
      toast.success("Agent updated");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update agent");
      throw e;
    }
  };

  const handleArchive = async (id: string) => {
    try {
      await api.archiveAgent(id);
      await refreshAgents();
      toast.success("Agent archived");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to archive agent");
    }
  };

  const handleRestore = async (id: string) => {
    try {
      await api.restoreAgent(id);
      await refreshAgents();
      toast.success("Agent restored");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to restore agent");
    }
  };

  const selected = agents.find((a) => a.id === selectedId) ?? null;

  if (isLoading) {
    return (
      <div className="flex flex-1 min-h-0">
        {/* List skeleton */}
        <div className="w-72 border-r">
          <div className="flex h-12 items-center justify-between border-b px-4">
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-6 w-6 rounded" />
          </div>
          <div className="divide-y">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="flex items-center gap-3 px-4 py-3">
                <Skeleton className="h-8 w-8 rounded-full" />
                <div className="flex-1 space-y-1.5">
                  <Skeleton className="h-4 w-24" />
                  <Skeleton className="h-3 w-16" />
                </div>
              </div>
            ))}
          </div>
        </div>
        {/* Detail skeleton */}
        <div className="flex-1 p-6 space-y-6">
          <div className="flex items-center gap-3">
            <Skeleton className="h-10 w-10 rounded-full" />
            <div className="space-y-1.5">
              <Skeleton className="h-5 w-32" />
              <Skeleton className="h-3 w-20" />
            </div>
          </div>
          <div className="space-y-3">
            <Skeleton className="h-8 w-full rounded-lg" />
            <Skeleton className="h-8 w-full rounded-lg" />
            <Skeleton className="h-8 w-3/4 rounded-lg" />
          </div>
        </div>
      </div>
    );
  }

  return (
    <ResizablePanelGroup
      orientation="horizontal"
      className="flex-1 min-h-0"
      defaultLayout={defaultLayout}
      onLayoutChanged={onLayoutChanged}
    >
      <ResizablePanel id="list" defaultSize={280} minSize={240} maxSize={400} groupResizeBehavior="preserve-pixel-size">
        {/* Left column — agent list */}
        <div className="overflow-y-auto h-full border-r">
          <div className="flex h-12 items-center justify-between border-b px-4">
            <h1 className="text-sm font-semibold">Agents</h1>
            <div className="flex items-center gap-1">
              {archivedCount > 0 && (
                <Button
                  variant={showArchived ? "secondary" : "ghost"}
                  size="icon-xs"
                  onClick={() => setShowArchived(!showArchived)}
                  title={showArchived ? "Show active agents" : "Show archived agents"}
                >
                  <Archive className="h-4 w-4 text-muted-foreground" />
                </Button>
              )}
              <Button
                variant="ghost"
                size="icon-xs"
                onClick={() => setShowCreate(true)}
              >
                <Plus className="h-4 w-4 text-muted-foreground" />
              </Button>
            </div>
          </div>
          {filteredAgents.length === 0 ? (
            <Empty className="border-0 px-4 py-12">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <Bot />
                </EmptyMedia>
                <EmptyTitle>
                  {showArchived ? "No archived agents" : archivedCount > 0 ? "No active agents" : "No agents yet"}
                </EmptyTitle>
              </EmptyHeader>
              {!showArchived && (
                <Button
                  onClick={() => setShowCreate(true)}
                  size="xs"
                >
                  <Plus className="h-3 w-3" />
                  Create Agent
                </Button>
              )}
            </Empty>
          ) : (
            <div className="divide-y">
              {filteredAgents.map((agent) => (
                <AgentListItem
                  key={agent.id}
                  agent={agent}
                  isSelected={agent.id === selectedId}
                  onClick={() => setSelectedId(agent.id)}
                />
              ))}
            </div>
          )}
        </div>
      </ResizablePanel>

      <ResizableHandle />

      <ResizablePanel id="detail" minSize="50%">
        {/* Right column — agent detail */}
        {selected ? (
          <AgentDetail
            key={selected.id}
            agent={selected}
            runtimes={runtimes}
            onUpdate={handleUpdate}
            onArchive={handleArchive}
            onRestore={handleRestore}
          />
        ) : (
          <Empty className="border-0">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <Bot />
              </EmptyMedia>
              <EmptyTitle>Select an agent to view details</EmptyTitle>
            </EmptyHeader>
            <Button
              onClick={() => setShowCreate(true)}
              size="xs"
            >
              <Plus className="h-3 w-3" />
              Create Agent
            </Button>
          </Empty>
        )}
      </ResizablePanel>

      {showCreate && (
        <CreateAgentDialog
          runtimes={runtimes}
          onClose={() => setShowCreate(false)}
          onCreate={handleCreate}
        />
      )}
    </ResizablePanelGroup>
  );
}
