"use client";

import { useState, useEffect, useRef } from "react";
import {
  ShieldCheck,
  AlertCircle,
  CheckCircle2,
  XCircle,
  Pause,
  Play,
  Ban,
  Droplets,
  Link2,
} from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import type { RuntimeAuditLog } from "@/shared/types";
import { runtimesApi } from "@/shared/api";

const ACTION_CONFIG: Record<string, { label: string; color: string; icon: typeof CheckCircle2 }> = {
  runtime_approved: { label: "Approved", color: "text-success", icon: CheckCircle2 },
  runtime_rejected: { label: "Rejected", color: "text-destructive", icon: XCircle },
  runtime_paused: { label: "Paused", color: "text-warning", icon: Pause },
  runtime_resumed: { label: "Resumed", color: "text-info", icon: Play },
  runtime_revoked: { label: "Revoked", color: "text-destructive", icon: Ban },
  runtime_drained: { label: "Drained", color: "text-muted-foreground", icon: Droplets },
  runtime_join_requested: { label: "Join Requested", color: "text-info", icon: Link2 },
};

function formatAction(action: string): { label: string; color: string; icon: typeof CheckCircle2 } {
  return ACTION_CONFIG[action] ?? { label: action, color: "text-muted-foreground", icon: ShieldCheck };
}

function formatDetails(details: Record<string, unknown>): string | null {
  const parts: string[] = [];
  for (const [key, value] of Object.entries(details)) {
    if (typeof value === "string" && value) {
      parts.push(`${key}: ${value}`);
    }
  }
  return parts.length > 0 ? parts.join(" · ") : null;
}

export function AuditLogSection({ runtimeId }: { runtimeId: string }) {
  const [logs, setLogs] = useState<RuntimeAuditLog[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    runtimesApi
      .getRuntimeAuditLogs(runtimeId)
      .then((data) => { if (!cancelled) setLogs(data); })
      .catch((e: unknown) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : "Failed to load audit logs");
        setLogs([]);
      })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [runtimeId]);

  if (loading) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="flex gap-3">
            <Skeleton className="h-6 w-6 rounded-full shrink-0" />
            <div className="flex-1 space-y-1.5">
              <Skeleton className="h-4 w-1/3" />
              <Skeleton className="h-3 w-1/2" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center rounded-lg border border-dashed py-6">
        <AlertCircle className="h-5 w-5 text-destructive/60" aria-hidden="true" />
        <p className="mt-2 text-xs text-destructive">{error}</p>
      </div>
    );
  }

  if (!logs || logs.length === 0) {
    return (
      <div className="flex flex-col items-center rounded-lg border border-dashed py-6">
        <ShieldCheck className="h-5 w-5 text-muted-foreground/40" aria-hidden="true" />
        <p className="mt-2 text-xs text-muted-foreground">No audit events yet</p>
      </div>
    );
  }

  return (
    <div className="relative space-y-0">
      {/* Vertical line */}
      <div className="absolute left-[11px] top-3 bottom-3 w-px bg-border" />

      {logs.map((log) => {
        const config = formatAction(log.action);
        const Icon = config.icon;
        const detailText = formatDetails(log.details);

        return (
          <div key={log.id} className="relative flex gap-3 py-2.5">
            <div className="relative z-10 flex h-6 w-6 shrink-0 items-center justify-center rounded-full border bg-background">
              <Icon className={`h-3 w-3 ${config.color}`} aria-hidden="true" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-xs font-medium">{config.label}</span>
                <Badge variant="outline" className="text-[10px]">
                  {log.action}
                </Badge>
              </div>
              {detailText && (
                <p className="mt-0.5 text-[11px] text-muted-foreground">{detailText}</p>
              )}
              <span className="text-[10px] text-muted-foreground">
                {(() => { const d = new Date(log.created_at); return isNaN(d.getTime()) ? "" : d.toLocaleString(); })()}
              </span>
            </div>
          </div>
        );
      })}
    </div>
  );
}
