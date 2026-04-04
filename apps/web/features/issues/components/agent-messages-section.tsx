"use client";

import { useState, useMemo, memo } from "react";
import { MessageSquare, Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
import { toast } from "sonner";
import { timeAgo } from "@/shared/utils";
import type { AgentMessage } from "@/shared/types";
import { CollapsibleSection } from "./collapsible-section";

function useAgentName(agentId: string): string {
  const agents = useWorkspaceStore((s) => s.agents);
  const agent = agents.find((a) => a.id === agentId);
  return agent?.name ?? `Agent ${agentId.slice(0, 8)}`;
}

const AgentLabel = memo(function AgentLabel({ agentId }: { agentId: string }) {
  const name = useAgentName(agentId);
  return <span className="font-medium truncate">{name}</span>;
});

interface AgentMessagesSectionProps {
  agentId: string;
  taskId: string | undefined;
  messages: AgentMessage[];
  messagesLoading: boolean;
  onMessageSent: (msg: AgentMessage) => void;
}

export function AgentMessagesSection({
  agentId,
  taskId,
  messages,
  messagesLoading,
  onMessageSent,
}: AgentMessagesSectionProps) {
  const [replyText, setReplyText] = useState("");
  const [selectedAgentId, setSelectedAgentId] = useState<string>("");

  const agents = useWorkspaceStore((s) => s.agents);
  const activeAgents = useMemo(() => agents.filter((a) => !a.archived_at), [agents]);

  const handleSendMessage = async () => {
    if (!agentId || !replyText.trim() || !selectedAgentId) return;
    try {
      const msg = await api.sendAgentMessage(agentId, {
        to_agent_id: selectedAgentId,
        content: replyText.trim(),
        message_type: "info",
        task_id: taskId ?? undefined,
      });
      onMessageSent(msg);
      setReplyText("");
      toast.success("Message sent");
    } catch {
      toast.error("Failed to send message");
    }
  };

  return (
    <CollapsibleSection
      title="Agent Messages"
      icon={<MessageSquare className="h-3.5 w-3.5 text-muted-foreground" />}
      count={messages.length}
      defaultOpen={messages.length > 0}
    >
      {messagesLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      ) : messages.length === 0 ? (
        <p className="text-xs text-muted-foreground py-1">No messages yet.</p>
      ) : (
        <div className="space-y-2 max-h-60 overflow-y-auto">
          {messages.map((msg) => (
            <div key={msg.id} className="flex items-start gap-2 text-xs">
              <ActorAvatar actorType="agent" actorId={msg.from_agent_id} size={20} />
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-1.5">
                  <AgentLabel agentId={msg.from_agent_id} />
                  <span className="text-muted-foreground">
                    {timeAgo(msg.created_at)}
                  </span>
                  <Badge variant="outline" className="text-[9px] px-1 py-0">
                    {msg.message_type}
                  </Badge>
                </div>
                <p className="text-muted-foreground mt-0.5 break-words">{msg.content}</p>
              </div>
            </div>
          ))}
        </div>
      )}
      {/* Send message */}
      <div className="mt-2 space-y-1.5">
        <Select value={selectedAgentId} onValueChange={(v) => setSelectedAgentId(v ?? "")}>
          <SelectTrigger className="h-7 text-xs">
            <SelectValue placeholder="Select recipient..." />
          </SelectTrigger>
          <SelectContent>
            {activeAgents.map((a) => (
              <SelectItem key={a.id} value={a.id} className="text-xs">
                {a.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className="flex gap-1.5">
          <Input
            value={replyText}
            onChange={(e) => setReplyText(e.target.value)}
            placeholder="Send a message to agents..."
            className="h-8 text-xs"
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                handleSendMessage();
              }
            }}
          />
          <Button
            size="sm"
            variant="ghost"
            className="h-8 w-8 p-0 shrink-0"
            onClick={handleSendMessage}
            disabled={!replyText.trim() || !selectedAgentId}
          >
            <Send className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </CollapsibleSection>
  );
}
