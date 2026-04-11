"use client";

import { useEffect } from "react";
import { Check, ChevronDown, Hash } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { useWorkspaceStore } from "@/features/workspace";
import { useChannelStore } from "../store";
import { cn } from "@/lib/utils";

export function ChannelPicker({
  channelId,
  onSelect,
  disabled,
}: {
  channelId: string;
  onSelect: (nextChannelId: string) => void;
  disabled?: boolean;
}) {
  const workspace = useWorkspaceStore((s) => s.workspace);
  const channels = useChannelStore((s) => s.channels);
  const loading = useChannelStore((s) => s.loading);
  const fetchChannels = useChannelStore((s) => s.fetchChannels);

  useEffect(() => {
    if (!workspace?.id) return;
    if (channels.length === 0 && !loading) void fetchChannels();
  }, [workspace?.id, channels.length, loading, fetchChannels]);

  const sorted = [...channels].sort((a, b) => {
    if (a.is_default && !b.is_default) return -1;
    if (!a.is_default && b.is_default) return 1;
    return a.name.localeCompare(b.name);
  });

  const current = sorted.find((c) => c.id === channelId);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        disabled={disabled}
        className={cn(
          "inline-flex w-full items-center justify-between gap-2 rounded-md border border-border/80 bg-background px-2 py-1.5 text-left text-xs font-medium hover:bg-accent/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50",
        )}
      >
        <span className="flex min-w-0 flex-1 items-center gap-1.5">
          <Hash className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
          <span className="truncate">{current?.name ?? (loading ? "Loading…" : "Select channel")}</span>
        </span>
        <ChevronDown className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-[var(--radix-dropdown-menu-trigger-width)] min-w-56 max-h-72 overflow-y-auto">
        {sorted.map((ch) => (
          <DropdownMenuItem key={ch.id} onClick={() => onSelect(ch.id)}>
            <Hash className="size-3.5 text-muted-foreground" aria-hidden="true" />
            <span className="flex-1 truncate">{ch.name}</span>
            {ch.is_default && (
              <span className="text-[10px] text-muted-foreground">Default</span>
            )}
            {ch.id === channelId && <Check className="ml-auto size-3.5 shrink-0" aria-hidden="true" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
