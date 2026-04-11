"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Hash, Plus, Trash2, User, Bot, Users } from "lucide-react";
import { toast } from "sonner";
import { Button, buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { api } from "@/shared/api";
import type { Channel, ChannelParticipant, Team } from "@/shared/types";
import { useWorkspaceStore, useActorName } from "@/features/workspace";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { useChannelStore } from "../store";

type ParticipantKind = "member" | "agent" | "team";

function participantKey(p: ChannelParticipant): string {
  return `${p.participant_type}:${p.participant_id}`;
}

export function ChannelSettingsPage({ channelId }: { channelId: string }) {
  const workspace = useWorkspaceStore((s) => s.workspace);
  const members = useWorkspaceStore((s) => s.members);
  const agents = useWorkspaceStore((s) => s.agents);
  const { getMemberName, getAgentName } = useActorName();

  const [channel, setChannel] = useState<Channel | null>(null);
  const [participants, setParticipants] = useState<ChannelParticipant[]>([]);
  const [teams, setTeams] = useState<Team[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [addOpen, setAddOpen] = useState(false);
  const [addKind, setAddKind] = useState<ParticipantKind>("member");
  const [addBusy, setAddBusy] = useState(false);

  const load = useCallback(async () => {
    if (!channelId) return;
    setLoading(true);
    setError(null);
    try {
      const [ch, parts, teamList] = await Promise.all([
        api.getChannel(channelId),
        api.listChannelParticipants(channelId),
        api.listTeams(),
      ]);
      setChannel(ch);
      setParticipants(parts.participants);
      setTeams(teamList);
      void useChannelStore.getState().fetchChannels();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load channel");
      setChannel(null);
      setParticipants([]);
    } finally {
      setLoading(false);
    }
  }, [channelId]);

  useEffect(() => {
    void load();
  }, [load]);

  const participantSet = useMemo(
    () => new Set(participants.map(participantKey)),
    [participants],
  );

  const memberCandidates = useMemo(
    () =>
      members.filter(
        (m) => !participantSet.has(`member:${m.user_id}`),
      ),
    [members, participantSet],
  );

  const agentCandidates = useMemo(
    () =>
      agents.filter(
        (a) => !a.archived_at && !participantSet.has(`agent:${a.id}`),
      ),
    [agents, participantSet],
  );

  const teamCandidates = useMemo(
    () =>
      teams.filter(
        (t) => !t.archived_at && !participantSet.has(`team:${t.id}`),
      ),
    [teams, participantSet],
  );

  const handleRemove = async (p: ChannelParticipant) => {
    try {
      await api.removeChannelParticipant(channelId, p.participant_type, p.participant_id);
      setParticipants((prev) => prev.filter((x) => participantKey(x) !== participantKey(p)));
      toast.success("Removed from channel");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to remove");
    }
  };

  const handleAdd = async (participantType: ParticipantKind, participantId: string) => {
    setAddBusy(true);
    try {
      await api.addChannelParticipant(channelId, { participant_type: participantType, participant_id: participantId });
      const res = await api.listChannelParticipants(channelId);
      setParticipants(res.participants);
      setAddOpen(false);
      toast.success("Added to channel");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to add");
    } finally {
      setAddBusy(false);
    }
  };

  if (!workspace) {
    return null;
  }

  if (loading && !channel) {
    return (
      <div className="flex flex-1 flex-col gap-4 p-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-72" />
        <Skeleton className="h-64 w-full max-w-lg" />
      </div>
    );
  }

  if (error || !channel) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-3 p-6 text-center">
        <p className="text-sm font-medium text-destructive">{error ?? "Channel not found"}</p>
        <Link href="/issues" className={buttonVariants({ variant: "outline", size: "sm" })}>
          Back to issues
        </Link>
      </div>
    );
  }

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      <div className="flex shrink-0 items-center gap-3 border-b px-4 py-3">
        <Link
          href="/issues"
          aria-label="Back to issues"
          className={buttonVariants({
            variant: "ghost",
            size: "icon-xs",
            className: "text-muted-foreground",
          })}
        >
          <ArrowLeft className="size-4" aria-hidden="true" />
        </Link>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <Hash className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <h1 className="truncate text-base font-semibold">{channel.name}</h1>
            {channel.is_default && (
              <Badge variant="secondary" className="text-[10px] font-normal">
                Default
              </Badge>
            )}
          </div>
          <p className="truncate text-xs text-muted-foreground">/{channel.slug}</p>
        </div>
        <Button size="sm" className="gap-1.5" onClick={() => setAddOpen(true)}>
          <Plus className="size-3.5" aria-hidden="true" />
          Add access
        </Button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-4 md:p-6">
        <p className="mb-4 max-w-xl text-sm text-muted-foreground">
          Members, agents, and teams listed here can see work in this channel. Issue lists respect the channel filter in the sidebar.
        </p>

        <ul className="max-w-xl divide-y rounded-xl border border-border/70 bg-card">
          {participants.length === 0 ? (
            <li className="px-4 py-8 text-center text-sm text-muted-foreground">No participants yet.</li>
          ) : (
            participants.map((p) => {
              const title =
                p.participant_type === "team"
                  ? teams.find((t) => t.id === p.participant_id)?.name ?? "Team"
                  : p.participant_type === "agent"
                    ? getAgentName(p.participant_id)
                    : getMemberName(p.participant_id);
              return (
                <li
                  key={participantKey(p)}
                  className="flex items-center gap-3 px-4 py-3"
                >
                  <ActorAvatar
                    key={`${participantKey(p)}-${title}`}
                    actorType={p.participant_type}
                    actorId={p.participant_id}
                    size={36}
                    className="rounded-lg"
                    getName={(type, id) => {
                      if (type === "team") return teams.find((t) => t.id === id)?.name ?? "Team";
                      if (type === "agent") return getAgentName(id);
                      return getMemberName(id);
                    }}
                    getInitials={(type, id) => {
                      const n =
                        type === "team"
                          ? teams.find((t) => t.id === id)?.name ?? "TM"
                          : type === "agent"
                            ? getAgentName(id)
                            : getMemberName(id);
                      return n
                        .split(" ")
                        .filter((w) => w.length > 0)
                        .map((w) => w[0])
                        .join("")
                        .toUpperCase()
                        .slice(0, 2);
                    }}
                  />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">{title}</p>
                    <p className="text-xs capitalize text-muted-foreground">{p.participant_type}</p>
                  </div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    className="text-muted-foreground hover:text-destructive"
                    aria-label={`Remove ${title}`}
                    onClick={() => void handleRemove(p)}
                  >
                    <Trash2 className="size-3.5" aria-hidden="true" />
                  </Button>
                </li>
              );
            })
          )}
        </ul>
      </div>

      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Add to channel</DialogTitle>
            <DialogDescription>
              Grant access for a workspace member, agent, or team.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label>Type</Label>
              <Select
                value={addKind}
                onValueChange={(v) => setAddKind(v as ParticipantKind)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="member">Member</SelectItem>
                  <SelectItem value="agent">Agent</SelectItem>
                  <SelectItem value="team">Team</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <AddParticipantPicker
              kind={addKind}
              memberCandidates={memberCandidates}
              agentCandidates={agentCandidates}
              teamCandidates={teamCandidates}
              getMemberName={getMemberName}
              getAgentName={getAgentName}
              disabled={addBusy}
              onPick={(type, id) => void handleAdd(type, id)}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setAddOpen(false)}>
              Close
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function AddParticipantPicker({
  kind,
  memberCandidates,
  agentCandidates,
  teamCandidates,
  getMemberName,
  getAgentName,
  disabled,
  onPick,
}: {
  kind: ParticipantKind;
  memberCandidates: { user_id: string }[];
  agentCandidates: { id: string; name: string }[];
  teamCandidates: { id: string; name: string }[];
  getMemberName: (id: string) => string;
  getAgentName: (id: string) => string;
  disabled: boolean;
  onPick: (type: ParticipantKind, id: string) => void;
}) {
  if (kind === "member") {
    if (memberCandidates.length === 0) {
      return <p className="text-sm text-muted-foreground">Everyone is already in this channel.</p>;
    }
    return (
      <Command className="rounded-lg border">
        <CommandInput placeholder="Search members…" disabled={disabled} />
        <CommandList className="max-h-48">
          <CommandEmpty>No matches.</CommandEmpty>
          <CommandGroup>
            {memberCandidates.map((m) => (
              <CommandItem
                key={m.user_id}
                value={getMemberName(m.user_id)}
                disabled={disabled}
                onSelect={() => onPick("member", m.user_id)}
              >
                <User className="mr-2 size-3.5 text-muted-foreground" aria-hidden="true" />
                {getMemberName(m.user_id)}
              </CommandItem>
            ))}
          </CommandGroup>
        </CommandList>
      </Command>
    );
  }

  if (kind === "agent") {
    if (agentCandidates.length === 0) {
      return <p className="text-sm text-muted-foreground">No agents to add.</p>;
    }
    return (
      <Command className="rounded-lg border">
        <CommandInput placeholder="Search agents…" disabled={disabled} />
        <CommandList className="max-h-48">
          <CommandEmpty>No matches.</CommandEmpty>
          <CommandGroup>
            {agentCandidates.map((a) => (
              <CommandItem
                key={a.id}
                value={`${a.name} ${getAgentName(a.id)}`}
                disabled={disabled}
                onSelect={() => onPick("agent", a.id)}
              >
                <Bot className="mr-2 size-3.5 text-muted-foreground" aria-hidden="true" />
                {a.name}
              </CommandItem>
            ))}
          </CommandGroup>
        </CommandList>
      </Command>
    );
  }

  if (teamCandidates.length === 0) {
    return <p className="text-sm text-muted-foreground">No teams to add.</p>;
  }
  return (
    <Command className="rounded-lg border">
      <CommandInput placeholder="Search teams…" disabled={disabled} />
      <CommandList className="max-h-48">
        <CommandEmpty>No matches.</CommandEmpty>
        <CommandGroup>
          {teamCandidates.map((t) => (
            <CommandItem
              key={t.id}
              value={t.name}
              disabled={disabled}
              onSelect={() => onPick("team", t.id)}
            >
              <Users className="mr-2 size-3.5 text-muted-foreground" aria-hidden="true" />
              {t.name}
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </Command>
  );
}
