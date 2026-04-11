"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import {
  Inbox,
  ListTodo,
  Bot,
  Monitor,
  ChevronDown,
  Settings,
  LogOut,
  Plus,
  Check,
  BookOpenText,
  SquarePen,
  Search,
  Server,
  Hash,
  Layers,
} from "lucide-react";
import { WorkspaceAvatar } from "@/features/workspace";
import { useIssueDraftStore } from "@/features/issues/stores/draft-store";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarFooter,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuAction,
  SidebarRail,
} from "@/components/ui/sidebar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { useAuthStore } from "@/features/auth";
import { clearLoggedInCookie } from "@/features/auth/auth-cookie";
import { useWorkspaceStore } from "@/features/workspace";
import { useInboxStore } from "@/features/inbox";
import { useModalStore } from "@/features/modals";
import { ConnectionStatus } from "@/components/connection-status";
import { useChannelStore } from "@/features/channels";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";

const workspaceNav = [
  { href: "/agents", label: "Agents & teams", icon: Bot },
  { href: "/runtimes", label: "Runtimes", icon: Monitor },
  { href: "/skills", label: "Skills", icon: BookOpenText },
  { href: "/mcp", label: "MCP Servers", icon: Server },
  { href: "/settings", label: "Settings", icon: Settings },
];

function DraftDot() {
  const hasDraft = useIssueDraftStore((s) => !!(s.draft.title || s.draft.description));
  if (!hasDraft) return null;
  return <span className="absolute top-0 right-0 size-1.5 rounded-full bg-brand" aria-hidden="true" />;
}

