"use client";

import { useState, useEffect, useMemo } from "react";
import { useDefaultLayout } from "react-resizable-panels";
import {
  Users,
  Plus,
  Crown,
  Trash2,
  ChevronRight,
  Search,
  Settings,
  Archive,
  ArchiveRestore,
  UserPlus,
  UserMinus,
  Sparkles,
  AlertCircle,
} from "lucide-react";
import type {
  Team,
  Agent,
  CreateTeamRequest,
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
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@/components/ui/resizable";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/shared/api";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { PRESET_TEAMS, type PresetTeam } from "@/shared/data/preset-teams";
import { PRESET_AGENTS } from "@/shared/data/preset-agents";
import { timeAgo } from "@/shared/utils";
import { EmptyState } from "@/components/common/empty-state";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function generateId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

// ---------------------------------------------------------------------------
// Preset Team Card
// ---------------------------------------------------------------------------

function PresetTeamCard({
  preset,
  onSelect,
}: {
  preset: PresetTeam;
  onSelect: () => void;
}) {
  return (
    <button
      onClick={onSelect}
      className="flex w-full items-start gap-3 rounded-lg border border-border p-3 text-left transition-colors hover:bg-accent/50 hover:border-primary/30"
    >
      <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
        <Users className="h-5 w-5" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium">{preset.name}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{preset.description}</div>
        <div className="text-xs text-muted-foreground mt-1">
          {preset.presetAgentSlugs.length} agents
        </div>
      </div>
      <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground mt-2" />
    </button>
  );
}

// ---------------------------------------------------------------------------
// Create Team Dialog
// ---------------------------------------------------------------------------

function CreateTeamDialog({
  agents,
  onClose,
  onCreate,
}: {
  agents: Agent[];
  onClose: () => void;
  onCreate: (data: CreateTeamRequest) => Promise<Team>;
}) {
  const [mode, setMode] = useState<"preset" | "custom">("preset");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [selectedAgentIds, setSelectedAgentIds] = useState<string[]>([]);
  const [leadAgentId, setLeadAgentId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedPreset, setSelectedPreset] = useState<PresetTeam | null>(null);

  const availableAgents = useMemo(
    () => agents.filter((a) => !a.archived_at),
    [agents],
  );

  const filteredPresets = useMemo(() => {
    if (!searchQuery.trim()) return PRESET_TEAMS;
    const q = searchQuery.toLowerCase();
    return PRESET_TEAMS.filter(
      (t) => t.name.toLowerCase().includes(q) || t.description.toLowerCase().includes(q),
    );
  }, [searchQuery]);

  const handleSelectPreset = (preset: PresetTeam) => {
    setSelectedPreset(preset);
    setName(preset.name);
    setDescription(preset.description);

    // Try to match preset agent slugs to actual workspace agents by name
    const matchedIds: string[] = [];
    for (const slug of preset.presetAgentSlugs) {
      const presetAgent = PRESET_AGENTS.find((a) => a.id === slug);
      if (presetAgent) {
        // Try to find a workspace agent with matching name
        const match = availableAgents.find(
          (a) => a.name.toLowerCase() === presetAgent.name.toLowerCase(),
        );
        if (match) {
          matchedIds.push(match.id);
        }
      }
    }
    setSelectedAgentIds(matchedIds);

    // Set lead if matched
    if (preset.leadSlug) {
      const leadPreset = PRESET_AGENTS.find((a) => a.id === preset.leadSlug);
      if (leadPreset) {
        const leadAgent = availableAgents.find(
          (a) => a.name.toLowerCase() === leadPreset.name.toLowerCase(),
        );
        if (leadAgent) {
          setLeadAgentId(leadAgent.id);
        }
      }
    }

    setMode("custom");
  };

  const toggleAgent = (agentId: string) => {
    setSelectedAgentIds((prev) =>
      prev.includes(agentId) ? prev.filter((id) => id !== agentId) : [...prev, agentId],
    );
  };

  const handleSubmit = async () => {
    if (!name.trim()) return;
    setCreating(true);
    try {
      const team = await onCreate({
        name: name.trim(),
        description: description.trim() || undefined,
        lead_agent_id: leadAgentId ?? undefined,
        member_agent_ids: selectedAgentIds,
      });
      toast.success(`Team "${team.name}" created`);
      onClose();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create team");
      setCreating(false);
    }
  };

  return (
    <Dialog open onOpenChange={(v) => { if (!v) onClose(); }}>
      <DialogContent className={mode === "preset" ? "sm:max-w-lg" : "sm:max-w-md"}>
        <DialogHeader>
          <DialogTitle>
            {mode === "preset"
              ? "Create Team"
              : selectedPreset
                ? `Create: ${selectedPreset.name}`
                : "Create Team"}
          </DialogTitle>
          <DialogDescription>
            {mode === "preset"
              ? "Choose a preset template or create a custom team."
              : "Configure your team and select members."}
          </DialogDescription>
        </DialogHeader>

        {mode === "preset" ? (
          <div className="space-y-3">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                autoFocus
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search team templates..."
                className="pl-9"
              />
            </div>

            <div className="max-h-[400px] overflow-y-auto space-y-1.5 pr-1">
              {filteredPresets.map((preset) => (
                <PresetTeamCard
                  key={preset.id}
                  preset={preset}
                  onSelect={() => handleSelectPreset(preset)}
                />
              ))}
            </div>

            <DialogFooter>
              <Button variant="ghost" onClick={onClose}>
                Cancel
              </Button>
              <Button variant="outline" onClick={() => setMode("custom")}>
                Create custom team
              </Button>
            </DialogFooter>
          </div>
        ) : (
          <div className="space-y-4">
            {selectedPreset && (
              <button
                onClick={() => { setMode("preset"); setSelectedPreset(null); }}
                className="text-xs text-muted-foreground hover:text-foreground transition-colors"
              >
                ← Back to templates
              </button>
            )}

            <div>
              <Label className="text-xs text-muted-foreground">Name</Label>
              <Input
                autoFocus={!selectedPreset}
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Frontend Team"
                className="mt-1"
              />
            </div>

            <div>
              <Label className="text-xs text-muted-foreground">Description</Label>
              <Input
                type="text"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="What does this team do?"
                className="mt-1"
              />
            </div>

            {/* Agent picker */}
            <div>
              <Label className="text-xs text-muted-foreground">
                Members ({selectedAgentIds.length} selected)
              </Label>
              <div className="mt-1.5 max-h-[200px] overflow-y-auto rounded-lg border border-border divide-y">
                {availableAgents.length === 0 ? (
                  <div className="py-4 text-center text-sm text-muted-foreground">
                    No agents available. Create an agent first.
                  </div>
                ) : (
                  availableAgents.map((agent) => {
                    const isSelected = selectedAgentIds.includes(agent.id);
                    const isLead = leadAgentId === agent.id;
                    return (
                      <div
                        key={agent.id}
                        className="flex items-center gap-2 px-3 py-2"
                      >
                        <button
                          onClick={() => toggleAgent(agent.id)}
                          className={`flex h-4 w-4 shrink-0 items-center justify-center rounded border transition-colors ${
                            isSelected
                              ? "border-primary bg-primary text-primary-foreground"
                              : "border-border"
                          }`}
                        >
                          {isSelected && <span className="text-[10px]">✓</span>}
                        </button>
                        <ActorAvatar actorType="agent" actorId={agent.id} size={24} />
                        <span className="flex-1 truncate text-sm">{agent.name}</span>
                        {isSelected && (
                          <button
                            onClick={() => setLeadAgentId(isLead ? null : agent.id)}
                            className={`rounded px-1.5 py-0.5 text-[10px] font-medium transition-colors ${
                              isLead
                                ? "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400"
                                : "text-muted-foreground hover:bg-muted"
                            }`}
                          >
                            <Crown className="h-3 w-3 inline mr-0.5" />
                            {isLead ? "Lead" : "Set lead"}
                          </button>
                        )}
                      </div>
                    );
                  })
                )}
              </div>
            </div>
          </div>
        )}

        {mode === "custom" && (
          <DialogFooter>
            <Button variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={creating || !name.trim()}
            >
              {creating ? "Creating..." : "Create Team"}
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// Team List Item
// ---------------------------------------------------------------------------

function TeamListItem({
  team,
  isSelected,
  onClick,
}: {
  team: Team;
  isSelected: boolean;
  onClick: () => void;
}) {
  const isArchived = !!team.archived_at;

  return (
    <button
      onClick={onClick}
      className={`flex w-full items-center gap-3 px-4 py-3 text-left transition-colors ${
        isSelected ? "bg-accent" : "hover:bg-accent/50"
      }`}
    >
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
        <Users className="h-4 w-4" />
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className={`truncate text-sm font-medium ${isArchived ? "text-muted-foreground" : ""}`}>
            {team.name}
          </span>
        </div>
        <div className="flex items-center gap-1.5 mt-0.5">
          {isArchived ? (
            <span className="text-xs text-muted-foreground">Archived</span>
          ) : (
            <span className="text-xs text-muted-foreground">
              {team.members?.length ?? 0} members
            </span>
          )}
        </div>
      </div>
    </button>
  );
}

// ---------------------------------------------------------------------------
// Team Detail
// ---------------------------------------------------------------------------

function TeamDetail({
  team,
  agents,
  onUpdate,
  onArchive,
  onRestore,
  onAddMember,
  onRemoveMember,
  onSetLead,
}: {
  team: Team;
  agents: Agent[];
  onUpdate: (id: string, data: Record<string, unknown>) => Promise<void>;
  onArchive: (id: string) => Promise<void>;
  onRestore: (id: string) => Promise<void>;
  onAddMember: (teamId: string, agentId: string) => Promise<void>;
  onRemoveMember: (teamId: string, agentId: string) => Promise<void>;
  onSetLead: (teamId: string, agentId: string) => Promise<void>;
}) {
  const [editingName, setEditingName] = useState(false);
  const [name, setName] = useState(team.name);
  const [description, setDescription] = useState(team.description ?? "");
  const [showAddMember, setShowAddMember] = useState(false);
  const isArchived = !!team.archived_at;

  useEffect(() => {
    setName(team.name);
    setDescription(team.description ?? "");
  }, [team.id, team.name, team.description]);

  const memberAgents = useMemo(() => {
    if (!team.members) return [];
    return team.members
      .map((m) => agents.find((a) => a.id === m.agent_id))
      .filter((a): a is Agent => !!a);
  }, [team.members, agents]);

  const nonMemberAgents = useMemo(() => {
    const memberIds = new Set((team.members ?? []).map((m) => m.agent_id));
    return agents.filter((a) => !a.archived_at && !memberIds.has(a.id));
  }, [agents, team.members]);

  const leadMember = team.members?.find((m) => m.role === "lead");

  const handleSaveName = async () => {
    if (name.trim() && name !== team.name) {
      await onUpdate(team.id, { name: name.trim() });
    }
    setEditingName(false);
  };

  const handleSaveDescription = async () => {
    if (description !== (team.description ?? "")) {
      await onUpdate(team.id, { description: description || null });
    }
  };

  return (
    <div className="flex h-full flex-col overflow-hidden">
      {/* Header */}
      <div className="flex items-center gap-3 border-b px-6 py-4">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Users className="h-5 w-5" />
        </div>
        <div className="min-w-0 flex-1">
          {editingName ? (
            <Input
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              onBlur={handleSaveName}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleSaveName();
                if (e.key === "Escape") {
                  setName(team.name);
                  setEditingName(false);
                }
              }}
              className="h-7 text-base font-semibold"
            />
          ) : (
            <h2
              className="text-base font-semibold cursor-pointer hover:text-muted-foreground transition-colors"
              onClick={() => !isArchived && setEditingName(true)}
            >
              {team.name}
            </h2>
          )}
          <p className="text-xs text-muted-foreground mt-0.5">
            {team.members?.length ?? 0} members
            {team.updated_at && (
              <span className="ml-1.5">· Updated {timeAgo(team.updated_at)}</span>
            )}
          </p>
        </div>
        <div className="flex items-center gap-1">
          {isArchived ? (
            <Button variant="ghost" size="icon-xs" onClick={() => onRestore(team.id)} title="Restore">
              <ArchiveRestore className="h-4 w-4 text-muted-foreground" />
            </Button>
          ) : (
            <Button variant="ghost" size="icon-xs" onClick={() => onArchive(team.id)} title="Archive">
              <Archive className="h-4 w-4 text-muted-foreground" />
            </Button>
          )}
        </div>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto px-6 py-4 space-y-6">
        {/* Description */}
        <div>
          <Label className="text-xs text-muted-foreground">Description</Label>
          <Input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            onBlur={handleSaveDescription}
            placeholder="Add a description..."
            disabled={isArchived}
            className="mt-1"
          />
        </div>

        {/* Members */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <Label className="text-xs text-muted-foreground">Members</Label>
            {!isArchived && nonMemberAgents.length > 0 && (
              <Button
                variant="ghost"
                size="xs"
                onClick={() => setShowAddMember(!showAddMember)}
              >
                <UserPlus className="h-3 w-3 mr-1" />
                Add
              </Button>
            )}
          </div>

          {/* Add member dropdown */}
          {showAddMember && nonMemberAgents.length > 0 && (
            <div className="mb-2 rounded-lg border border-border divide-y max-h-[150px] overflow-y-auto">
              {nonMemberAgents.map((agent) => (
                <button
                  key={agent.id}
                  onClick={async () => {
                    await onAddMember(team.id, agent.id);
                    setShowAddMember(false);
                  }}
                  className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-accent/50 transition-colors"
                >
                  <ActorAvatar actorType="agent" actorId={agent.id} size={20} />
                  <span className="truncate">{agent.name}</span>
                </button>
              ))}
            </div>
          )}

          {/* Member list */}
          <div className="rounded-lg border border-border divide-y">
            {memberAgents.length === 0 ? (
              <div className="py-4 text-center text-sm text-muted-foreground">
                No members yet
              </div>
            ) : (
              memberAgents.map((agent) => {
                const member = team.members?.find((m) => m.agent_id === agent.id);
                const isLead = member?.role === "lead";
                return (
                  <div key={agent.id} className="flex items-center gap-2 px-3 py-2">
                    <ActorAvatar actorType="agent" actorId={agent.id} size={24} />
                    <div className="flex-1 min-w-0">
                      <span className="block truncate text-sm">{agent.name}</span>
                      {member?.joined_at && (
                        <span className="block text-[10px] text-muted-foreground">
                          Joined {timeAgo(member.joined_at)}
                        </span>
                      )}
                    </div>
                    {isLead && (
                      <span className="rounded bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 px-1.5 py-0.5 text-[10px] font-medium">
                        <Crown className="h-3 w-3 inline mr-0.5" />
                        Lead
                      </span>
                    )}
                    {!isArchived && (
                      <div className="flex items-center gap-0.5">
                        {!isLead && (
                          <Button
                            variant="ghost"
                            size="icon-xs"
                            onClick={() => onSetLead(team.id, agent.id)}
                            title="Set as lead"
                          >
                            <Crown className="h-3 w-3 text-muted-foreground" />
                          </Button>
                        )}
                        <Button
                          variant="ghost"
                          size="icon-xs"
                          onClick={() => onRemoveMember(team.id, agent.id)}
                          title="Remove member"
                        >
                          <UserMinus className="h-3 w-3 text-muted-foreground" />
                        </Button>
                      </div>
                    )}
                  </div>
                );
              })
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function TeamsPage() {
  const isLoading = useAuthStore((s) => s.isLoading);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const agents = useWorkspaceStore((s) => s.agents);
  const refreshAgents = useWorkspaceStore((s) => s.refreshAgents);
  const [teams, setTeams] = useState<Team[]>([]);
  const [loadingTeams, setLoadingTeams] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string>("");
  const [showArchived, setShowArchived] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id: "multica_teams_layout",
  });

  const fetchTeams = async () => {
    try {
      const data = await api.listTeams();
      setTeams(data);
      setError(null);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to load teams");
    } finally {
      setLoadingTeams(false);
    }
  };

  useEffect(() => {
    if (workspace) {
      fetchTeams();
      refreshAgents();
    }
  }, [workspace]);

  const filteredTeams = useMemo(
    () =>
      showArchived
        ? teams.filter((t) => !!t.archived_at)
        : teams.filter((t) => !t.archived_at),
    [teams, showArchived],
  );

  const archivedCount = useMemo(() => teams.filter((t) => !!t.archived_at).length, [teams]);

  useEffect(() => {
    if (filteredTeams.length > 0 && !filteredTeams.some((t) => t.id === selectedId)) {
      setSelectedId(filteredTeams[0]!.id);
    }
  }, [filteredTeams, selectedId]);

  const handleCreate = async (data: CreateTeamRequest): Promise<Team> => {
    const team = await api.createTeam(data);
    await fetchTeams();
    setSelectedId(team.id);
    return team;
  };

  const handleUpdate = async (id: string, data: Record<string, unknown>) => {
    try {
      await api.updateTeam(id, data as any);
      await fetchTeams();
      toast.success("Team updated");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update team");
      throw e;
    }
  };

  const handleArchive = async (id: string) => {
    try {
      await api.archiveTeam(id);
      await fetchTeams();
      toast.success("Team archived");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to archive team");
    }
  };

  const handleRestore = async (id: string) => {
    try {
      await api.restoreTeam(id);
      await fetchTeams();
      toast.success("Team restored");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to restore team");
    }
  };

  const handleAddMember = async (teamId: string, agentId: string) => {
    try {
      await api.addTeamMember(teamId, agentId);
      await fetchTeams();
      toast.success("Member added");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to add member");
    }
  };

  const handleRemoveMember = async (teamId: string, agentId: string) => {
    try {
      await api.removeTeamMember(teamId, agentId);
      await fetchTeams();
      toast.success("Member removed");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to remove member");
    }
  };

  const handleSetLead = async (teamId: string, agentId: string) => {
    try {
      await api.setTeamLead(teamId, agentId);
      await fetchTeams();
      toast.success("Lead updated");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to set lead");
    }
  };

  const selected = teams.find((t) => t.id === selectedId) ?? null;

  if (isLoading || loadingTeams) {
    return (
      <div className="flex flex-1 min-h-0">
        <div className="w-72 border-r">
          <div className="px-4 py-3 border-b">
            <Skeleton className="h-5 w-20" />
          </div>
          {[1, 2, 3].map((i) => (
            <div key={i} className="flex items-center gap-3 px-4 py-3">
              <Skeleton className="h-8 w-8 rounded-lg" />
              <div className="flex-1">
                <Skeleton className="h-4 w-24 mb-1" />
                <Skeleton className="h-3 w-16" />
              </div>
            </div>
          ))}
        </div>
        <div className="flex-1 flex items-center justify-center">
          <Skeleton className="h-10 w-10 rounded-lg" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle className="h-8 w-8 text-destructive/60" aria-hidden="true" />
          <p className="text-sm font-medium text-destructive">Failed to load teams</p>
          <p className="text-xs text-destructive/70">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <ResizablePanelGroup
      orientation="horizontal"
      className="flex-1 min-h-0"
      defaultLayout={defaultLayout}
      onLayoutChanged={onLayoutChanged}
    >
      <ResizablePanel id="list" defaultSize={280} minSize={240} maxSize={400} groupResizeBehavior="preserve-pixel-size">
        {/* Left column — team list */}
        <div className="flex h-full flex-col">
          <div className="flex items-center justify-between border-b px-4 py-2">
            <h2 className="text-sm font-semibold">Teams</h2>
            <div className="flex items-center gap-0.5">
              {archivedCount > 0 && (
                <Button
                  variant={showArchived ? "secondary" : "ghost"}
                  size="icon-xs"
                  onClick={() => setShowArchived(!showArchived)}
                  title={showArchived ? "Show active teams" : "Show archived teams"}
                >
                  <Archive className="h-4 w-4 text-muted-foreground" />
                </Button>
              )}
              <Button
                variant="ghost"
                size="icon-xs"
                onClick={() => setShowCreate(true)}
              >
                <Plus className="h-4 w-4 text-muted-foreground" />
              </Button>
            </div>
          </div>

          {filteredTeams.length === 0 ? (
            <EmptyState
              icon={Users}
              title={showArchived ? "No archived teams" : "Build your first team"}
              description={
                showArchived
                  ? undefined
                  : "Teams group agents and members together for coordinated task execution."
              }
              actions={
                showArchived
                  ? undefined
                  : [{ label: "Create Team", onClick: () => setShowCreate(true), icon: Plus }]
              }
            />
          ) : (
            <div className="divide-y">
              {filteredTeams.map((team) => (
                <TeamListItem
                  key={team.id}
                  team={team}
                  isSelected={team.id === selectedId}
                  onClick={() => setSelectedId(team.id)}
                />
              ))}
            </div>
          )}
        </div>
      </ResizablePanel>

      <ResizableHandle />

      <ResizablePanel id="detail" minSize={400}>
        {selected ? (
          <TeamDetail
            key={selected.id}
            team={selected}
            agents={agents}
            onUpdate={handleUpdate}
            onArchive={handleArchive}
            onRestore={handleRestore}
            onAddMember={handleAddMember}
            onRemoveMember={handleRemoveMember}
            onSetLead={handleSetLead}
          />
        ) : (
          <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
            <Users className="h-10 w-10 text-muted-foreground/30" />
            <p className="mt-3 text-sm">Select a team to view details</p>
            <Button onClick={() => setShowCreate(true)} size="xs" className="mt-3">
              <Plus className="h-3 w-3" />
              Create Team
            </Button>
          </div>
        )}
      </ResizablePanel>

      {showCreate && (
        <CreateTeamDialog
          agents={agents}
          onClose={() => setShowCreate(false)}
          onCreate={handleCreate}
        />
      )}
    </ResizablePanelGroup>
  );
}
