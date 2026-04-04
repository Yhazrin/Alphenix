"use client";

import { useState, useEffect, use } from "react";
import { useRouter } from "next/navigation";
import { ArrowLeft, Bot } from "lucide-react";
import type { Agent, UpdateAgentRequest } from "@/shared/types";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { api } from "@/shared/api";
import { useRuntimeStore } from "@/features/runtimes";
import { AgentDetail } from "../_components/agent-detail-panel";

// ---------------------------------------------------------------------------
// Loading skeleton
// ---------------------------------------------------------------------------

function AgentDetailSkeleton() {
  return (
    <div className="flex h-full flex-col">
      {/* Header skeleton */}
      <div className="flex h-12 shrink-0 items-center gap-3 border-b px-4">
        <Skeleton className="h-7 w-7 rounded-md" />
        <div className="flex-1 space-y-1.5">
          <Skeleton className="h-4 w-32" />
        </div>
      </div>
      {/* Tabs skeleton */}
      <div className="flex gap-2 border-b px-6 py-2.5">
        {Array.from({ length: 6 }).map((_, i) => (
          <Skeleton key={i} className="h-5 w-16" />
        ))}
      </div>
      {/* Content skeleton */}
      <div className="flex-1 space-y-4 p-6">
        <div className="space-y-2">
          <Skeleton className="h-4 w-24" />
          <Skeleton className="h-3 w-64" />
        </div>
        <Skeleton className="h-[300px] w-full rounded-md" />
        <div className="flex items-center justify-between">
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-7 w-16 rounded" />
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Error state
// ---------------------------------------------------------------------------

function AgentNotFound({ onBack }: { onBack: () => void }) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-4 p-8">
      <Bot className="h-12 w-12 text-muted-foreground/30" />
      <div className="text-center">
        <h2 className="text-lg font-semibold">Agent not found</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          This agent may have been deleted or you may not have access to it.
        </p>
      </div>
      <Button variant="outline" size="sm" onClick={onBack}>
        <ArrowLeft className="h-4 w-4" />
        Back to Agents
      </Button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function AgentDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const router = useRouter();
  const [agent, setAgent] = useState<Agent | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const runtimes = useRuntimeStore((s) => s.runtimes);
  const fetchRuntimes = useRuntimeStore((s) => s.fetchRuntimes);

  useEffect(() => {
    fetchRuntimes();
  }, [fetchRuntimes]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(false);

    api
      .getAgent(id)
      .then((data) => {
        if (!cancelled) {
          setAgent(data);
          setLoading(false);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setError(true);
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [id]);

  const handleUpdate = async (agentId: string, data: Partial<Agent>) => {
    try {
      const updated = await api.updateAgent(agentId, data as UpdateAgentRequest);
      setAgent(updated);
      toast.success("Agent updated");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update agent");
      throw e;
    }
  };

  const handleArchive = async (agentId: string) => {
    try {
      await api.archiveAgent(agentId);
      const updated = await api.getAgent(agentId);
      setAgent(updated);
      toast.success("Agent archived");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to archive agent");
    }
  };

  const handleRestore = async (agentId: string) => {
    try {
      await api.restoreAgent(agentId);
      const updated = await api.getAgent(agentId);
      setAgent(updated);
      toast.success("Agent restored");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to restore agent");
    }
  };

  if (loading) {
    return <AgentDetailSkeleton />;
  }

  if (error || !agent) {
    return <AgentNotFound onBack={() => router.push("/agents")} />;
  }

  return (
    <AgentDetail
      agent={agent}
      runtimes={runtimes}
      onUpdate={handleUpdate}
      onArchive={handleArchive}
      onRestore={handleRestore}
    />
  );
}
