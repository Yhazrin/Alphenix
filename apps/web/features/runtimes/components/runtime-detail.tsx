import { useState } from "react";
import { toast } from "sonner";
import type { AgentRuntime } from "@/shared/types";
import { runtimesApi } from "@/shared/api";
import { useWorkspaceStore, useActorName } from "@/features/workspace";
import { formatLastSeen } from "../utils";
import { timeAgo } from "@/shared/utils";
import { RuntimeModeIcon, StatusBadge, InfoField, ApprovalStatusBadge } from "./shared";
import { PingSection } from "./ping-section";
import { UpdateSection } from "./update-section";
import { UsageSection } from "./usage-section";
import { AuditLogSection } from "./audit-log-section";
import { JoinTokenSection } from "./join-token-section";
import { Button } from "@/components/ui/button";
import { Pause, Play, Ban, RotateCcw } from "lucide-react";

function getCliVersion(metadata: Record<string, unknown>): string | null {
  if (
    metadata &&
    typeof metadata.cli_version === "string" &&
    metadata.cli_version
  ) {
    return metadata.cli_version;
  }
  return null;
}

function getTags(metadata: Record<string, unknown>): string[] {
  if (metadata && Array.isArray(metadata.tags)) {
    return metadata.tags as string[];
  }
  return [];
}

