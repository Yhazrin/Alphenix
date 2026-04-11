"use client";

import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

export type ScopePillItem<T extends string = string> = {
  value: T;
  label: string;
  description: string;
  /** Optional trailing badge (e.g. issue count) */
  badge?: React.ReactNode;
};

type ScopePillRailProps<T extends string> = {
  items: ScopePillItem<T>[];
  value: T;
  onChange: (next: T) => void;
  className?: string;
};

/**
 * Magazine-style scope control: frosted pill container with full-round segments.
 */
export function ScopePillRail<T extends string>({
  items,
  value,
  onChange,
  className,
}: ScopePillRailProps<T>) {
  return (
    <div
      className={cn(
        "inline-flex rounded-full border border-border/45 bg-background/70 p-0.5 shadow-sm backdrop-blur-xl dark:border-border/35 dark:bg-background/40",
        className,
      )}
      role="tablist"
      aria-label="Scope"
    >
      <div className="flex items-center gap-0.5">
        {items.map((item) => {
          const selected = value === item.value;
          return (
            <Tooltip key={item.value}>
              <TooltipTrigger
                render={
                  <button
                    type="button"
                    role="tab"
                    aria-selected={selected}
                    data-testid={`scope-pill-${item.value}`}
                    className={cn(
                      "inline-flex min-h-8 items-center gap-1.5 rounded-full px-3 py-1 text-sm transition-colors duration-200 ease-out outline-none",
                      "focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
                      selected
                        ? "bg-primary/15 font-medium text-foreground dark:bg-primary/22"
                        : "text-muted-foreground hover:text-foreground",
                    )}
                    onClick={() => onChange(item.value)}
                  >
                    <span>{item.label}</span>
                    {item.badge != null && (
                      <span className="text-xs tabular-nums opacity-60">{item.badge}</span>
                    )}
                  </button>
                }
              />
              <TooltipContent side="bottom">{item.description}</TooltipContent>
            </Tooltip>
          );
        })}
      </div>
    </div>
  );
}
