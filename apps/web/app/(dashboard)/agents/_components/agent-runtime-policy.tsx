"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { Shield, Plus, Trash2, Save, Loader2, Server, AlertCircle } from "lucide-react";
import type { Agent, RuntimePolicy, RuntimeDevice } from "@/shared/types";
import { runtimesApi } from "@/shared/api/runtimes";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/common/empty-state";

function TagInput({
  label,
  tags,
  onChange,
  placeholder,
  "data-testid": testId,
}: {
  label: string;
  tags: string[];
  onChange: (tags: string[]) => void;
  placeholder?: string;
  "data-testid"?: string;
}) {
  const [input, setInput] = useState("");

  const addTag = () => {
    const tag = input.trim().toLowerCase();
    if (tag && !tags.includes(tag)) {
      onChange([...tags, tag]);
    }
    setInput("");
  };

  return (
    <div>
      <Label className="text-xs text-muted-foreground">{label}</Label>
      <div className="mt-1.5 flex flex-wrap gap-1.5">
        {tags.map((tag) => (
          <Badge key={tag} variant="secondary" className="gap-1 text-xs">
            {tag}
            <button
              className="ml-0.5 text-muted-foreground hover:text-foreground"
              onClick={() => onChange(tags.filter((t) => t !== tag))}
              aria-label={`Remove ${tag}`}
            >
              ×
            </button>
          </Badge>
        ))}
        <Input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              addTag();
            }
          }}
          placeholder={placeholder ?? "Add tag..."}
          className="h-6 w-32 text-xs"
          data-testid={testId}
        />
      </div>
    </div>
  );
}

function RuntimeMultiSelect({
  label,
  runtimeIds,
  runtimes,
  onChange,
}: {
  label: string;
  runtimeIds: string[];
  runtimes: RuntimeDevice[];
  onChange: (ids: string[]) => void;
}) {
  const toggle = (id: string) => {
    if (runtimeIds.includes(id)) {
      onChange(runtimeIds.filter((r) => r !== id));
    } else {
      onChange([...runtimeIds, id]);
    }
  };

  return (
    <div>
      <Label className="text-xs text-muted-foreground">{label}</Label>
      <div className="mt-1.5 space-y-1">
        {runtimes.length === 0 ? (
          <EmptyState
            className="border-0 py-6"
            icon={Server}
            title="No runtimes connected"
            description="Install the daemon first to make runtimes available for scheduling."
          >
            <code className="rounded bg-muted px-2 py-1 text-xs">
              alphenix daemon start
            </code>
          </EmptyState>
        ) : (
          runtimes.map((rt) => (
            <label
              key={rt.id}
              className="flex items-center gap-2 rounded-md border px-3 py-1.5 text-xs cursor-pointer hover:bg-muted/30"
            >
              <input
                type="checkbox"
                checked={runtimeIds.includes(rt.id)}
                onChange={() => toggle(rt.id)}
                className="rounded"
              />
              <Server className="h-3 w-3 text-muted-foreground" />
              <span className="font-medium">{rt.name}</span>
              <Badge variant="outline" className="text-[10px] ml-auto">
                {rt.status}
              </Badge>
            </label>
          ))
        )}
      </div>
    </div>
  );
}

