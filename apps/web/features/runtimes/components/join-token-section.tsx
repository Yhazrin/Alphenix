"use client";

import { useState, useEffect, useCallback } from "react";
import {
  KeyRound,
  AlertCircle,
  Copy,
  Check,
  Plus,
  Clock,
  CheckCircle2,
} from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { toast } from "sonner";
import type { RuntimeJoinToken, CreateRuntimeJoinTokenResponse } from "@/shared/types";
import { runtimesApi } from "@/shared/api";

function formatExpiry(expiresAt: string): string {
  const date = new Date(expiresAt);
  if (isNaN(date.getTime())) return "—";
  const now = new Date();
  if (date < now) return "Expired";
  const diffMs = date.getTime() - now.getTime();
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  if (diffHours < 1) {
    const diffMinutes = Math.floor(diffMs / (1000 * 60));
    return `${diffMinutes}m left`;
  }
  if (diffHours < 24) return `${diffHours}h left`;
  const diffDays = Math.floor(diffHours / 24);
  return `${diffDays}d left`;
}

function TokenRow({ token }: { token: RuntimeJoinToken }) {
  const expDate = new Date(token.expires_at);
  const isExpired = !isNaN(expDate.getTime()) && expDate < new Date();
  const isUsed = token.used_at !== null;

  return (
    <div className="flex items-center justify-between rounded-lg border px-3 py-2">
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <code className="text-xs font-mono">{token.token_prefix}•••</code>
          {isUsed && (
            <Badge variant="outline" className="text-[10px] text-success">
              <CheckCircle2 className="mr-0.5 h-2.5 w-2.5" />
              Used
            </Badge>
          )}
          {isExpired && !isUsed && (
            <Badge variant="outline" className="text-[10px] text-muted-foreground">
              Expired
            </Badge>
          )}
        </div>
        <div className="mt-0.5 flex items-center gap-2 text-[10px] text-muted-foreground">
          <Clock className="h-2.5 w-2.5" />
          {formatExpiry(token.expires_at)}
          <span>·</span>
          <span>Created {new Date(token.created_at).toLocaleDateString()}</span>
        </div>
      </div>
    </div>
  );
}

export function JoinTokenSection({ workspaceId }: { workspaceId: string }) {
  const [tokens, setTokens] = useState<RuntimeJoinToken[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [newToken, setNewToken] = useState<CreateRuntimeJoinTokenResponse | null>(null);
  const [copied, setCopied] = useState(false);

  const fetchTokens = useCallback(() => {
    setLoading(true);
    setError(null);
    runtimesApi
      .listJoinTokens(workspaceId)
      .then(setTokens)
      .catch((e: unknown) => {
        setError(e instanceof Error ? e.message : "Failed to load join tokens");
        setTokens([]);
      })
      .finally(() => setLoading(false));
  }, [workspaceId]);

  useEffect(() => {
    fetchTokens();
  }, [fetchTokens]);

  const handleCreate = async () => {
    setCreating(true);
    try {
      const result = await runtimesApi.createJoinToken(workspaceId);
      setNewToken(result);
      fetchTokens();
      toast.success("Join token created");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to create join token");
    } finally {
      setCreating(false);
    }
  };

  const handleCopy = async () => {
    if (!newToken) return;
    try {
      await navigator.clipboard.writeText(newToken.token);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error("Failed to copy token");
    }
  };

  if (loading) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center rounded-lg border border-dashed py-4">
        <AlertCircle className="h-4 w-4 text-destructive/60" aria-hidden="true" />
        <p className="mt-1.5 text-xs text-destructive">{error}</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {/* Newly created token display */}
      {newToken && (
        <div className="rounded-lg border border-success/30 bg-success/5 p-3">
          <p className="text-xs font-medium text-success mb-1">New join token generated</p>
          <div className="flex items-center gap-2">
            <code className="flex-1 rounded bg-background px-2 py-1 text-xs font-mono break-all">
              {newToken.token}
            </code>
            <Button size="xs" variant="outline" onClick={handleCopy}>
              {copied ? (
                <Check className="h-3 w-3 text-success" />
              ) : (
                <Copy className="h-3 w-3" />
              )}
            </Button>
          </div>
          <p className="mt-1 text-[10px] text-muted-foreground">
            Expires {formatExpiry(newToken.expires_at)} — copy this token now, it won't be shown again
          </p>
        </div>
      )}

      {/* Token list */}
      {tokens && tokens.length > 0 ? (
        <div className="space-y-1.5">
          {tokens.map((token) => (
            <TokenRow key={token.id} token={token} />
          ))}
        </div>
      ) : (
        <div className="flex flex-col items-center rounded-lg border border-dashed py-4">
          <KeyRound className="h-4 w-4 text-muted-foreground/40" aria-hidden="true" />
          <p className="mt-1.5 text-xs text-muted-foreground">No join tokens yet</p>
        </div>
      )}

      <Button size="sm" variant="outline" onClick={handleCreate} disabled={creating}>
        <Plus className="mr-1 h-3 w-3" />
        {creating ? "Generating..." : "Generate Join Token"}
      </Button>
    </div>
  );
}
