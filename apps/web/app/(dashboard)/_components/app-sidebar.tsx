"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
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
  CircleUser,
  Users,
  Search,
  Server,
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

const primaryNav = [
  { href: "/inbox", label: "Inbox", icon: Inbox },
  { href: "/my-issues", label: "My Issues", icon: CircleUser },
  { href: "/issues", label: "Issues", icon: ListTodo },
];

const workspaceNav = [
  { href: "/agents", label: "Agents", icon: Bot },
  { href: "/teams", label: "Teams", icon: Users },
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

  const unreadCount = useInboxStore((s) => s.unreadCount);

  const logout = async () => {
    clearLoggedInCookie();
    await authLogout();
    useWorkspaceStore.getState().clearWorkspace();
    router.push("/");
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
                          {workspace?.name ?? "Multicode"}
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
                className="flex h-7 flex-1 items-center gap-1.5 rounded-md border border-input bg-background px-2 text-xs text-muted-foreground hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
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
                aria-label="New issue"
                data-testid="new-issue-button"
                className="relative flex h-7 w-7 items-center justify-center rounded-lg bg-background text-foreground shadow-sm dark:shadow-none hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
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
            <SidebarGroupLabel>Personal</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu className="gap-0.5">
                {primaryNav.map((item) => {
                  const isActive = pathname === item.href;
                  return (
                    <SidebarMenuItem key={item.href}>
                      <SidebarMenuButton
                        isActive={isActive}
                        render={<Link href={item.href} data-testid={`sidebar-nav-${item.label.toLowerCase().replace(/\s+/g, "-")}`} />}
                        className="text-muted-foreground hover:not-data-active:bg-sidebar-accent/70 data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground border-l-2 border-l-transparent data-active:border-l-brand"
                      >
                        <item.icon aria-hidden="true" />
                        <span>{item.label}</span>
                        {item.label === "Inbox" && unreadCount > 0 && (
                          <span className="ml-auto text-xs">
                            {unreadCount > 99 ? "99+" : unreadCount}
                          </span>
                        )}
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  );
                })}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>

          <SidebarGroup>
            <SidebarGroupLabel>{workspace?.name ?? "Workspace"}</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu className="gap-0.5">
                {workspaceNav.map((item) => {
                  const isActive = pathname === item.href || pathname.startsWith(item.href + "/");
                  return (
                    <SidebarMenuItem key={item.href}>
                      <SidebarMenuButton
                        isActive={isActive}
                        render={<Link href={item.href} data-testid={`sidebar-nav-${item.label.toLowerCase().replace(/\s+/g, "-")}`} />}
                        className="text-muted-foreground hover:not-data-active:bg-sidebar-accent/70 data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground border-l-2 border-l-transparent data-active:border-l-brand"
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
      </Sidebar>
  );
}
