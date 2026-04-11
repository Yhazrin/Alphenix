"use client";

import { useMemo, useState, type ReactNode } from "react";
import { useShallow } from "zustand/react/shallow";
import {
  ArrowDown,
  ArrowUp,
  Check,
  ChevronDown,
  CircleDot,
  Filter,
  SignalHigh,
  SlidersHorizontal,
  User,
  UserMinus,
  UserPen,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuCheckboxItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
} from "@/components/ui/dropdown-menu";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "@/components/ui/popover";
import { Switch } from "@/components/ui/switch";
import {
  ALL_STATUSES,
  STATUS_CONFIG,
  PRIORITY_ORDER,
  PRIORITY_CONFIG,
} from "@/features/issues/config";
import { StatusIcon, PriorityIcon } from "@/features/issues/components";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@core/hooks";
import { memberListOptions, agentListOptions } from "@core/workspace/queries";
import { ActorAvatar } from "@/components/common/actor-avatar";
import {
  useIssueViewStore,
  SORT_OPTIONS,
  CARD_PROPERTY_OPTIONS,
  type ActorFilterValue,
} from "@/features/issues/stores/view-store";
import {
  useIssuesScopeStore,
  type IssuesScope,
} from "@/features/issues/stores/issues-scope-store";
import {
  useIssuesMeFilterStore,
  type IssuesMeFilter,
} from "@/features/issues/stores/me-filter-store";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { ScopePillRail } from "@/components/common/scope-pill-rail";
import { cn } from "@/lib/utils";
import type { Issue } from "@/shared/types";

// ---------------------------------------------------------------------------
// HoverCheck — shadcn official pattern (PR #6862)
// ---------------------------------------------------------------------------

const FILTER_ITEM_CLASS =
  "group/fitem pr-1.5 [&>[data-slot=dropdown-menu-checkbox-item-indicator]]:hidden";

