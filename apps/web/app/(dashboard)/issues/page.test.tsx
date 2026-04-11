import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Issue } from "@/shared/types";
import { useIssueStore } from "@/features/issues/store";

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/issues",
}));

// Mock next/link
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...props
  }: {
    children: React.ReactNode;
    href: string;
    [key: string]: any;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

// Mock workspace feature
vi.mock("@/features/workspace", () => ({
  useActorName: () => ({
    getMemberName: (id: string) => (id === "user-1" ? "Test User" : "Unknown"),
    getAgentName: (id: string) => (id === "agent-1" ? "Claude Agent" : "Unknown Agent"),
    getActorName: (type: string, id: string) =>
      type === "member" ? "Test User" : "Claude Agent",
    getActorInitials: () => "TU",
    getActorAvatarUrl: () => null,
  }),
  useWorkspaceStore: Object.assign(
    (selector?: any) => {
      const state = { workspace: { id: "ws-1", name: "Test", slug: "test" }, agents: [], members: [] };
      return selector ? selector(state) : state;
    },
    { getState: () => ({ workspace: { id: "ws-1", name: "Test", slug: "test" }, agents: [], members: [] }) },
  ),
  WorkspaceAvatar: ({ name }: { name: string }) => <span>{name.charAt(0)}</span>,
}));

// Mock WebSocket context
vi.mock("@/features/realtime", () => ({
  useWSEvent: vi.fn(),
  useWSReconnect: vi.fn(),
  useWS: () => ({ subscribe: vi.fn(() => () => {}), onReconnect: vi.fn(() => () => {}) }),
  WSProvider: ({ children }: { children: React.ReactNode }) => children,
}));

// Mock sonner toast
vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

// Mock api
const mockUpdateIssue = vi.fn();
const mockListIssues = vi.hoisted(() => vi.fn().mockResolvedValue({ issues: [], total: 0 }));

vi.mock("@/shared/api", () => ({
  api: {
    listIssues: (...args: any[]) => mockListIssues(...args),
    updateIssue: (...args: any[]) => mockUpdateIssue(...args),
  },
}));

vi.mock("@/features/issues", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/features/issues")>();
  return {
    ...actual,
    StatusIcon: () => null,
    PriorityIcon: () => null,
    StatusPicker: ({ value, onChange }: any) => (
      <button type="button" onClick={() => onChange?.("todo")}>
        {value || "todo"}
      </button>
    ),
    PriorityPicker: ({ value, onChange }: any) => (
      <button type="button" onClick={() => onChange?.("none")}>
        {value || "none"}
      </button>
    ),
  };
});

// Mock view store
const mockViewState = {
  statusFilters: [] as string[],
  priorityFilters: [] as string[],
  assigneeFilters: [] as { type: string; id: string }[],
  includeNoAssignee: false,
  creatorFilters: [] as { type: string; id: string }[],
  sortBy: "updated_at" as const,
  sortDirection: "desc" as const,
  cardProperties: { priority: true, description: true, assignee: true, dueDate: true },
  toggleStatusFilter: vi.fn(),
  togglePriorityFilter: vi.fn(),
  toggleAssigneeFilter: vi.fn(),
  toggleNoAssignee: vi.fn(),
  toggleCreatorFilter: vi.fn(),
  hideStatus: vi.fn(),
  showStatus: vi.fn(),
  clearFilters: vi.fn(),
  setSortBy: vi.fn(),
  setSortDirection: vi.fn(),
  toggleCardProperty: vi.fn(),
};

vi.mock("@/features/issues/stores/view-store", () => ({
  initFilterWorkspaceSync: vi.fn(),
  useIssueViewStore: Object.assign(
    (selector?: any) => (selector ? selector(mockViewState) : mockViewState),
    { getState: () => mockViewState, setState: vi.fn() },
  ),
  SORT_OPTIONS: [
    { value: "updated_at", label: "Last updated" },
    { value: "created_at", label: "Created" },
    { value: "priority", label: "Priority" },
    { value: "due_date", label: "Due date" },
    { value: "status", label: "Status" },
    { value: "title", label: "Title" },
    { value: "identifier", label: "ID" },
    { value: "position", label: "Manual" },
  ],
  CARD_PROPERTY_OPTIONS: [
    { key: "priority", label: "Priority" },
    { key: "description", label: "Description" },
    { key: "assignee", label: "Assignee" },
    { key: "dueDate", label: "Due date" },
  ],
}));

