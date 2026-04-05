import type { Issue } from "@/shared/types";

export type AgentIssueStatus = "working" | "failed" | "blocked" | "idle";

export function getAgentIssueStatus(issue: Issue): AgentIssueStatus {
  if (issue.status === "in_progress") return "working";
  // TODO: When Issue type includes latest_task_status, check for failed/blocked here.
  // e.g. if ((issue as any).latest_task_status === "failed") return "failed";
  return "idle";
}

const STATUS_CONFIG: Record<Exclude<AgentIssueStatus, "idle">, { dot: string; ping: boolean; title: string }> = {
  working: {
    dot: "bg-info",
    ping: true,
    title: "Agent is working",
  },
  failed: {
    dot: "bg-destructive",
    ping: false,
    title: "Agent task failed",
  },
  blocked: {
    dot: "bg-warning",
    ping: false,
    title: "Agent is blocked — needs input",
  },
};

export function AgentStatusDot({ status }: { status: AgentIssueStatus }) {
  if (status === "idle") return null;

  const config = STATUS_CONFIG[status];

  return (
    <span className="relative flex h-2 w-2" title={config.title}>
      {config.ping && (
        <span
          className={`absolute inline-flex h-full w-full animate-ping rounded-full ${config.dot} opacity-75`}
        />
      )}
      <span className={`relative inline-flex h-2 w-2 rounded-full ${config.dot}`} />
    </span>
  );
}
