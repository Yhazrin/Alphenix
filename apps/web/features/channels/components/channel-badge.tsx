"use client";

import { useChannelStore } from "../store";
import { cn } from "@/lib/utils";

export function ChannelBadge({
  channelId,
  className,
}: {
  channelId: string;
  className?: string;
}) {
  const label = useChannelStore((s) => {
    const c = s.channels.find((x) => x.id === channelId);
    return c?.name ?? c?.slug ?? "Channel";
  });

  return (
    <span
      className={cn(
        "inline-flex max-w-[7.5rem] shrink-0 truncate rounded-md bg-muted/70 px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground",
        className,
      )}
      title={label}
    >
      {label}
    </span>
  );
}
