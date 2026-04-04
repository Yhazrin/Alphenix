"use client";

import { useState, useMemo } from "react";
import { GitMerge } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
import { toast } from "sonner";
import { CollapsibleSection } from "./collapsible-section";

interface ChainTaskSectionProps {
  taskId: string;
}

export function ChainTaskSection({ taskId }: ChainTaskSectionProps) {
  const [chainAgentId, setChainAgentId] = useState("");
  const [chainReason, setChainReason] = useState("");
  const [showChain, setShowChain] = useState(false);

  const agents = useWorkspaceStore((s) => s.agents);
  const activeAgents = useMemo(() => agents.filter((a) => !a.archived_at), [agents]);

  const handleChainTask = async () => {
    if (!taskId || !chainAgentId) return;
    try {
      await api.chainTask(taskId, { target_agent_id: chainAgentId, chain_reason: chainReason.trim() || undefined });
      setChainAgentId("");
      setChainReason("");
      setShowChain(false);
      toast.success("Task chained successfully");
    } catch {
      toast.error("Failed to chain task");
    }
  };

  return (
    <CollapsibleSection
      title="Chain Task"
      icon={<GitMerge className="h-3.5 w-3.5 text-muted-foreground" />}
      defaultOpen={false}
    >
      {showChain ? (
        <div className="space-y-2">
          <Select value={chainAgentId} onValueChange={(v) => setChainAgentId(v ?? "")}>
            <SelectTrigger className="h-7 text-xs">
              <SelectValue placeholder="Select target agent..." />
            </SelectTrigger>
            <SelectContent>
              {activeAgents.map((a) => (
                <SelectItem key={a.id} value={a.id} className="text-xs">
                  {a.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Input
            value={chainReason}
            onChange={(e) => setChainReason(e.target.value)}
            placeholder="Reason (optional)..."
            className="h-8 text-xs"
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                handleChainTask();
              }
              if (e.key === "Escape") {
                setShowChain(false);
                setChainAgentId("");
                setChainReason("");
              }
            }}
          />
          <div className="flex gap-1.5">
            <Button size="sm" className="h-7 text-xs flex-1" onClick={handleChainTask} disabled={!chainAgentId}>
              <GitMerge className="h-3 w-3 mr-1" /> Chain task
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="h-7 text-xs"
              onClick={() => {
                setShowChain(false);
                setChainAgentId("");
                setChainReason("");
              }}
            >
              Cancel
            </Button>
          </div>
        </div>
      ) : (
        <Button
          variant="ghost"
          size="sm"
          className="h-6 text-xs text-muted-foreground w-full"
          onClick={() => setShowChain(true)}
        >
          <GitMerge className="h-3 w-3 mr-1" />
          Chain to another agent
        </Button>
      )}
    </CollapsibleSection>
  );
}
