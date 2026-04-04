"use client";

import { useState, useCallback, memo } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { Badge } from "@/components/ui/badge";

export interface SectionProps {
  title: string;
  icon: React.ReactNode;
  count?: number;
  defaultOpen?: boolean;
  onOpen?: () => void;
  children: React.ReactNode;
}

export const CollapsibleSection = memo(function CollapsibleSection({
  title,
  icon,
  count,
  defaultOpen = false,
  onOpen,
  children,
}: SectionProps) {
  const [open, setOpen] = useState(defaultOpen);

  const handleToggle = useCallback(() => {
    setOpen((prev) => {
      if (!prev && onOpen) onOpen();
      return !prev;
    });
  }, [onOpen]);

  return (
    <div className="rounded-lg border bg-card">
      <button
        className="flex w-full items-center gap-2 px-3 py-2 text-sm font-medium hover:bg-muted/50 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        onClick={handleToggle}
        aria-expanded={open}
      >
        {open ? (
          <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" aria-hidden="true" />
        ) : (
          <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" aria-hidden="true" />
        )}
        {icon}
        <span className="flex-1 text-left">{title}</span>
        {count !== undefined && count > 0 && (
          <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
            {count}
          </Badge>
        )}
      </button>
      {open && <div className="border-t px-3 py-2.5">{children}</div>}
    </div>
  );
});
