"use client";

import { useState, useEffect } from "react";
import { Lock, UserMinus, Users } from "lucide-react";
import type { Agent, IssueAssigneeType, Team, UpdateIssueRequest } from "@/shared/types";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore, useActorName } from "@/features/workspace";
import { ActorAvatar } from "@/components/common/actor-avatar";
import {
  PropertyPicker,
  PickerItem,
  PickerSection,
  PickerEmpty,
} from "./property-picker";
import { api } from "@/shared/api";

export function canAssignAgent(agent: Agent, userId: string | undefined, memberRole: string | undefined): boolean {
  if (agent.visibility !== "private") return true;
  if (agent.owner_id === userId) return true;
  if (memberRole === "owner" || memberRole === "admin") return true;
  return false;
}

export function AssigneePicker({
  assigneeType,
  assigneeId,
  onUpdate,
  trigger: customTrigger,
  triggerRender,
  open: controlledOpen,
  onOpenChange: controlledOnOpenChange,
  align,
}: {
  assigneeType: IssueAssigneeType | null;
  assigneeId: string | null;
  onUpdate: (updates: Partial<UpdateIssueRequest>) => void;
  trigger?: React.ReactNode;
  triggerRender?: React.ReactElement;
  open?: boolean;
  onOpenChange?: (v: boolean) => void;
  align?: "start" | "center" | "end";
}) {
  const [internalOpen, setInternalOpen] = useState(false);
  const open = controlledOpen ?? internalOpen;
  const setOpen = controlledOnOpenChange ?? setInternalOpen;
  const [filter, setFilter] = useState("");
  const [teams, setTeams] = useState<Team[]>([]);
  const [loadingTeams, setLoadingTeams] = useState(false);
  const user = useAuthStore((s) => s.user);
  const members = useWorkspaceStore((s) => s.members);
  const agents = useWorkspaceStore((s) => s.agents);
  const { getActorName } = useActorName();

  const currentMember = members.find((m) => m.user_id === user?.id);
  const memberRole = currentMember?.role;

  // Fetch teams when picker opens
  useEffect(() => {
    if (!open) return;
    if (teams.length > 0) return; // already loaded
    let cancelled = false;
    setLoadingTeams(true);
    api.listTeams()
      .then((data) => { if (!cancelled) setTeams(data.filter((t) => !t.archived_at)); })
      .catch(() => { /* silent fail */ })
      .finally(() => { if (!cancelled) setLoadingTeams(false); });
    return () => { cancelled = true; };
  }, [open]);

  const query = filter.toLowerCase();
  const filteredMembers = members.filter((m) =>
    m.name.toLowerCase().includes(query),
  );
  const filteredAgents = agents.filter((a) =>
    !a.archived_at && a.name.toLowerCase().includes(query),
  );
  const filteredTeams = teams.filter((t) =>
    t.name.toLowerCase().includes(query),
  );

  const isSelected = (type: string, id: string) =>
    assigneeType === type && assigneeId === id;

  const triggerLabel =
    assigneeType && assigneeId
      ? assigneeType === "team"
        ? teams.find((t) => t.id === assigneeId)?.name ?? "Team"
        : getActorName(assigneeType, assigneeId)
      : "Unassigned";

  return (
    <PropertyPicker
      open={open}
      onOpenChange={(v: boolean) => {
        setOpen(v);
        if (!v) setFilter("");
      }}
      width="w-52"
      align={align}
      searchable
      searchPlaceholder="Assign to..."
      onSearchChange={setFilter}
      triggerRender={triggerRender}
      trigger={
        customTrigger ? customTrigger : assigneeType && assigneeId ? (
          <>
            {assigneeType === "team" ? (
              <div className="flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded-md bg-primary/10">
                <Users className="h-3 w-3 text-primary" aria-hidden="true" />
              </div>
            ) : (
              <ActorAvatar actorType={assigneeType} actorId={assigneeId} size={18} />
            )}
            <span className="truncate">{triggerLabel}</span>
          </>
        ) : (
          <span className="text-muted-foreground">Unassigned</span>
        )
      }
    >
      {/* Unassigned option */}
      <PickerItem
        selected={!assigneeType && !assigneeId}
        onClick={() => {
          onUpdate({ assignee_type: null, assignee_id: null });
          setOpen(false);
        }}
      >
        <UserMinus className="h-3.5 w-3.5 text-muted-foreground" aria-hidden="true" />
        <span className="text-muted-foreground">Unassigned</span>
      </PickerItem>

      {/* Members */}
      {filteredMembers.length > 0 && (
        <PickerSection label="Members">
          {filteredMembers.map((m) => (
            <PickerItem
              key={m.user_id}
              selected={isSelected("member", m.user_id)}
              onClick={() => {
                onUpdate({
                  assignee_type: "member",
                  assignee_id: m.user_id,
                });
                setOpen(false);
              }}
            >
              <ActorAvatar actorType="member" actorId={m.user_id} size={18} />
              <span>{m.name}</span>
            </PickerItem>
          ))}
        </PickerSection>
      )}

      {/* Agents */}
      {filteredAgents.length > 0 && (
        <PickerSection label="Agents">
          {filteredAgents.map((a) => {
            const allowed = canAssignAgent(a, user?.id, memberRole);
            return (
              <PickerItem
                key={a.id}
                selected={isSelected("agent", a.id)}
                disabled={!allowed}
                onClick={() => {
                  if (!allowed) return;
                  onUpdate({
                    assignee_type: "agent",
                    assignee_id: a.id,
                  });
                  setOpen(false);
                }}
              >
                <ActorAvatar actorType="agent" actorId={a.id} size={18} />
                <span className={allowed ? "" : "text-muted-foreground"}>{a.name}</span>
                {a.visibility === "private" && (
                  <Lock className="ml-auto h-3 w-3 text-muted-foreground" aria-hidden="true" />
                )}
              </PickerItem>
            );
          })}
        </PickerSection>
      )}

      {/* Teams */}
      {(filteredTeams.length > 0 || loadingTeams) && (
        <PickerSection label="Teams">
          {loadingTeams ? (
            <PickerItem selected={false} disabled onClick={() => {}}>
              <span className="text-muted-foreground text-xs">Loading teams...</span>
            </PickerItem>
          ) : (
            filteredTeams.map((t) => (
              <PickerItem
                key={t.id}
                selected={isSelected("team", t.id)}
                onClick={() => {
                  onUpdate({
                    assignee_type: "team",
                    assignee_id: t.id,
                  });
                  setOpen(false);
                }}
              >
                <div className="flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded-md bg-primary/10">
                  <Users className="h-3 w-3 text-primary" aria-hidden="true" />
                </div>
                <span>{t.name}</span>
              </PickerItem>
            ))
          )}
        </PickerSection>
      )}

      {filteredMembers.length === 0 &&
        filteredAgents.length === 0 &&
        filteredTeams.length === 0 &&
        filter && <PickerEmpty />}
    </PropertyPicker>
  );
}