export function AppSidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const authLogout = useAuthStore((s) => s.logout);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const workspaces = useWorkspaceStore((s) => s.workspaces);
  const switchWorkspace = useWorkspaceStore((s) => s.switchWorkspace);

  const channels = useChannelStore((s) => s.channels);
  const filterChannelId = useChannelStore((s) => s.filterChannelId);
  const channelsLoading = useChannelStore((s) => s.loading);
  const setFilterChannelId = useChannelStore((s) => s.setFilterChannelId);
  const createChannel = useChannelStore((s) => s.createChannel);

  const [createChannelOpen, setCreateChannelOpen] = useState(false);
  const [newChannelName, setNewChannelName] = useState("");
  const [creatingChannel, setCreatingChannel] = useState(false);

  const unreadCount = useInboxStore((s) => s.unreadCount);

  useEffect(() => {
    if (!workspace?.id) return;
    void useChannelStore.getState().fetchChannels();
  }, [workspace?.id]);

  const sortedChannels = [...channels].sort((a, b) => {
    if (a.is_default && !b.is_default) return -1;
    if (!a.is_default && b.is_default) return 1;
    return a.name.localeCompare(b.name);
  });

  const logout = async () => {
    clearLoggedInCookie();
    try {
      await authLogout();
    } catch (e) {
      console.error("Logout failed:", e);
    } finally {
      useWorkspaceStore.getState().clearWorkspace();
      router.push("/");
    }
  };

  return (
      <Sidebar variant="inset">
        {/* Workspace Switcher */}
        <SidebarHeader className="py-3">
          <div className="flex items-center gap-4">
            <SidebarMenu className="min-w-0 flex-1">
              <SidebarMenuItem>
                <DropdownMenu>
                  <DropdownMenuTrigger
                    render={
                      <SidebarMenuButton data-testid="workspace-menu-trigger">
                        <WorkspaceAvatar name={workspace?.name ?? "M"} size="sm" />
                        <span className="flex-1 truncate font-medium">
                          {workspace?.name ?? "Alphenix"}
                        </span>
                        <ChevronDown className="size-3 text-muted-foreground" aria-hidden="true" />
                      </SidebarMenuButton>
                    }
                  />
                <DropdownMenuContent
                  className="w-52"
                  align="start"
                  side="bottom"
                  sideOffset={4}
                >
                  <DropdownMenuGroup>
                    <DropdownMenuLabel className="text-xs text-muted-foreground">
                      {user?.email}
                    </DropdownMenuLabel>
                  </DropdownMenuGroup>
                  <DropdownMenuSeparator />
                  <DropdownMenuGroup className="group/ws-section">
                    <DropdownMenuLabel className="flex items-center text-xs text-muted-foreground">
                      Workspaces
                      <Tooltip>
                        <TooltipTrigger
                          aria-label="Create workspace"
                          className="ml-auto opacity-0 group-hover/ws-section:opacity-100 transition-opacity rounded hover:bg-accent p-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                          onClick={() => useModalStore.getState().open("create-workspace")}
                        >
                          <Plus className="h-3.5 w-3.5" aria-hidden="true" />
                        </TooltipTrigger>
                        <TooltipContent side="right">
                          Create workspace
                        </TooltipContent>
                      </Tooltip>
                    </DropdownMenuLabel>
                    {workspaces.map((ws) => (
                      <DropdownMenuItem
                        key={ws.id}
                        onClick={() => {
                          if (ws.id !== workspace?.id) {
                            switchWorkspace(ws.id);
                          }
                        }}
                      >
                        <WorkspaceAvatar name={ws.name} size="sm" />
                        <span className="flex-1 truncate">{ws.name}</span>
                        {ws.id === workspace?.id && (
                          <Check className="h-3.5 w-3.5 text-primary" aria-hidden="true" />
                        )}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuGroup>
                  <DropdownMenuSeparator />
                  <DropdownMenuGroup>
                    <DropdownMenuItem variant="destructive" onClick={logout} data-testid="auth-logout-button">
                      <LogOut className="h-3.5 w-3.5" aria-hidden="true" />
                      Log out
                    </DropdownMenuItem>
                  </DropdownMenuGroup>
                </DropdownMenuContent>
                </DropdownMenu>
              </SidebarMenuItem>
            </SidebarMenu>
            <Tooltip>
              <TooltipTrigger
                className="flex h-7 flex-1 items-center gap-1.5 rounded-full border border-border/50 bg-background/70 px-2.5 text-xs text-muted-foreground shadow-sm backdrop-blur-md transition-colors duration-200 ease-out hover:border-border hover:bg-background/85 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring dark:bg-background/45"
                onClick={() => useModalStore.getState().open("search")}
              >
                <Search className="size-3" aria-hidden="true" />
                <span className="flex-1 text-left">Search...</span>
                <kbd className="pointer-events-none hidden h-4 select-none items-center gap-0.5 rounded border bg-muted px-1 font-mono text-[10px] font-medium opacity-100 sm:flex">
                  <span className="text-xs">&#x2318;</span>K
                </kbd>
              </TooltipTrigger>
              <TooltipContent side="bottom">Search issues, agents, settings</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger
                render={
                  <Link
                    href="/inbox"
                    data-testid="header-inbox-link"
                    className="relative flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-border/40 bg-background/80 text-foreground shadow-sm backdrop-blur-md transition-colors duration-200 ease-out hover:border-border hover:bg-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring dark:bg-background/50"
                    aria-label="Inbox"
                  >
                    <Inbox className="size-3.5" aria-hidden="true" />
                    {unreadCount > 0 && (
                      <span className="absolute -right-0.5 -top-0.5 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-primary px-0.5 text-[9px] font-semibold text-primary-foreground tabular-nums">
                        {unreadCount > 99 ? "99+" : unreadCount}
                      </span>
                    )}
                  </Link>
                }
              />
              <TooltipContent side="bottom">Inbox</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger
                aria-label="New issue"
                data-testid="new-issue-button"
                className="relative flex h-7 w-7 items-center justify-center rounded-full border border-border/40 bg-background/80 text-foreground shadow-sm backdrop-blur-md transition-colors duration-200 ease-out hover:border-border hover:bg-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring dark:bg-background/50"
                onClick={() => useModalStore.getState().open("create-issue")}
              >
                <SquarePen className="size-3.5" aria-hidden="true" />
                <DraftDot />
              </TooltipTrigger>
              <TooltipContent side="bottom">New issue</TooltipContent>
            </Tooltip>
          </div>
        </SidebarHeader>

        {/* Navigation */}
        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupContent>
              <SidebarMenu className="gap-0.5">
                <SidebarMenuItem>
                  <SidebarMenuButton
                    isActive={pathname === "/issues" || pathname.startsWith("/issues/")}
                    render={<Link href="/issues" data-testid="sidebar-nav-issues" />}
                    className="rounded-full text-muted-foreground transition-colors duration-200 ease-out hover:bg-sidebar-accent/50 hover:text-sidebar-accent-foreground data-active:font-medium"
                  >
                    <ListTodo aria-hidden="true" />
                    <span>Issues</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>

          <SidebarGroup>
            <SidebarGroupLabel className="text-[10px] font-medium uppercase tracking-[0.12em] text-sidebar-foreground/55">
              Channels
            </SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu className="gap-0.5">
                <SidebarMenuItem>
                  <SidebarMenuButton
                    isActive={filterChannelId === null}
                    type="button"
                    onClick={() => setFilterChannelId(null)}
                    className="rounded-full text-muted-foreground transition-colors duration-200 ease-out hover:bg-sidebar-accent/50 hover:text-sidebar-accent-foreground data-active:font-medium"
                  >
                    <Layers aria-hidden="true" />
                    <span>All channels</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                {channelsLoading && sortedChannels.length === 0 ? (
                  <SidebarMenuItem>
                    <span className="px-2 py-1.5 text-xs text-muted-foreground">Loading…</span>
                  </SidebarMenuItem>
                ) : (
                  sortedChannels.map((ch) => (
                    <SidebarMenuItem key={ch.id} className="group/menu-item relative">
                      <SidebarMenuButton
                        isActive={filterChannelId === ch.id}
                        type="button"
                        onClick={() => setFilterChannelId(ch.id)}
                        className="rounded-full pr-8 text-muted-foreground transition-colors duration-200 ease-out hover:bg-sidebar-accent/50 hover:text-sidebar-accent-foreground data-active:font-medium"
                      >
                        <Hash aria-hidden="true" />
                        <span className="truncate">{ch.name}</span>
                      </SidebarMenuButton>
                      <SidebarMenuAction
                        showOnHover
                        render={<Link href={`/channels/${ch.id}`} aria-label={`${ch.name} settings`} />}
                      >
                        <Settings className="size-3.5" aria-hidden="true" />
                      </SidebarMenuAction>
                    </SidebarMenuItem>
                  ))
                )}
                <SidebarMenuItem>
                  <SidebarMenuButton
                    type="button"
                    onClick={() => {
                      setNewChannelName("");
                      setCreateChannelOpen(true);
                    }}
                    className="rounded-full text-muted-foreground transition-colors duration-200 ease-out hover:bg-sidebar-accent/50 hover:text-sidebar-accent-foreground"
                  >
                    <Plus aria-hidden="true" />
                    <span>New channel</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>

          <SidebarGroup>
            <SidebarGroupLabel className="text-[10px] font-medium uppercase tracking-[0.12em] text-sidebar-foreground/55">
              {workspace?.name ?? "Workspace"}
            </SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu className="gap-0.5">
                {workspaceNav.map((item) => {
                  const navTestId = `sidebar-nav-${item.href.replace(/^\//, "").replace(/\//g, "-")}`;
                  const isActive =
                    item.href === "/agents"
                      ? pathname === "/agents" ||
                        pathname.startsWith("/agents/") ||
                        pathname === "/teams" ||
                        pathname.startsWith("/teams/")
                      : pathname === item.href || pathname.startsWith(`${item.href}/`);
                  return (
                    <SidebarMenuItem key={item.href}>
                      <SidebarMenuButton
                        isActive={isActive}
                        render={<Link href={item.href} data-testid={navTestId} />}
                        className="rounded-full text-muted-foreground transition-colors duration-200 ease-out hover:bg-sidebar-accent/50 hover:text-sidebar-accent-foreground data-active:font-medium"
                      >
                        <item.icon aria-hidden="true" />
                        <span>{item.label}</span>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  );
                })}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
        <SidebarFooter>
          <ConnectionStatus />
          <div className="flex items-center gap-2.5 px-2 py-1.5">
            <div className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary text-[11px] font-medium text-primary-foreground">
              {user?.name?.charAt(0)?.toUpperCase() ?? user?.email?.charAt(0)?.toUpperCase() ?? "?"}
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium leading-tight">
                {user?.name ?? user?.email ?? "User"}
              </p>
              {user?.name && user?.email && (
                <p className="truncate text-xs text-muted-foreground leading-tight">
                  {user.email}
                </p>
              )}
            </div>
          </div>
        </SidebarFooter>
        <SidebarRail />

        <Dialog open={createChannelOpen} onOpenChange={setCreateChannelOpen}>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>New channel</DialogTitle>
              <DialogDescription>
                Channels isolate issues and membership. Use them like projects.
              </DialogDescription>
            </DialogHeader>
            <Input
              autoFocus
              placeholder="Channel name"
              value={newChannelName}
              onChange={(e) => setNewChannelName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && newChannelName.trim() && !creatingChannel) {
                  e.preventDefault();
                  void (async () => {
                    setCreatingChannel(true);
                    try {
                      await createChannel(newChannelName);
                      toast.success("Channel created");
                      setCreateChannelOpen(false);
                      setNewChannelName("");
                    } catch (err) {
                      toast.error(err instanceof Error ? err.message : "Failed to create channel");
                    } finally {
                      setCreatingChannel(false);
                    }
                  })();
                }
              }}
            />
            <DialogFooter>
              <Button variant="ghost" type="button" onClick={() => setCreateChannelOpen(false)}>
                Cancel
              </Button>
              <Button
                type="button"
                disabled={!newChannelName.trim() || creatingChannel}
                onClick={async () => {
                  setCreatingChannel(true);
                  try {
                    await createChannel(newChannelName);
                    toast.success("Channel created");
                    setCreateChannelOpen(false);
                    setNewChannelName("");
                  } catch (err) {
                    toast.error(err instanceof Error ? err.message : "Failed to create channel");
                  } finally {
                    setCreatingChannel(false);
                  }
                }}
              >
                {creatingChannel ? "Creating…" : "Create"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </Sidebar>
  );
}