// Mock view store context (shared components read from context)
vi.mock("@/features/issues/stores/view-store-context", () => ({
  ViewStoreProvider: ({ children }: { children: React.ReactNode }) => children,
  useViewStore: (selector?: any) => (selector ? selector(mockViewState) : mockViewState),
  useViewStoreApi: () => ({ getState: () => mockViewState, setState: vi.fn(), subscribe: vi.fn() }),
}));

// Mock issue config
vi.mock("@/features/issues/config", () => ({
  ALL_STATUSES: ["backlog", "todo", "in_progress", "in_review", "done", "blocked", "cancelled"],
  BOARD_STATUSES: ["backlog", "todo", "in_progress", "in_review", "done", "blocked"],
  STATUS_ORDER: ["backlog", "todo", "in_progress", "in_review", "done", "blocked", "cancelled"],
  STATUS_CONFIG: {
    backlog: { label: "Backlog", iconColor: "text-muted-foreground", hoverBg: "hover:bg-accent", badgeBg: "bg-muted", badgeText: "text-muted-foreground", dividerColor: "", columnBg: "" },
    todo: { label: "Todo", iconColor: "text-muted-foreground", hoverBg: "hover:bg-accent", badgeBg: "bg-muted", badgeText: "text-muted-foreground", dividerColor: "", columnBg: "" },
    in_progress: { label: "In Progress", iconColor: "text-warning", hoverBg: "hover:bg-warning/10", badgeBg: "bg-warning", badgeText: "text-warning-foreground", dividerColor: "", columnBg: "" },
    in_review: { label: "In Review", iconColor: "text-success", hoverBg: "hover:bg-success/10", badgeBg: "bg-success", badgeText: "text-success-foreground", dividerColor: "", columnBg: "" },
    done: { label: "Done", iconColor: "text-info", hoverBg: "hover:bg-info/10", badgeBg: "bg-info", badgeText: "text-info-foreground", dividerColor: "", columnBg: "" },
    blocked: { label: "Blocked", iconColor: "text-destructive", hoverBg: "hover:bg-destructive/10", badgeBg: "bg-destructive", badgeText: "text-destructive-foreground", dividerColor: "", columnBg: "" },
    cancelled: { label: "Cancelled", iconColor: "text-muted-foreground", hoverBg: "hover:bg-accent", badgeBg: "bg-muted", badgeText: "text-muted-foreground", dividerColor: "", columnBg: "" },
  },
  PRIORITY_ORDER: ["urgent", "high", "medium", "low", "none"],
  PRIORITY_CONFIG: {
    urgent: { label: "Urgent", bars: 4, color: "text-destructive", badgeBg: "bg-destructive", badgeText: "text-destructive-foreground" },
    high: { label: "High", bars: 3, color: "text-warning", badgeBg: "bg-warning", badgeText: "text-warning-foreground" },
    medium: { label: "Medium", bars: 2, color: "text-warning", badgeBg: "bg-warning", badgeText: "text-warning-foreground" },
    low: { label: "Low", bars: 1, color: "text-info", badgeBg: "bg-info", badgeText: "text-info-foreground" },
    none: { label: "No priority", bars: 0, color: "text-muted-foreground", badgeBg: "bg-muted", badgeText: "text-muted-foreground" },
  },
}));

// Mock modals
vi.mock("@/features/modals", () => ({
  useModalStore: Object.assign(
    () => ({ open: vi.fn() }),
    { getState: () => ({ open: vi.fn() }) },
  ),
}));

// Mock dnd-kit
vi.mock("@dnd-kit/core", () => ({
  DndContext: ({ children }: any) => children,
  DragOverlay: () => null,
  PointerSensor: class {},
  KeyboardSensor: class {},
  useSensor: () => ({}),
  useSensors: () => [],
  useDroppable: () => ({ setNodeRef: vi.fn(), isOver: false }),
  pointerWithin: vi.fn(),
  closestCenter: vi.fn(),
}));

vi.mock("@dnd-kit/sortable", () => ({
  SortableContext: ({ children }: any) => children,
  verticalListSortingStrategy: {},
  useSortable: () => ({
    attributes: {},
    listeners: {},
    setNodeRef: vi.fn(),
    transform: null,
    transition: null,
    isDragging: false,
  }),
}));

