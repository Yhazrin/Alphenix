"use client";

import { useState, useEffect } from "react";
import {
  Bot,
  Cloud,
  Monitor,
  Plus,
  ListTodo,
  Wrench,
  FileText,
  BookOpenText,
  MessageSquare,
  Timer,
  Trash2,
  Save,
  Key,
  Link2,
  Loader2,
  AlertCircle,
  MoreHorizontal,
  Settings,
  Eye,
} from "lucide-react";
import type {
  Agent,
  AgentTool,
  AgentTrigger,
  AgentTriggerType,
  RuntimeDevice,
} from "@/shared/types";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { statusConfig, getRuntimeDevice, generateId } from "./agent-configs";
import { TasksTab } from "./agent-task-list";
import { SkillsTab } from "./agent-skill-manager";
import { SettingsTab } from "./agent-settings-form";
import { PromptPreviewTab } from "./agent-prompt-preview";

// ---------------------------------------------------------------------------
// Instructions Tab
// ---------------------------------------------------------------------------

function InstructionsTab({
  agent,
  onSave,
}: {
  agent: Agent;
  onSave: (instructions: string) => Promise<void>;
}) {
  const [value, setValue] = useState(agent.instructions ?? "");
  const [saving, setSaving] = useState(false);
  const isDirty = value !== (agent.instructions ?? "");

  // Sync when switching between agents.
  useEffect(() => {
    setValue(agent.instructions ?? "");
  }, [agent.id, agent.instructions]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave(value);
    } catch {
      // toast handled by parent
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold">Agent Instructions</h3>
        <p className="text-xs text-muted-foreground mt-0.5">
          Define this agent&apos;s identity and working style. These instructions are
          injected into the agent&apos;s context for every task.
        </p>
      </div>

      <textarea
        value={value}
        onChange={(e) => setValue(e.target.value)}
        aria-label="Agent instructions"
        placeholder={`Define this agent's role, expertise, and working style.\n\nExample:\nYou are a frontend engineer specializing in React and TypeScript.\n\n## Working Style\n- Write small, focused PRs — one commit per logical change\n- Prefer composition over inheritance\n- Always add unit tests for new components\n\n## Constraints\n- Do not modify shared/ types without explicit approval\n- Follow the existing component patterns in features/`}
        className="w-full min-h-[300px] rounded-md border bg-transparent px-3 py-2 text-sm font-mono placeholder:text-muted-foreground/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring resize-y"
      />

      <div className="flex items-center justify-between">
        <span className="text-xs text-muted-foreground">
          {value.length > 0 ? `${value.length} characters` : "No instructions set"}
        </span>
        <Button
          size="xs"
          onClick={handleSave}
          disabled={!isDirty || saving}
        >
          {saving ? (
            <Loader2 className="h-3 w-3 animate-spin" aria-hidden="true" />
          ) : (
            <Save className="h-3 w-3" aria-hidden="true" />
          )}
          Save
        </Button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Add Tool Dialog
// ---------------------------------------------------------------------------

function AddToolDialog({
  onClose,
  onAdd,
}: {
  onClose: () => void;
  onAdd: (tool: AgentTool) => void;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [authType, setAuthType] = useState<"oauth" | "api_key" | "none">("api_key");

  const handleAdd = () => {
    if (!name.trim()) return;
    onAdd({
      id: generateId(),
      name: name.trim(),
      description: description.trim(),
      auth_type: authType,
      connected: false,
      config: {},
    });
    onClose();
  };

  return (
    <Dialog open onOpenChange={(v) => { if (!v) onClose(); }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="text-sm">Add Tool</DialogTitle>
          <DialogDescription className="text-xs">
            Connect an external tool for this agent to use.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <div>
            <Label htmlFor="tool-name" className="text-xs text-muted-foreground">Tool Name</Label>
            <Input
              id="tool-name"
              autoFocus
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Google Search, Slack, GitHub"
              className="mt-1"
              onKeyDown={(e) => e.key === "Enter" && handleAdd()}
            />
          </div>
          <div>
            <Label htmlFor="tool-description" className="text-xs text-muted-foreground">Description</Label>
            <Input
              id="tool-description"
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What does this tool do?"
              className="mt-1"
            />
          </div>
          <div>
            <Label className="text-xs text-muted-foreground">Authentication</Label>
            <div className="mt-1.5 flex gap-2">
              {(["api_key", "oauth", "none"] as const).map((type) => (
                <Button
                  key={type}
                  variant={authType === type ? "outline" : "ghost"}
                  size="xs"
                  onClick={() => setAuthType(type)}
                  className={`flex-1 ${
                    authType === type
                      ? "border-primary bg-primary/5 font-medium"
                      : ""
                  }`}
                >
                  {type === "api_key" ? "API Key" : type === "oauth" ? "OAuth" : "None"}
                </Button>
              ))}
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button
            onClick={handleAdd}
            disabled={!name.trim()}
          >
            Add
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// Tools Tab
// ---------------------------------------------------------------------------

function ToolsTab({
  agent,
  onSave,
}: {
  agent: Agent;
  onSave: (tools: AgentTool[]) => Promise<void>;
}) {
  const [tools, setTools] = useState<AgentTool[]>(agent.tools ?? []);
  const [showAdd, setShowAdd] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setTools(agent.tools ?? []);
  }, [agent.id, agent.tools]);

  const isDirty = JSON.stringify(tools) !== JSON.stringify(agent.tools ?? []);

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave(tools);
    } catch {
      // toast handled by parent
    } finally {
      setSaving(false);
    }
  };

  const toggleConnect = (toolId: string) => {
    setTools((prev) =>
      prev.map((t) => (t.id === toolId ? { ...t, connected: !t.connected } : t)),
    );
  };

  const removeTool = (toolId: string) => {
    setTools((prev) => prev.filter((t) => t.id !== toolId));
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold">Tools</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            External tools and APIs this agent can use during task execution.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {isDirty && (
            <Button
              onClick={handleSave}
              disabled={saving}
              size="xs"
            >
              <Save className="h-3 w-3" aria-hidden="true" />
              {saving ? "Saving..." : "Save"}
            </Button>
          )}
          <Button
            variant="outline"
            size="xs"
            onClick={() => setShowAdd(true)}
          >
            <Plus className="h-3 w-3" aria-hidden="true" />
            Add Tool
          </Button>
        </div>
      </div>

      {tools.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-12">
          <Wrench className="h-8 w-8 text-muted-foreground/40" aria-hidden="true" />
          <p className="mt-3 text-sm text-muted-foreground">No tools configured</p>
          <Button
            onClick={() => setShowAdd(true)}
            size="xs"
            className="mt-3"
          >
            <Plus className="h-3 w-3" aria-hidden="true" />
            Add Tool
          </Button>
        </div>
      ) : (
        <div className="space-y-2">
          {tools.map((tool) => (
            <div
              key={tool.id}
              className="flex items-center gap-3 rounded-lg border px-4 py-3"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                {tool.auth_type === "oauth" ? (
                  <Link2 className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
                ) : tool.auth_type === "api_key" ? (
                  <Key className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
                ) : (
                  <Wrench className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
                )}
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium">{tool.name}</div>
                {tool.description && (
                  <div className="text-xs text-muted-foreground truncate">
                    {tool.description}
                  </div>
                )}
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  size="xs"
                  onClick={() => toggleConnect(tool.id)}
                  className={
                    tool.connected
                      ? "bg-success/10 text-success"
                      : "bg-muted text-muted-foreground hover:bg-accent"
                  }
                >
                  {tool.connected ? "Connected" : "Connect"}
                </Button>
                <Button
                  variant="ghost"
                  size="icon-xs"
                  aria-label={`Remove ${tool.name}`}
                  onClick={() => removeTool(tool.id)}
                  className="text-muted-foreground hover:text-destructive"
                >
                  <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      {showAdd && (
        <AddToolDialog
          onClose={() => setShowAdd(false)}
          onAdd={(tool) => setTools((prev) => [...prev, tool])}
        />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Triggers Tab
// ---------------------------------------------------------------------------

function TriggersTab({
  agent,
  onSave,
}: {
  agent: Agent;
  onSave: (triggers: AgentTrigger[]) => Promise<void>;
}) {
  const [triggers, setTriggers] = useState<AgentTrigger[]>(agent.triggers ?? []);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setTriggers(agent.triggers ?? []);
  }, [agent.id, agent.triggers]);

  const isDirty = JSON.stringify(triggers) !== JSON.stringify(agent.triggers ?? []);

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave(triggers);
    } catch {
      // toast handled by parent
    } finally {
      setSaving(false);
    }
  };

  const toggleTrigger = (triggerId: string) => {
    setTriggers((prev) =>
      prev.map((t) => (t.id === triggerId ? { ...t, enabled: !t.enabled } : t)),
    );
  };

  const removeTrigger = (triggerId: string) => {
    setTriggers((prev) => prev.filter((t) => t.id !== triggerId));
  };

  const addTrigger = (type: AgentTriggerType) => {
    const newTrigger: AgentTrigger = {
      id: generateId(),
      type,
      enabled: true,
      config: type === "scheduled" ? { cron: "0 9 * * 1-5", timezone: "UTC" } : {},
    };
    setTriggers((prev) => [...prev, newTrigger]);
  };

  const updateTriggerConfig = (triggerId: string, config: Record<string, unknown>) => {
    setTriggers((prev) =>
      prev.map((t) => (t.id === triggerId ? { ...t, config } : t)),
    );
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold">Triggers</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            Configure when this agent should start working.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {isDirty && (
            <Button
              onClick={handleSave}
              disabled={saving}
              size="xs"
            >
              <Save className="h-3 w-3" aria-hidden="true" />
              {saving ? "Saving..." : "Save"}
            </Button>
          )}
        </div>
      </div>

      <div className="space-y-2">
        {triggers.map((trigger) => (
          <div
            key={trigger.id}
            className="rounded-lg border px-4 py-3"
          >
            <div className="flex items-center gap-3">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                {trigger.type === "on_assign" ? (
                  <Bot className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
                ) : trigger.type === "on_comment" ? (
                  <MessageSquare className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
                ) : (
                  <Timer className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
                )}
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium">
                  {trigger.type === "on_assign"
                    ? "On Issue Assign"
                    : trigger.type === "on_comment"
                      ? "On Comment"
                      : "Scheduled"}
                </div>
                <div className="text-xs text-muted-foreground">
                  {trigger.type === "on_assign"
                    ? "Runs when an issue is assigned to this agent"
                    : trigger.type === "on_comment"
                      ? "Runs when a member comments on the agent's issue"
                      : `Cron: ${(trigger.config as { cron?: string }).cron ?? "Not set"}`}
                </div>
              </div>
              <div className="flex items-center gap-2">
                <button
                  role="switch"
                  aria-checked={trigger.enabled}
                  aria-label={`${trigger.type === "on_assign" ? "On Issue Assign" : trigger.type === "on_comment" ? "On Comment" : "Scheduled"} trigger`}
                  onClick={() => toggleTrigger(trigger.id)}
                  className={`relative h-5 w-9 rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                    trigger.enabled ? "bg-primary" : "bg-muted"
                  }`}
                >
                  <span
                    className={`absolute top-0.5 h-4 w-4 rounded-full bg-background shadow-sm transition-transform ${
                      trigger.enabled ? "left-[18px]" : "left-0.5"
                    }`}
                  />
                </button>
                <Button
                  variant="ghost"
                  size="icon-xs"
                  aria-label="Remove trigger"
                  onClick={() => removeTrigger(trigger.id)}
                  className="text-muted-foreground hover:text-destructive"
                >
                  <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
                </Button>
              </div>
            </div>

            {trigger.type === "scheduled" && (
              <div className="mt-3 grid grid-cols-2 gap-3 pl-12">
                <div>
                  <Label htmlFor={`cron-${trigger.id}`} className="text-xs text-muted-foreground">
                    Cron Expression
                  </Label>
                  <Input
                    id={`cron-${trigger.id}`}
                    type="text"
                    value={(trigger.config as { cron?: string }).cron ?? ""}
                    onChange={(e) =>
                      updateTriggerConfig(trigger.id, {
                        ...trigger.config,
                        cron: e.target.value,
                      })
                    }
                    placeholder="0 9 * * 1-5"
                    className="mt-1 text-xs font-mono"
                  />
                </div>
                <div>
                  <Label htmlFor={`tz-${trigger.id}`} className="text-xs text-muted-foreground">
                    Timezone
                  </Label>
                  <Input
                    id={`tz-${trigger.id}`}
                    type="text"
                    value={(trigger.config as { timezone?: string }).timezone ?? ""}
                    onChange={(e) =>
                      updateTriggerConfig(trigger.id, {
                        ...trigger.config,
                        timezone: e.target.value,
                      })
                    }
                    placeholder="UTC"
                    className="mt-1 text-xs"
                  />
                </div>
              </div>
            )}
          </div>
        ))}
      </div>

      <div className="flex gap-2">
        <Button
          variant="outline"
          size="xs"
          onClick={() => addTrigger("on_assign")}
          className="border-dashed text-muted-foreground hover:text-foreground"
        >
          <Bot className="h-3 w-3" aria-hidden="true" />
          Add On Assign
        </Button>
        <Button
          variant="outline"
          size="xs"
          onClick={() => addTrigger("on_comment")}
          className="border-dashed text-muted-foreground hover:text-foreground"
        >
          <MessageSquare className="h-3 w-3" aria-hidden="true" />
          Add On Comment
        </Button>
        <Button
          variant="outline"
          size="xs"
          onClick={() => addTrigger("scheduled")}
          className="border-dashed text-muted-foreground hover:text-foreground"
        >
          <Timer className="h-3 w-3" aria-hidden="true" />
          Add Scheduled
        </Button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Agent Detail
// ---------------------------------------------------------------------------

type DetailTab = "instructions" | "skills" | "tools" | "triggers" | "tasks" | "prompt" | "settings";

const detailTabs: { id: DetailTab; label: string; icon: typeof FileText }[] = [
  { id: "instructions", label: "Instructions", icon: FileText },
  { id: "skills", label: "Skills", icon: BookOpenText },
  { id: "tools", label: "Tools", icon: Wrench },
  { id: "triggers", label: "Triggers", icon: Timer },
  { id: "tasks", label: "Tasks", icon: ListTodo },
  { id: "prompt", label: "Prompt", icon: Eye },
  { id: "settings", label: "Settings", icon: Settings },
];

export function AgentDetail({
  agent,
  runtimes,
  onUpdate,
  onArchive,
  onRestore,
}: {
  agent: Agent;
  runtimes: RuntimeDevice[];
  onUpdate: (id: string, data: Partial<Agent>) => Promise<void>;
  onArchive: (id: string) => Promise<void>;
  onRestore: (id: string) => Promise<void>;
}) {
  const st = statusConfig[agent.status];
  const runtimeDevice = getRuntimeDevice(agent, runtimes);
  const [confirmArchive, setConfirmArchive] = useState(false);
  const isArchived = !!agent.archived_at;

  return (
    <div className="flex h-full flex-col">
      {/* Archive Banner */}
      {isArchived && (
        <div className="flex items-center gap-2 bg-muted/50 px-4 py-2 text-xs text-muted-foreground border-b">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          <span className="flex-1">This agent is archived. It cannot be assigned or mentioned.</span>
          <Button variant="outline" size="sm" className="h-6 text-xs" onClick={() => onRestore(agent.id)}>
            Restore
          </Button>
        </div>
      )}

      {/* Header */}
      <div className="flex h-12 shrink-0 items-center gap-3 border-b px-4">
        <ActorAvatar actorType="agent" actorId={agent.id} size={28} className={`rounded-md ${isArchived ? "opacity-50" : ""}`} />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h2 className={`text-sm font-semibold truncate ${isArchived ? "text-muted-foreground" : ""}`}>{agent.name}</h2>
            {isArchived ? (
              <span className="rounded-md bg-muted px-1.5 py-0.5 text-xs font-medium text-muted-foreground">
                Archived
              </span>
            ) : (
              <span className={`flex items-center gap-1.5 text-xs ${st.color}`}>
                <span className={`h-1.5 w-1.5 rounded-full ${st.dot}`} />
                {st.label}
              </span>
            )}
            <span className="flex items-center gap-1 rounded-md bg-muted px-1.5 py-0.5 text-xs font-medium text-muted-foreground">
              {agent.runtime_mode === "cloud" ? (
                <Cloud className="h-3 w-3" aria-hidden="true" />
              ) : (
                <Monitor className="h-3 w-3" aria-hidden="true" />
              )}
              {runtimeDevice?.name ?? (agent.runtime_mode === "cloud" ? "Cloud" : "Local")}
            </span>
          </div>
        </div>
        {!isArchived && (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button variant="ghost" size="icon-sm" aria-label="More actions" />
              }
            >
              <MoreHorizontal className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                className="text-destructive"
                onClick={() => setConfirmArchive(true)}
              >
                <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
                Archive Agent
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>

      {/* Tabs */}
      <Tabs defaultValue="instructions" className="flex-1 min-h-0 flex flex-col">
        <TabsList variant="line" className="w-full justify-start rounded-none border-b px-6 gap-0">
          {detailTabs.map((tab) => (
            <TabsTrigger key={tab.id} value={tab.id} className="gap-1.5 px-3 py-2.5 text-xs">
              <tab.icon className="h-3.5 w-3.5" aria-hidden="true" />
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>

      {/* Tab Content */}
      <div className="flex-1 overflow-y-auto p-6">
        <TabsContent value="instructions">
          <InstructionsTab
            agent={agent}
            onSave={(instructions) => onUpdate(agent.id, { instructions })}
          />
        </TabsContent>
        <TabsContent value="skills">
          <SkillsTab agent={agent} />
        </TabsContent>
        <TabsContent value="tools">
          <ToolsTab
            agent={agent}
            onSave={(tools) => onUpdate(agent.id, { tools })}
          />
        </TabsContent>
        <TabsContent value="triggers">
          <TriggersTab
            agent={agent}
            onSave={(triggers) => onUpdate(agent.id, { triggers })}
          />
        </TabsContent>
        <TabsContent value="tasks">
          <TasksTab agent={agent} />
        </TabsContent>
        <TabsContent value="prompt">
          <PromptPreviewTab agent={agent} />
        </TabsContent>
        <TabsContent value="settings">
          <SettingsTab
            agent={agent}
            runtimes={runtimes}
            onSave={(updates) => onUpdate(agent.id, updates)}
          />
        </TabsContent>
      </div>
      </Tabs>

      {/* Archive Confirmation */}
      {confirmArchive && (
        <Dialog open onOpenChange={(v) => { if (!v) setConfirmArchive(false); }}>
          <DialogContent className="max-w-sm" showCloseButton={false}>
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-destructive/10">
                <AlertCircle className="h-5 w-5 text-destructive" aria-hidden="true" />
              </div>
              <DialogHeader className="flex-1 gap-1">
                <DialogTitle className="text-sm font-semibold">Archive agent?</DialogTitle>
                <DialogDescription className="text-xs">
                  &quot;{agent.name}&quot; will be archived. It won&apos;t be assignable or mentionable, but all history is preserved. You can restore it later.
                </DialogDescription>
              </DialogHeader>
            </div>
            <DialogFooter>
              <Button variant="ghost" onClick={() => setConfirmArchive(false)}>
                Cancel
              </Button>
              <Button
                variant="destructive"
                onClick={() => {
                  setConfirmArchive(false);
                  onArchive(agent.id);
                }}
              >
                Archive
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}
