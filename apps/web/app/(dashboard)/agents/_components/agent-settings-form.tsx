"use client";

import { useState, useRef } from "react";
import {
  Cloud,
  Monitor,
  Loader2,
  Camera,
  Save,
  Globe,
  Lock,
  Activity,
  CheckCircle2,
  XCircle,
  Clock,
} from "lucide-react";
import type {
  Agent,
  AgentVisibility,
  RuntimeDevice,
} from "@/shared/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { useFileUpload } from "@/shared/hooks/use-file-upload";
import { useActorName } from "@/features/workspace/hooks";

export function SettingsTab({
  agent,
  runtimes,
  onSave,
}: {
  agent: Agent;
  runtimes: RuntimeDevice[];
  onSave: (updates: Partial<Agent>) => Promise<void>;
}) {
  const [name, setName] = useState(agent.name);
  const [description, setDescription] = useState(agent.description ?? "");
  const [visibility, setVisibility] = useState<AgentVisibility>(agent.visibility);
  const [maxTasks, setMaxTasks] = useState(agent.max_concurrent_tasks);
  const [saving, setSaving] = useState(false);
  const { upload, uploading } = useFileUpload();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleAvatarUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    e.target.value = "";
    try {
      const result = await upload(file);
      if (!result) return;
      await onSave({ avatar_url: result.link });
      toast.success("Avatar updated");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to upload avatar");
    }
  };

  const dirty =
    name !== agent.name ||
    description !== (agent.description ?? "") ||
    visibility !== agent.visibility ||
    maxTasks !== agent.max_concurrent_tasks;

  const handleSave = async () => {
    if (!name.trim()) {
      toast.error("Name is required");
      return;
    }
    setSaving(true);
    try {
      await onSave({ name: name.trim(), description, visibility, max_concurrent_tasks: maxTasks });
      toast.success("Settings saved");
    } catch {
      toast.error("Failed to save settings");
    } finally {
      setSaving(false);
    }
  };

  const runtimeDevice = runtimes.find((r) => r.id === agent.runtime_id);
  const { getMemberName } = useActorName();

  return (
    <div className="max-w-lg space-y-6">
      <div>
        <span className="text-xs text-muted-foreground">Avatar</span>
        <div className="mt-1.5 flex items-center gap-4">
          <button
            type="button"
            aria-label="Upload agent avatar"
            className="group relative h-16 w-16 shrink-0 rounded-full bg-muted overflow-hidden focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
          >
            <ActorAvatar actorType="agent" actorId={agent.id} size={64} className="rounded-none" />
            <div className="absolute inset-0 flex items-center justify-center bg-background/60 opacity-0 transition-opacity group-hover:opacity-100 backdrop-blur-sm">
              {uploading ? (
                <Loader2 className="h-5 w-5 animate-spin text-primary-foreground" aria-hidden="true" />
              ) : (
                <Camera className="h-5 w-5 text-primary-foreground" aria-hidden="true" />
              )}
            </div>
          </button>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            className="hidden"
            onChange={handleAvatarUpload}
          />
          <div className="text-xs text-muted-foreground">
            Click to upload avatar
          </div>
        </div>
      </div>

      <div>
        <Label htmlFor="agent-name" className="text-xs text-muted-foreground">Name</Label>
        <Input
          id="agent-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="mt-1"
        />
      </div>

      <div>
        <Label htmlFor="agent-description" className="text-xs text-muted-foreground">Description</Label>
        <Input
          id="agent-description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="What does this agent do?"
          className="mt-1"
        />
      </div>

      <div>
        <span className="text-xs text-muted-foreground">Visibility</span>
        <div className="mt-1.5 flex gap-2">
          <button
            type="button"
            aria-pressed={visibility === "workspace"}
            onClick={() => setVisibility("workspace")}
            className={`flex flex-1 items-center gap-2 rounded-lg border px-3 py-2.5 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
              visibility === "workspace"
                ? "border-primary bg-primary/5"
                : "border-border hover:bg-muted"
            }`}
          >
            <Globe className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <div className="text-left">
              <div className="font-medium">Workspace</div>
              <div className="text-xs text-muted-foreground">All members can assign</div>
            </div>
          </button>
          <button
            type="button"
            aria-pressed={visibility === "private"}
            onClick={() => setVisibility("private")}
            className={`flex flex-1 items-center gap-2 rounded-lg border px-3 py-2.5 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
              visibility === "private"
                ? "border-primary bg-primary/5"
                : "border-border hover:bg-muted"
            }`}
          >
            <Lock className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <div className="text-left">
              <div className="font-medium">Private</div>
              <div className="text-xs text-muted-foreground">Only you can assign</div>
            </div>
          </button>
        </div>
      </div>

      {agent.owner_id && (
        <div>
          <span className="text-xs text-muted-foreground">Owner</span>
          <div className="mt-1 flex items-center gap-2 rounded-lg border px-3 py-2.5 text-sm">
            <ActorAvatar actorType="member" actorId={agent.owner_id} size={20} />
            <span>{getMemberName(agent.owner_id)}</span>
          </div>
        </div>
      )}

      <div>
        <Label htmlFor="agent-max-tasks" className="text-xs text-muted-foreground">Max Concurrent Tasks</Label>
        <Input
          id="agent-max-tasks"
          type="number"
          min={1}
          max={50}
          value={maxTasks}
          onChange={(e) => { const v = Number(e.target.value); if (!isNaN(v) && v > 0) setMaxTasks(v); }}
          className="mt-1 w-24"
        />
      </div>

      <div>
        <span className="text-xs text-muted-foreground">Runtime</span>
        <div className="mt-1 flex items-center gap-2 rounded-lg border px-3 py-2.5 text-sm text-muted-foreground">
          {agent.runtime_mode === "cloud" ? (
            <Cloud className="h-4 w-4" aria-hidden="true" />
          ) : (
            <Monitor className="h-4 w-4" aria-hidden="true" />
          )}
          {runtimeDevice?.name ?? (agent.runtime_mode === "cloud" ? "Cloud" : "Local")}
        </div>
      </div>

      {runtimeDevice && (
        <div>
          <span className="text-xs text-muted-foreground flex items-center gap-1">
            <Activity className="h-3 w-3" aria-hidden="true" />
            Runtime Health (24h)
          </span>
          <div className="mt-1 grid grid-cols-3 gap-2">
            <div className="flex items-center gap-1.5 rounded-lg border px-3 py-2 text-xs">
              <CheckCircle2 className="h-3.5 w-3.5 text-success" aria-hidden="true" />
              <span className="text-muted-foreground">Succeeded</span>
              <span className="ml-auto font-medium">{runtimeDevice.success_count_24h}</span>
            </div>
            <div className="flex items-center gap-1.5 rounded-lg border px-3 py-2 text-xs">
              <XCircle className="h-3.5 w-3.5 text-destructive" aria-hidden="true" />
              <span className="text-muted-foreground">Failed</span>
              <span className="ml-auto font-medium">{runtimeDevice.failure_count_24h}</span>
            </div>
            <div className="flex items-center gap-1.5 rounded-lg border px-3 py-2 text-xs">
              <Clock className="h-3.5 w-3.5 text-muted-foreground" aria-hidden="true" />
              <span className="text-muted-foreground">Avg</span>
              <span className="ml-auto font-medium">
                {runtimeDevice.avg_task_duration_ms > 0
                  ? runtimeDevice.avg_task_duration_ms < 60_000
                    ? `${(runtimeDevice.avg_task_duration_ms / 1000).toFixed(1)}s`
                    : `${(runtimeDevice.avg_task_duration_ms / 60_000).toFixed(1)}m`
                  : "—"}
              </span>
            </div>
          </div>
        </div>
      )}

      <Button onClick={handleSave} disabled={!dirty || saving} size="sm">
        {saving ? <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" aria-hidden="true" /> : <Save className="h-3.5 w-3.5 mr-1.5" aria-hidden="true" />}
        Save Changes
      </Button>
    </div>
  );
}