function HoverCheck({ checked }: { checked: boolean }) {
  return (
    <div
      className="border-input data-[selected=true]:border-primary data-[selected=true]:bg-primary data-[selected=true]:text-primary-foreground pointer-events-none size-4 shrink-0 rounded-[4px] border transition-all select-none *:[svg]:opacity-0 data-[selected=true]:*:[svg]:opacity-100 opacity-0 group-hover/fitem:opacity-100 group-focus/fitem:opacity-100 data-[selected=true]:opacity-100"
      data-selected={checked}
    >
      <Check className="size-3.5 text-current" aria-hidden="true" />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function getActiveFilterCount(state: {
  statusFilters: string[];
  priorityFilters: string[];
  assigneeFilters: ActorFilterValue[];
  includeNoAssignee: boolean;
  creatorFilters: ActorFilterValue[];
}) {
  let count = 0;
  if (state.statusFilters.length > 0) count++;
  if (state.priorityFilters.length > 0) count++;
  if (state.assigneeFilters.length > 0 || state.includeNoAssignee) count++;
  if (state.creatorFilters.length > 0) count++;
  return count;
}

function useIssueCounts(allIssues: Issue[]) {
  return useMemo(() => {
    const status = new Map<string, number>();
    const priority = new Map<string, number>();
    const assignee = new Map<string, number>();
    const creator = new Map<string, number>();
    let noAssignee = 0;

    for (const issue of allIssues) {
      status.set(issue.status, (status.get(issue.status) ?? 0) + 1);
      priority.set(issue.priority, (priority.get(issue.priority) ?? 0) + 1);

      if (!issue.assignee_id) {
        noAssignee++;
      } else {
        const aKey = `${issue.assignee_type}:${issue.assignee_id}`;
        assignee.set(aKey, (assignee.get(aKey) ?? 0) + 1);
      }

      const cKey = `${issue.creator_type}:${issue.creator_id}`;
      creator.set(cKey, (creator.get(cKey) ?? 0) + 1);
    }

    return { status, priority, assignee, creator, noAssignee };
  }, [allIssues]);
}

// ---------------------------------------------------------------------------
// Scope config
// ---------------------------------------------------------------------------

const SCOPES: { value: IssuesScope; label: string; description: string }[] = [
  { value: "all", label: "All", description: "All issues in this workspace" },
  { value: "members", label: "Members", description: "Issues assigned to team members" },
  { value: "agents", label: "Agents", description: "Issues assigned to AI agents" },
];

const ME_FILTERS: { value: IssuesMeFilter; label: string; description: string }[] = [
  { value: "off", label: "Everyone", description: "All issues in the workspace (with channel filter)" },
  { value: "assigned", label: "Assigned to me", description: "Issues assigned to you" },
  { value: "created", label: "Created by me", description: "Issues you created" },
  { value: "my_agents", label: "My agents", description: "Issues assigned to agents you own" },
];

// ---------------------------------------------------------------------------
// Actor sub-menu content (shared between Assignee and Creator)
// ---------------------------------------------------------------------------

function ActorSubContent({
  counts,
  selected,
  onToggle,
  showNoAssignee,
  includeNoAssignee,
  onToggleNoAssignee,
  noAssigneeCount,
}: {
  counts: Map<string, number>;
  selected: ActorFilterValue[];
  onToggle: (value: ActorFilterValue) => void;
  showNoAssignee?: boolean;
  includeNoAssignee?: boolean;
  onToggleNoAssignee?: () => void;
  noAssigneeCount?: number;
}) {
  const [search, setSearch] = useState("");
  const wsId = useWorkspaceId();
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const query = search.toLowerCase();
  const filteredMembers = members.filter((m) =>
    m.name.toLowerCase().includes(query),
  );
  const filteredAgents = agents.filter((a) =>
    !a.archived_at && a.name.toLowerCase().includes(query),
  );

  const isSelected = (type: "member" | "agent", id: string) =>
    selected.some((f) => f.type === type && f.id === id);

  return (
    <>
      <div className="px-2 py-1.5 border-b border-foreground/5">
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Filter..."
          className="w-full bg-transparent text-sm placeholder:text-muted-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring rounded"
          autoFocus
        />
      </div>

      <div className="max-h-64 overflow-y-auto p-1">
        {showNoAssignee &&
          (!query || "no assignee".includes(query) || "unassigned".includes(query)) && (
            <DropdownMenuCheckboxItem
              checked={includeNoAssignee ?? false}
              onCheckedChange={() => onToggleNoAssignee?.()}
              className={FILTER_ITEM_CLASS}
            >
              <HoverCheck checked={includeNoAssignee ?? false} />
              <UserMinus className="size-3.5 text-muted-foreground" aria-hidden="true" />
              No assignee
              {(noAssigneeCount ?? 0) > 0 && (
                <span className="ml-auto text-xs text-muted-foreground">
                  {noAssigneeCount}
                </span>
              )}
            </DropdownMenuCheckboxItem>
          )}

        {filteredMembers.length > 0 && (
          <DropdownMenuGroup>
            <DropdownMenuLabel>Members</DropdownMenuLabel>
            {filteredMembers.map((m) => {
              const checked = isSelected("member", m.user_id);
              const count = counts.get(`member:${m.user_id}`) ?? 0;
              return (
                <DropdownMenuCheckboxItem
                  key={m.user_id}
                  checked={checked}
                  onCheckedChange={() =>
                    onToggle({ type: "member", id: m.user_id })
                  }
                  className={FILTER_ITEM_CLASS}
                >
                  <HoverCheck checked={checked} />
                  <ActorAvatar actorType="member" actorId={m.user_id} size={18} />
                  <span className="truncate">{m.name}</span>
                  {count > 0 && (
                    <span className="ml-auto text-xs text-muted-foreground">
                      {count}
                    </span>
                  )}
                </DropdownMenuCheckboxItem>
              );
            })}
          </DropdownMenuGroup>
        )}

        {filteredAgents.length > 0 && (
          <DropdownMenuGroup>
            <DropdownMenuLabel>Agents</DropdownMenuLabel>
            {filteredAgents.map((a) => {
              const checked = isSelected("agent", a.id);
              const count = counts.get(`agent:${a.id}`) ?? 0;
              return (
                <DropdownMenuCheckboxItem
                  key={a.id}
                  checked={checked}
                  onCheckedChange={() =>
                    onToggle({ type: "agent", id: a.id })
                  }
                  className={FILTER_ITEM_CLASS}
                >
                  <HoverCheck checked={checked} />
                  <ActorAvatar actorType="agent" actorId={a.id} size={18} />
                  <span className="truncate">{a.name}</span>
                  {count > 0 && (
                    <span className="ml-auto text-xs text-muted-foreground">
                      {count}
                    </span>
                  )}
                </DropdownMenuCheckboxItem>
              );
            })}
          </DropdownMenuGroup>
        )}

        {filteredMembers.length === 0 && filteredAgents.length === 0 && search && (
          <div className="px-2 py-3 text-center text-sm text-muted-foreground">
            No results
          </div>
        )}
      </div>
    </>
  );
}

// ---------------------------------------------------------------------------
// IssuesHeader
// ---------------------------------------------------------------------------

export function IssuesHeader({
  scopedIssues,
  leadingSlot,
}: {
  scopedIssues: Issue[];
  /** Workspace breadcrumb — when set, scope pills center in remaining space */
  leadingSlot?: ReactNode;
}) {
  const scope = useIssuesScopeStore((s) => s.scope);
  const setScope = useIssuesScopeStore((s) => s.setScope);
  const meFilter = useIssuesMeFilterStore((s) => s.meFilter);
  const setMeFilter = useIssuesMeFilterStore((s) => s.setMeFilter);

  const {
    statusFilters,
    priorityFilters,
    assigneeFilters,
    includeNoAssignee,
    creatorFilters,
    sortBy,
    sortDirection,
    cardProperties,
  } = useIssueViewStore(useShallow((s) => ({
    statusFilters: s.statusFilters,
    priorityFilters: s.priorityFilters,
    assigneeFilters: s.assigneeFilters,
    includeNoAssignee: s.includeNoAssignee,
    creatorFilters: s.creatorFilters,
    sortBy: s.sortBy,
    sortDirection: s.sortDirection,
    cardProperties: s.cardProperties,
  })));
  const act = useIssueViewStore.getState();

  const counts = useIssueCounts(scopedIssues);

  const scopeCounts = useMemo(() => {
    let membersCount = 0;
    let agentsCount = 0;
    for (const issue of scopedIssues) {
      if (issue.assignee_type === "member") membersCount++;
      else if (issue.assignee_type === "agent") agentsCount++;
    }
    return { all: scopedIssues.length, members: membersCount, agents: agentsCount };
  }, [scopedIssues]);

  const hasActiveFilters =
    meFilter !== "off" ||
    getActiveFilterCount({
      statusFilters,
      priorityFilters,
      assigneeFilters,
      includeNoAssignee,
      creatorFilters,
    }) > 0;

  const sortLabel =
    SORT_OPTIONS.find((o) => o.value === sortBy)?.label ?? "Last updated";

  const meFilterLabel =
    ME_FILTERS.find((f) => f.value === meFilter)?.label ?? "Everyone";

  return (
    <div className="flex h-14 min-h-14 shrink-0 items-center gap-3 border-b px-4">
      {leadingSlot ? (
        <div className="min-w-0 max-w-[min(44vw,300px)] shrink-0 overflow-hidden">
          {leadingSlot}
        </div>
      ) : null}
      <div
        className={cn(
          "flex min-h-9 min-w-0 flex-1 items-center overflow-x-auto [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden",
          leadingSlot ? "justify-center" : "justify-start",
        )}
      >
        <DropdownMenu>
          <Tooltip>
            <DropdownMenuTrigger
              render={
                <TooltipTrigger
                  render={
                    <Button
                      variant="outline"
                      size="sm"
                      className="relative mr-2 shrink-0 gap-1 rounded-full px-2.5 text-xs font-medium"
                      aria-label="Focus: whose issues to show"
                      data-testid="issues-focus-filter"
                    >
                      {meFilterLabel}
                      <ChevronDown className="size-3 opacity-60" aria-hidden="true" />
                      {meFilter !== "off" && (
                        <span
                          className="absolute top-0.5 right-1 size-1.5 rounded-full bg-primary"
                          aria-hidden="true"
                        />
                      )}
                    </Button>
                  }
                />
              }
            />
            <TooltipContent side="bottom">Narrow to your work</TooltipContent>
          </Tooltip>
          <DropdownMenuContent align="start" className="w-56">
            <DropdownMenuLabel>Focus</DropdownMenuLabel>
            <DropdownMenuGroup>
              {ME_FILTERS.map((f) => (
                <DropdownMenuItem
                  key={f.value}
                  onClick={() => setMeFilter(f.value)}
                  title={f.description}
                >
                  <span className="flex-1">{f.label}</span>
                  {meFilter === f.value ? (
                    <Check className="ml-auto size-3.5 shrink-0" aria-hidden="true" />
                  ) : null}
                </DropdownMenuItem>
              ))}
            </DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
        <ScopePillRail
          items={SCOPES.map((s) => ({
            value: s.value,
            label: s.label,
            description: s.description,
            badge: scopeCounts[s.value],
          }))}
          value={scope}
          onChange={setScope}
          className="shrink-0"
        />
      </div>

      <div className="flex shrink-0 items-center gap-1 border-l border-border/50 pl-2">
        {/* Filter */}
        <DropdownMenu>
          <Tooltip>
            <DropdownMenuTrigger
              render={
                <TooltipTrigger
                  render={
                    <Button variant="outline" size="icon-sm" className="relative rounded-full text-muted-foreground transition-colors duration-200 ease-out" aria-label="Filter issues">
                      <Filter className="size-4" aria-hidden="true" />
                      {hasActiveFilters && (
                        <span className="absolute top-0 right-0 size-1.5 rounded-full bg-brand" />
                      )}
                    </Button>
                  }
                />
              }
            />
            <TooltipContent side="bottom">Filter</TooltipContent>
          </Tooltip>
          <DropdownMenuContent align="end" className="w-auto">
            <DropdownMenuLabel>Focus</DropdownMenuLabel>
            <DropdownMenuGroup>
              {ME_FILTERS.map((f) => (
                <DropdownMenuItem
                  key={f.value}
                  onClick={() => setMeFilter(f.value)}
                  title={f.description}
                >
                  <span className="flex-1">{f.label}</span>
                  {meFilter === f.value ? (
                    <Check className="ml-auto size-3.5 shrink-0" aria-hidden="true" />
                  ) : null}
                </DropdownMenuItem>
              ))}
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
            {/* Status */}
            <DropdownMenuSub>
              <DropdownMenuSubTrigger>
                <CircleDot className="size-3.5" aria-hidden="true" />
                <span className="flex-1">Status</span>
                {statusFilters.length > 0 && (
                  <span className="text-xs text-primary font-medium">
                    {statusFilters.length}
                  </span>
                )}
              </DropdownMenuSubTrigger>
              <DropdownMenuSubContent className="w-auto min-w-48">
                {ALL_STATUSES.map((s) => {
                  const checked = statusFilters.includes(s);
                  const count = counts.status.get(s) ?? 0;
                  return (
                    <DropdownMenuCheckboxItem
                      key={s}
                      checked={checked}
                      onCheckedChange={() => act.toggleStatusFilter(s)}
                      className={FILTER_ITEM_CLASS}
                    >
                      <HoverCheck checked={checked} />
                      <StatusIcon status={s} className="h-3.5 w-3.5" />
                      {STATUS_CONFIG[s].label}
                      {count > 0 && (
                        <span className="ml-auto text-xs text-muted-foreground">
                          {count} {count === 1 ? "issue" : "issues"}
                        </span>
                      )}
                    </DropdownMenuCheckboxItem>
                  );
                })}
              </DropdownMenuSubContent>
            </DropdownMenuSub>

            {/* Priority */}
            <DropdownMenuSub>
              <DropdownMenuSubTrigger>
                <SignalHigh className="size-3.5" aria-hidden="true" />
                <span className="flex-1">Priority</span>
                {priorityFilters.length > 0 && (
                  <span className="text-xs text-primary font-medium">
                    {priorityFilters.length}
                  </span>
                )}
              </DropdownMenuSubTrigger>
              <DropdownMenuSubContent className="w-auto min-w-44">
                {PRIORITY_ORDER.map((p) => {
                  const checked = priorityFilters.includes(p);
                  const count = counts.priority.get(p) ?? 0;
                  return (
                    <DropdownMenuCheckboxItem
                      key={p}
                      checked={checked}
                      onCheckedChange={() => act.togglePriorityFilter(p)}
                      className={FILTER_ITEM_CLASS}
                    >
                      <HoverCheck checked={checked} />
                      <PriorityIcon priority={p} />
                      {PRIORITY_CONFIG[p].label}
                      {count > 0 && (
                        <span className="ml-auto text-xs text-muted-foreground">
                          {count} {count === 1 ? "issue" : "issues"}
                        </span>
                      )}
                    </DropdownMenuCheckboxItem>
                  );
                })}
              </DropdownMenuSubContent>
            </DropdownMenuSub>

            {/* Assignee */}
            <DropdownMenuSub>
              <DropdownMenuSubTrigger>
                <User className="size-3.5" aria-hidden="true" />
                <span className="flex-1">Assignee</span>
                {(assigneeFilters.length > 0 || includeNoAssignee) && (
                  <span className="text-xs text-primary font-medium">
                    {assigneeFilters.length + (includeNoAssignee ? 1 : 0)}
                  </span>
                )}
              </DropdownMenuSubTrigger>
              <DropdownMenuSubContent className="w-auto min-w-52 p-0">
                <ActorSubContent
                  counts={counts.assignee}
                  selected={assigneeFilters}
                  onToggle={act.toggleAssigneeFilter}
                  showNoAssignee
                  includeNoAssignee={includeNoAssignee}
                  onToggleNoAssignee={act.toggleNoAssignee}
                  noAssigneeCount={counts.noAssignee}
                />
              </DropdownMenuSubContent>
            </DropdownMenuSub>

            {/* Creator */}
            <DropdownMenuSub>
              <DropdownMenuSubTrigger>
                <UserPen className="size-3.5" aria-hidden="true" />
                <span className="flex-1">Creator</span>
                {creatorFilters.length > 0 && (
                  <span className="text-xs text-primary font-medium">
                    {creatorFilters.length}
                  </span>
                )}
              </DropdownMenuSubTrigger>
              <DropdownMenuSubContent className="w-auto min-w-52 p-0">
                <ActorSubContent
                  counts={counts.creator}
                  selected={creatorFilters}
                  onToggle={act.toggleCreatorFilter}
                />
              </DropdownMenuSubContent>
            </DropdownMenuSub>

            {/* Reset */}
            {hasActiveFilters && (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={act.clearFilters}>
                  Reset all filters
                </DropdownMenuItem>
              </>
            )}
          </DropdownMenuContent>
        </DropdownMenu>

        {/* Display settings */}
        <Popover>
          <Tooltip>
            <PopoverTrigger
              render={
                <TooltipTrigger
                  render={
                    <Button variant="outline" size="icon-sm" className="rounded-full text-muted-foreground transition-colors duration-200 ease-out" aria-label="Display settings">
                      <SlidersHorizontal className="size-4" aria-hidden="true" />
                    </Button>
                  }
                />
              }
            />
            <TooltipContent side="bottom">Display settings</TooltipContent>
          </Tooltip>
          <PopoverContent align="end" className="w-64 p-0">
            <div className="border-b px-3 py-2.5">
              <span className="text-xs font-medium text-muted-foreground">
                Ordering
              </span>
              <div className="mt-2 flex items-center gap-1.5">
                <DropdownMenu>
                  <DropdownMenuTrigger
                    render={
                      <Button
                        variant="outline"
                        size="sm"
                        className="flex-1 justify-between text-xs"
                      >
                        {sortLabel}
                        <ChevronDown className="size-3 text-muted-foreground" aria-hidden="true" />
                      </Button>
                    }
                  />
                  <DropdownMenuContent align="start" className="w-auto">
                    {SORT_OPTIONS.map((opt) => (
                      <DropdownMenuItem
                        key={opt.value}
                        onClick={() => act.setSortBy(opt.value)}
                      >
                        {opt.label}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
                <Button
                  variant="outline"
                  size="icon-sm"
                  className="rounded-full transition-colors duration-200 ease-out"
                  onClick={() =>
                    act.setSortDirection(sortDirection === "asc" ? "desc" : "asc")
                  }
                  aria-label={sortDirection === "asc" ? "Sort ascending" : "Sort descending"}
                >
                  {sortDirection === "asc" ? (
                    <ArrowUp className="size-3.5" aria-hidden="true" />
                  ) : (
                    <ArrowDown className="size-3.5" aria-hidden="true" />
                  )}
                </Button>
              </div>
            </div>

            <div className="px-3 py-2.5">
              <span className="text-xs font-medium text-muted-foreground">
                Card properties
              </span>
              <div className="mt-2 space-y-2">
                {CARD_PROPERTY_OPTIONS.map((opt) => (
                  <label
                    key={opt.key}
                    htmlFor={`card-prop-${opt.key}`}
                    className="flex cursor-pointer items-center justify-between"
                  >
                    <span className="text-sm">{opt.label}</span>
                    <Switch
                      id={`card-prop-${opt.key}`}
                      size="sm"
                      checked={cardProperties[opt.key]}
                      onCheckedChange={() => act.toggleCardProperty(opt.key)}
                    />
                  </label>
                ))}
              </div>
            </div>
          </PopoverContent>
        </Popover>
      </div>
    </div>
  );
}
