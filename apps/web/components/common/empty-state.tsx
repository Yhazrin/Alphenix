import type { LucideIcon } from "lucide-react";
import { Button } from "@/components/ui/button";

interface EmptyStateAction {
  label: string;
  onClick: () => void;
  icon?: LucideIcon;
}

interface EmptyStateProps {
  icon?: LucideIcon;
  title: string;
  description?: string;
  actions?: EmptyStateAction[];
}

export function EmptyState({
  icon: Icon,
  title,
  description,
  actions,
}: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-12 text-center">
      {Icon && (
        <div className="flex size-12 items-center justify-center rounded-full bg-muted">
          <Icon className="size-6 text-muted-foreground" aria-hidden="true" />
        </div>
      )}
      <div className="space-y-1">
        <p className="text-sm font-medium">{title}</p>
        {description && (
          <p className="text-sm text-muted-foreground">{description}</p>
        )}
      </div>
      {actions && actions.length > 0 && (
        <div className="flex gap-2 pt-1">
          {actions.map((action) => (
            <Button
              key={action.label}
              variant="secondary"
              size="sm"
              onClick={action.onClick}
            >
              {action.icon && (
                <action.icon className="mr-1.5 size-3.5" aria-hidden="true" />
              )}
              {action.label}
            </Button>
          ))}
        </div>
      )}
    </div>
  );
}