export function RuntimePolicyTab({
  agent,
  runtimes,
}: {
  agent: Agent;
  runtimes: RuntimeDevice[];
}) {
  const [policy, setPolicy] = useState<RuntimePolicy | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Form state
  const [requiredTags, setRequiredTags] = useState<string[]>([]);
  const [forbiddenTags, setForbiddenTags] = useState<string[]>([]);
  const [preferredIds, setPreferredIds] = useState<string[]>([]);
  const [fallbackIds, setFallbackIds] = useState<string[]>([]);
  const [maxQueueDepth, setMaxQueueDepth] = useState(0);
  const [isActive, setIsActive] = useState(true);

  const loadPolicy = useCallback(async () => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    try {
      const p = await runtimesApi.getRuntimePolicyByAgent(agent.id);
      if (cancelled) return;
      setPolicy(p);
      setRequiredTags(p.required_tags);
      setForbiddenTags(p.forbidden_tags);
      setPreferredIds(p.preferred_runtime_ids);
      setFallbackIds(p.fallback_runtime_ids);
      setMaxQueueDepth(p.max_queue_depth);
      setIsActive(p.is_active);
    } catch (e) {
      if (cancelled) return;
      if (e instanceof Error && e.message.includes("not found")) {
        setPolicy(null);
      } else {
        setError(e instanceof Error ? e.message : "Failed to load policy");
      }
    } finally {
      if (!cancelled) setLoading(false);
    }
    return () => { cancelled = true; };
  }, [agent.id]);

  useEffect(() => {
    const cleanup = loadPolicy();
    return () => { cleanup?.then((fn) => fn?.()); };
  }, [loadPolicy]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const data = {
        required_tags: requiredTags,
        forbidden_tags: forbiddenTags,
        preferred_runtime_ids: preferredIds,
        fallback_runtime_ids: fallbackIds,
        max_queue_depth: maxQueueDepth,
        is_active: isActive,
      };
      if (policy) {
        const updated = await runtimesApi.updateRuntimePolicy(policy.id, data);
        setPolicy(updated);
      } else {
        const created = await runtimesApi.createRuntimePolicy({
          agent_id: agent.id,
          ...data,
        });
        setPolicy(created);
      }
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save policy");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!policy) return;
    setSaving(true);
    try {
      await runtimesApi.deleteRuntimePolicy(policy.id);
      setPolicy(null);
      setRequiredTags([]);
      setForbiddenTags([]);
      setPreferredIds([]);
      setFallbackIds([]);
      setMaxQueueDepth(0);
      setIsActive(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to delete policy");
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-20 w-full" />
        <Skeleton className="h-20 w-full" />
      </div>
    );
  }

  if (error) {
    return (
      <EmptyState
        icon={Shield}
        title="Failed to load policy"
        description={error}
        actions={[{ label: "Retry", onClick: loadPolicy }]}
      />
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold">Runtime Scheduling Policy</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            Control which runtimes execute this agent&apos;s tasks using tag-based rules.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-2">
            <Switch checked={isActive} onCheckedChange={setIsActive} />
            <Label className="text-xs">{isActive ? "Active" : "Inactive"}</Label>
          </div>
        </div>
      </div>

      {!policy && (
        <EmptyState
          icon={AlertCircle}
          title="No scheduling policy yet"
          description="Tasks will run on any available runtime. Create a policy to control which runtimes this agent can use."
        />
      )}

      {/* Tag Rules */}
      <div className="space-y-4 rounded-lg border p-4">
        <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
          Tag Rules
        </h4>
        <p className="text-xs text-muted-foreground">
          Tags control which runtimes can execute this agent&apos;s tasks.
        </p>
        <TagInput
          label="Required Tags"
          tags={requiredTags}
          onChange={setRequiredTags}
          placeholder="e.g. gpu, high-memory"
          data-testid="policy-required-tags-input"
        />
        <TagInput
          label="Forbidden Tags"
          tags={forbiddenTags}
          onChange={setForbiddenTags}
          placeholder="e.g. experimental"
          data-testid="policy-forbidden-tags-input"
        />
      </div>

      {/* Runtime Selection */}
      <div className="space-y-4 rounded-lg border p-4">
        <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
          Runtime Selection
        </h4>
        <RuntimeMultiSelect
          label="Preferred Runtimes"
          runtimeIds={preferredIds}
          runtimes={runtimes}
          onChange={setPreferredIds}
        />
        <RuntimeMultiSelect
          label="Fallback Runtimes"
          runtimeIds={fallbackIds}
          runtimes={runtimes}
          onChange={setFallbackIds}
        />
      </div>

      {/* Queue Control */}
      <div className="space-y-4 rounded-lg border p-4">
        <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
          Queue Control
        </h4>
        <div>
          <Label htmlFor="max-queue" className="text-xs text-muted-foreground">
            Max Queue Depth (0 = unlimited)
          </Label>
          <Input
            id="max-queue"
            type="number"
            min={0}
            value={maxQueueDepth}
            onChange={(e) => { const v = parseInt(e.target.value, 10); if (!isNaN(v)) setMaxQueueDepth(v); }}
            className="mt-1 w-32"
          />
        </div>
      </div>

      {/* Actions */}
      <div className="flex items-center justify-between">
        {policy ? (
          <Button
            variant="ghost"
            size="sm"
            onClick={handleDelete}
            disabled={saving}
            className="text-destructive hover:text-destructive"
          >
            <Trash2 className="mr-1.5 h-3.5 w-3.5" />
            Delete Policy
          </Button>
        ) : (
          <div />
        )}
        <Button onClick={handleSave} disabled={saving} size="sm">
          {saving ? (
            <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
          ) : policy ? (
            <Save className="mr-1.5 h-3.5 w-3.5" />
          ) : (
            <Plus className="mr-1.5 h-3.5 w-3.5" />
          )}
          {policy ? "Save Changes" : "Create Policy"}
        </Button>
      </div>
    </div>
  );
}