vi.mock("@dnd-kit/utilities", () => ({
  CSS: { Transform: { toString: () => undefined } },
}));

const issueDefaults = {
  parent_issue_id: null,
  position: 0,
  repo_id: null,
  channel_id: "ch-general",
};

const mockIssues: Issue[] = [
  {
    ...issueDefaults,
    id: "issue-1",
    workspace_id: "ws-1",
    number: 1,
    identifier: "TES-1",
    title: "Implement auth",
    description: "Add JWT authentication",
    status: "todo",
    priority: "high",
    assignee_type: "member",
    assignee_id: "user-1",
    creator_type: "member",
    creator_id: "user-1",
    due_date: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  {
    ...issueDefaults,
    id: "issue-2",
    workspace_id: "ws-1",
    number: 2,
    identifier: "TES-2",
    title: "Design landing page",
    description: null,
    status: "in_progress",
    priority: "medium",
    assignee_type: "agent",
    assignee_id: "agent-1",
    creator_type: "member",
    creator_id: "user-1",
    due_date: "2026-02-01T00:00:00Z",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  {
    ...issueDefaults,
    id: "issue-3",
    workspace_id: "ws-1",
    number: 3,
    identifier: "TES-3",
    title: "Write tests",
    description: null,
    status: "backlog",
    priority: "low",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "user-1",
    due_date: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
];

import IssuesPage from "./page";

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe("IssuesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListIssues.mockResolvedValue({ issues: [], total: 0 });
    mockViewState.statusFilters = [];
    mockViewState.priorityFilters = [];
    useIssueStore.setState({
      issues: [],
      loading: false,
      loadingMore: false,
      hasMore: false,
      error: null,
      activeIssueId: null,
    });
  });

  it("shows loading state initially", () => {
    renderWithQuery(<IssuesPage />);
    expect(screen.getAllByRole("generic").some(el => el.getAttribute("data-slot") === "skeleton")).toBe(true);
  });

  it("renders issues in card grid after loading", async () => {
    // issueListOptions makes 2 calls: open_only + closed page. Return issues for open, empty for closed.
    mockListIssues.mockImplementation((params: any) =>
      Promise.resolve(params?.open_only ? { issues: mockIssues, total: mockIssues.length } : { issues: [], total: 0 }),
    );

    renderWithQuery(<IssuesPage />);

    await screen.findByText("Implement auth");
    expect(screen.getByText("Design landing page")).toBeInTheDocument();
    expect(screen.getByText("Write tests")).toBeInTheDocument();
  });

  it("renders card grid with status on tiles", async () => {
    mockListIssues.mockImplementation((params: any) =>
      Promise.resolve(params?.open_only ? { issues: mockIssues, total: mockIssues.length } : { issues: [], total: 0 }),
    );

    renderWithQuery(<IssuesPage />);

    await screen.findByTestId("issues-card-grid");
    expect(screen.getByText("Todo")).toBeInTheDocument();
    expect(screen.getByText("In Progress")).toBeInTheDocument();
    expect(screen.getByText("Backlog")).toBeInTheDocument();
  });

  it("shows workspace breadcrumb", async () => {
    renderWithQuery(<IssuesPage />);

    await screen.findByText("Issues");
  });

  it("shows scope buttons", async () => {
    renderWithQuery(<IssuesPage />);

    await screen.findByText("All");
    expect(screen.getByText("Members")).toBeInTheDocument();
    expect(screen.getByText("Agents")).toBeInTheDocument();
  });

  it("shows filter and display icon buttons", async () => {
    mockListIssues.mockImplementation((params: any) =>
      Promise.resolve(params?.open_only ? { issues: mockIssues, total: mockIssues.length } : { issues: [], total: 0 }),
    );

    renderWithQuery(<IssuesPage />);

    await screen.findByText("Implement auth");
    const buttons = screen.getAllByRole("button");
    expect(buttons.length).toBeGreaterThan(0);
  });

  it("shows create-first empty state when no issues exist", async () => {
    renderWithQuery(<IssuesPage />);

    expect(await screen.findByText("Create your first task")).toBeInTheDocument();
  });
});
