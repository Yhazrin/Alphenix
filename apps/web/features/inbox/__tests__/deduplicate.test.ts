import { describe, it, expect } from "vitest";
import { deduplicateInboxItems } from "../store";
import type { InboxItem } from "@/shared/types";

function makeItem(overrides: Partial<InboxItem> = {}): InboxItem {
  return {
    id: crypto.randomUUID(),
    workspace_id: "ws-1",
    recipient_type: "member",
    recipient_id: "user-1",
    actor_type: null,
    actor_id: null,
    type: "issue_assigned",
    severity: "info",
    issue_id: null,
    title: "Test",
    body: null,
    issue_status: null,
    read: false,
    archived: false,
    created_at: new Date().toISOString(),
    details: null,
    ...overrides,
  };
}

describe("deduplicateInboxItems", () => {
  it("returns empty array for empty input", () => {
    expect(deduplicateInboxItems([])).toEqual([]);
  });

  it("filters out archived items", () => {
    const items = [
      makeItem({ id: "1", archived: true, created_at: "2026-01-01T00:00:00Z" }),
      makeItem({ id: "2", archived: false, created_at: "2026-01-02T00:00:00Z" }),
    ];
    const result = deduplicateInboxItems(items);
    expect(result).toHaveLength(1);
    expect(result[0]!.id).toBe("2");
  });

  it("keeps only the latest item per issue_id", () => {
    const items = [
      makeItem({ id: "a", issue_id: "issue-1", created_at: "2026-01-01T00:00:00Z" }),
      makeItem({ id: "b", issue_id: "issue-1", created_at: "2026-01-03T00:00:00Z" }),
      makeItem({ id: "c", issue_id: "issue-1", created_at: "2026-01-02T00:00:00Z" }),
    ];
    const result = deduplicateInboxItems(items);
    expect(result).toHaveLength(1);
    expect(result[0]!.id).toBe("b");
  });

  it("groups by id when issue_id is null", () => {
    const items = [
      makeItem({ id: "x", issue_id: null, created_at: "2026-01-01T00:00:00Z" }),
      makeItem({ id: "y", issue_id: null, created_at: "2026-01-02T00:00:00Z" }),
    ];
    const result = deduplicateInboxItems(items);
    expect(result).toHaveLength(2);
    // sorted desc by created_at
    expect(result[0]!.id).toBe("y");
    expect(result[1]!.id).toBe("x");
  });

  it("preserves distinct issues", () => {
    const items = [
      makeItem({ id: "a", issue_id: "issue-1", created_at: "2026-01-01T00:00:00Z" }),
      makeItem({ id: "b", issue_id: "issue-2", created_at: "2026-01-02T00:00:00Z" }),
    ];
    const result = deduplicateInboxItems(items);
    expect(result).toHaveLength(2);
  });

  it("sorts final result by created_at descending", () => {
    const items = [
      makeItem({ id: "a", issue_id: "issue-1", created_at: "2026-01-01T00:00:00Z" }),
      makeItem({ id: "b", issue_id: "issue-2", created_at: "2026-01-03T00:00:00Z" }),
      makeItem({ id: "c", issue_id: "issue-3", created_at: "2026-01-02T00:00:00Z" }),
    ];
    const result = deduplicateInboxItems(items);
    expect(result.map((r) => r.id)).toEqual(["b", "c", "a"]);
  });

  it("excludes archived items from dedup even if they are the latest", () => {
    const items = [
      makeItem({ id: "old", issue_id: "issue-1", archived: false, created_at: "2026-01-01T00:00:00Z" }),
      makeItem({ id: "new-but-archived", issue_id: "issue-1", archived: true, created_at: "2026-01-05T00:00:00Z" }),
    ];
    const result = deduplicateInboxItems(items);
    expect(result).toHaveLength(1);
    expect(result[0]!.id).toBe("old");
  });
});
