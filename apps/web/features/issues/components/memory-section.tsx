"use client";

import { useState, useMemo } from "react";
import { Brain, Trash2, Plus, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { api } from "@/shared/api";
import { toast } from "sonner";
import { timeAgo } from "@/shared/utils";
import type { AgentMemory } from "@/shared/types";
import { CollapsibleSection } from "./collapsible-section";

interface MemorySectionProps {
  agentId: string;
  memories: AgentMemory[];
  memLoading: boolean;
  memoriesLoaded: boolean;
  onLoadMemories: () => void;
  onSetMemoriesLoaded: (loaded: boolean) => void;
  onMemoryStored: (mem: AgentMemory) => void;
  onMemoryDeleted: (memoryId: string) => void;
}

export function MemorySection({
  agentId,
  memories,
  memLoading,
  memoriesLoaded,
  onLoadMemories,
  onSetMemoriesLoaded,
  onMemoryStored,
  onMemoryDeleted,
}: MemorySectionProps) {
  const [memoryContent, setMemoryContent] = useState("");
  const [memorySearch, setMemorySearch] = useState("");

  const filteredMemories = useMemo(() => {
    if (!memorySearch.trim()) return memories;
    const q = memorySearch.toLowerCase();
    return memories.filter((m) => m.content.toLowerCase().includes(q));
  }, [memories, memorySearch]);

  const handleStoreMemory = async () => {
    if (!agentId || !memoryContent.trim()) return;
    try {
      const mem = await api.storeAgentMemory(agentId, { content: memoryContent.trim() });
      onMemoryStored(mem);
      setMemoryContent("");
      toast.success("Memory stored");
    } catch {
      toast.error("Failed to store memory");
    }
  };

  const handleDeleteMemory = async (memoryId: string) => {
    if (!agentId) return;
    try {
      await api.deleteAgentMemory(agentId, memoryId);
      onMemoryDeleted(memoryId);
      toast.success("Memory deleted");
    } catch {
      toast.error("Failed to delete memory");
    }
  };

  return (
    <CollapsibleSection
      title="Memory"
      icon={<Brain className="h-3.5 w-3.5 text-muted-foreground" />}
      count={memories.length}
      onOpen={() => {
        if (!memoriesLoaded) {
          onSetMemoriesLoaded(true);
          onLoadMemories();
        }
      }}
    >
      {memLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-6 w-full" />
          <Skeleton className="h-6 w-full" />
        </div>
      ) : memories.length === 0 ? (
        <p className="text-xs text-muted-foreground py-1">No memories stored.</p>
      ) : (
        <>
          {memories.length > 3 && (
            <div className="relative mb-1.5">
              <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3 w-3 text-muted-foreground" />
              <Input
                value={memorySearch}
                onChange={(e) => setMemorySearch(e.target.value)}
                placeholder="Search memories..."
                className="h-7 pl-7 text-xs"
              />
            </div>
          )}
          <div className="space-y-2 max-h-48 overflow-y-auto">
            {filteredMemories.length === 0 ? (
              <p className="text-xs text-muted-foreground py-1">No matching memories.</p>
            ) : (
              filteredMemories.map((mem) => (
                <div key={mem.id} className="group rounded-md border px-2 py-1.5">
                  <div className="flex items-start justify-between gap-2">
                    <p className="text-xs text-muted-foreground flex-1 break-words">{mem.content}</p>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      className="h-5 w-5 p-0 opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
                      onClick={() => handleDeleteMemory(mem.id)}
                    >
                      <Trash2 className="h-3 w-3 text-destructive" />
                    </Button>
                  </div>
                  <div className="mt-1 flex items-center gap-2">
                    <span className="text-[10px] text-muted-foreground">
                      {timeAgo(mem.created_at)}
                    </span>
                    {mem.similarity !== undefined && (
                      <Badge variant="outline" className="text-[9px] px-1 py-0">
                        {(mem.similarity * 100).toFixed(0)}% match
                      </Badge>
                    )}
                  </div>
                </div>
              ))
            )}
          </div>
        </>
      )}
      <div className="mt-2 flex gap-1.5">
        <Input
          value={memoryContent}
          onChange={(e) => setMemoryContent(e.target.value)}
          placeholder="Store a memory..."
          className="h-8 text-xs"
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              handleStoreMemory();
            }
          }}
        />
        <Button
          size="sm"
          variant="ghost"
          className="h-8 w-8 p-0 shrink-0"
          onClick={handleStoreMemory}
          disabled={!memoryContent.trim()}
        >
          <Plus className="h-3.5 w-3.5" />
        </Button>
      </div>
    </CollapsibleSection>
  );
}