export function RuntimeDetail({ runtime, onUpdate }: { runtime: AgentRuntime; onUpdate?: (updated: AgentRuntime) => void }) {
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const { getMemberName } = useActorName();
  const cliVersion =
    runtime.runtime_mode === "local" ? getCliVersion(runtime.metadata) : null;
  const tags = getTags(runtime.metadata);

  const handleAction = async (action: string, fn: () => Promise<AgentRuntime>) => {
    setActionLoading(action);
    try {
      const updated = await fn();
      onUpdate?.(updated);
      toast.success(`${action} successful`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : `${action} failed`);
    } finally {
      setActionLoading(null);
    }
  };

  const isPending = runtime.approval_status === "pending";
  const isApproved = runtime.approval_status === "approved";
  const canTakeActions = isApproved && runtime.status === "online";

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
        <div className="flex min-w-0 items-center gap-2">
          <div
            className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-md ${
              runtime.status === "online" ? "bg-success/10" : "bg-muted"
            }`}
          >
            <RuntimeModeIcon mode={runtime.runtime_mode} />
          </div>
          <div className="min-w-0">
            <h2 className="text-sm font-semibold truncate">{runtime.name}</h2>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <ApprovalStatusBadge status={runtime.approval_status} />
          <StatusBadge status={runtime.status} />
        </div>
      </div>

      {/* Action buttons for pending runtimes */}
      {isPending && (
        <div className="flex items-center gap-2 border-b bg-warning/5 px-4 py-2">
          <span className="text-xs text-warning">This runtime needs approval</span>
          <Button
            size="xs"
            variant="outline"
            onClick={() => handleAction("approve", () => runtimesApi.approve(runtime.id))}
            disabled={actionLoading === "approve"}
          >
            {actionLoading === "approve" ? "Approving..." : "Approve"}
          </Button>
          <Button
            size="xs"
            variant="ghost"
            onClick={() => handleAction("reject", () => runtimesApi.reject(runtime.id))}
            disabled={actionLoading === "reject"}
          >
            {actionLoading === "reject" ? "Rejecting..." : "Reject"}
          </Button>
        </div>
      )}

      {/* Action buttons for approved runtimes */}
      {isApproved && canTakeActions && (
        <div className="flex items-center gap-2 border-b px-4 py-2">
          <Button
            size="xs"
            variant="outline"
            onClick={() => handleAction("pause", () => runtimesApi.pause(runtime.id))}
            disabled={actionLoading === "pause" || runtime.paused}
          >
            <Pause className="h-3 w-3 mr-1" />
            {actionLoading === "pause" ? "Pausing..." : "Pause"}
          </Button>
          {runtime.paused && (
            <Button
              size="xs"
              variant="outline"
              onClick={() => handleAction("resume", () => runtimesApi.resume(runtime.id))}
              disabled={actionLoading === "resume"}
            >
              <Play className="h-3 w-3 mr-1" />
              {actionLoading === "resume" ? "Resuming..." : "Resume"}
            </Button>
          )}
          <Button
            size="xs"
            variant="ghost"
            onClick={() => handleAction("revoke", () => runtimesApi.revoke(runtime.id))}
            disabled={actionLoading === "revoke"}
            className="text-destructive hover:text-destructive"
          >
            <Ban className="h-3 w-3 mr-1" />
            {actionLoading === "revoke" ? "Revoking..." : "Revoke"}
          </Button>
        </div>
      )}

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        {/* Info grid */}
        <div className="grid grid-cols-2 gap-4">
          <InfoField label="Runtime Mode" value={runtime.runtime_mode} />
          <InfoField label="Provider" value={runtime.provider} />
          <InfoField label="Status" value={runtime.status} />
          <InfoField
            label="Last Seen"
            value={formatLastSeen(runtime.last_seen_at)}
          />
          {runtime.device_info && (
            <InfoField label="Device" value={runtime.device_info} />
          )}
          {runtime.daemon_id && (
            <InfoField label="Daemon ID" value={runtime.daemon_id} mono />
          )}
          {runtime.visibility && (
            <InfoField label="Visibility" value={runtime.visibility} />
          )}
          {runtime.trust_level && (
            <InfoField label="Trust Level" value={runtime.trust_level} />
          )}
          {runtime.last_claimed_at && (
            <InfoField label="Last Claimed" value={timeAgo(runtime.last_claimed_at)} />
          )}
          {runtime.max_concurrent_tasks_override !== null && (
            <InfoField label="Max Tasks Override" value={String(runtime.max_concurrent_tasks_override)} />
          )}
          {runtime.drain_mode && (
            <InfoField label="Drain Mode" value="Active" />
          )}
          {runtime.owner_user_id && (
            <InfoField label="Owner" value={getMemberName(runtime.owner_user_id)} />
          )}
        </div>

        {/* Tags */}
        {tags.length > 0 && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-2">Tags</h3>
            <div className="flex flex-wrap gap-1">
              {tags.map((tag) => (
                <span
                  key={tag}
                  className="rounded bg-muted px-2 py-0.5 text-xs text-muted-foreground"
                >
                  {tag}
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Health metrics */}
        {(runtime.success_count_24h > 0 || runtime.failure_count_24h > 0) && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-2">24h Health</h3>
            <div className="grid grid-cols-3 gap-2">
              <div className="rounded border px-3 py-2 text-center">
                <div className="text-sm font-semibold text-success">{runtime.success_count_24h}</div>
                <div className="text-xs text-muted-foreground">Success</div>
              </div>
              <div className="rounded border px-3 py-2 text-center">
                <div className="text-sm font-semibold text-destructive">{runtime.failure_count_24h}</div>
                <div className="text-xs text-muted-foreground">Failed</div>
              </div>
              <div className="rounded border px-3 py-2 text-center">
                <div className="text-sm font-semibold">
                  {runtime.avg_task_duration_ms > 0
                    ? `${Math.round(runtime.avg_task_duration_ms / 1000)}s`
                    : "-"}
                </div>
                <div className="text-xs text-muted-foreground">Avg Time</div>
              </div>
            </div>
          </div>
        )}

        {/* CLI Version & Update */}
        {runtime.runtime_mode === "local" && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-3">
              CLI Version
            </h3>
            <UpdateSection
              runtimeId={runtime.id}
              currentVersion={cliVersion}
              isOnline={runtime.status === "online"}
            />
          </div>
        )}

        {/* Connection Test */}
        <div>
          <h3 className="text-xs font-medium text-muted-foreground mb-3">
            Connection Test
          </h3>
          <PingSection runtimeId={runtime.id} />
        </div>

        {/* Usage */}
        <div>
          <h3 className="text-xs font-medium text-muted-foreground mb-3">
            Token Usage
          </h3>
          <UsageSection runtimeId={runtime.id} />
        </div>

        {/* Audit Log */}
        <div>
          <h3 className="text-xs font-medium text-muted-foreground mb-3">
            Audit Log
          </h3>
          <AuditLogSection runtimeId={runtime.id} />
        </div>

        {/* Join Tokens */}
        {workspace && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-3">
              Join Tokens
            </h3>
            <JoinTokenSection workspaceId={workspace.id} />
          </div>
        )}

        {/* Metadata */}
        {runtime.metadata && Object.keys(runtime.metadata).length > 0 && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-2">
              Metadata
            </h3>
            <div className="rounded-lg border bg-muted/30 p-3">
              <pre className="text-xs font-mono whitespace-pre-wrap break-all">
                {JSON.stringify(runtime.metadata, null, 2)}
              </pre>
            </div>
          </div>
        )}

        {/* Timestamps */}
        <div className="grid grid-cols-2 gap-4 border-t pt-4">
          <InfoField
            label="Created"
            value={new Date(runtime.created_at).toLocaleString()}
          />
          <InfoField
            label="Updated"
            value={new Date(runtime.updated_at).toLocaleString()}
          />
        </div>
      </div>
    </div>
  );
}
