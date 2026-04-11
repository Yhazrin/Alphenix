import type { Issue } from "@/shared/types";
import { PRIORITY_ORDER, STATUS_ORDER } from "@/features/issues/config";
import type { SortField, SortDirection } from "@/features/issues/stores/view-store";

const PRIORITY_RANK: Record<string, number> = Object.fromEntries(
  PRIORITY_ORDER.map((p, i) => [p, i])
);

const STATUS_RANK: Record<string, number> = Object.fromEntries(
  STATUS_ORDER.map((s, i) => [s, i])
);

export function sortIssues(
  issues: Issue[],
  field: SortField,
  direction: SortDirection
): Issue[] {
  const dir = direction === "desc" ? -1 : 1;
  const sorted = [...issues].sort((a, b) => {
    switch (field) {
      case "priority":
        return dir * (
          (PRIORITY_RANK[a.priority] ?? 99) -
          (PRIORITY_RANK[b.priority] ?? 99)
        );
      case "status":
        return dir * (
          (STATUS_RANK[a.status] ?? 99) - (STATUS_RANK[b.status] ?? 99)
        );
      case "identifier":
        return dir * a.identifier.localeCompare(b.identifier);
      case "due_date": {
        if (!a.due_date && !b.due_date) return 0;
        if (!a.due_date) return 1;
        if (!b.due_date) return -1;
        const aDue = new Date(a.due_date).getTime();
        const bDue = new Date(b.due_date).getTime();
        return dir * ((isNaN(aDue) ? 0 : aDue) - (isNaN(bDue) ? 0 : bDue));
      }
      case "created_at": {
        const aCreated = new Date(a.created_at).getTime();
        const bCreated = new Date(b.created_at).getTime();
        return dir * ((isNaN(aCreated) ? 0 : aCreated) - (isNaN(bCreated) ? 0 : bCreated));
      }
      case "updated_at": {
        const aU = new Date(a.updated_at).getTime();
        const bU = new Date(b.updated_at).getTime();
        return dir * ((isNaN(aU) ? 0 : aU) - (isNaN(bU) ? 0 : bU));
      }
      case "title":
        return dir * a.title.localeCompare(b.title);
      case "position":
      default:
        return dir * (a.position - b.position);
    }
  });
  return sorted;
}
