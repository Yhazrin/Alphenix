import { test, expect } from "@playwright/test";
import { loginAsDefault, createTestApi } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Task Report — inline panel via agent detail (P0-2)", () => {
  let api: TestApiClient;
  let agentId: string;
  let issueId: string;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    await loginAsDefault(page);

    const agent = await api.createAgent("E2E Task Agent");
    agentId = agent.id;

    const issue = await api.createIssue("E2E Task Issue " + Date.now());
    issueId = issue.id;
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("agent Tasks tab shows task queue", async ({ page }) => {
    // Create a completed task
    await api.createTaskWithReport(agentId, issueId, {
      status: "completed",
      result: "Task completed successfully.",
    });

    await page.goto(`/agents/${agentId}`);
    await page.waitForURL(`**/agents/${agentId}`);

    // Click Tasks tab
    await page.locator('[role="tab"]:has-text("Tasks")').click();
    await expect(page.locator("text=Task Queue")).toBeVisible({
      timeout: 10000,
    });
  });

  test("View report opens TaskReportPanel inline", async ({ page }) => {
    // Create a completed task with result
    const task = await api.createTaskWithReport(agentId, issueId, {
      status: "completed",
      result: "Task completed successfully.",
    });

    await page.goto(`/agents/${agentId}`);
    await page.waitForURL(`**/agents/${agentId}`);

    // Click Tasks tab
    await page.locator('[role="tab"]:has-text("Tasks")').click();
    await expect(page.locator("text=Task Queue")).toBeVisible({
      timeout: 10000,
    });

    // Click View report button
    const viewReport = page.locator('[aria-label="View report"]').first();
    await expect(viewReport).toBeVisible({ timeout: 5000 });
    await viewReport.click();

    // TaskReportPanel should appear
    await expect(
      page.getByRole("heading", { name: "Task Report" }),
    ).toBeVisible({ timeout: 10000 });
  });

  test("Close button returns to Task Queue", async ({ page }) => {
    await api.createTaskWithReport(agentId, issueId, {
      status: "completed",
      result: "Done.",
    });

    await page.goto(`/agents/${agentId}`);
    await page.waitForURL(`**/agents/${agentId}`);
    await page.locator('[role="tab"]:has-text("Tasks")').click();
    await expect(page.locator("text=Task Queue")).toBeVisible({
      timeout: 10000,
    });

    // Open report
    const viewReport = page.locator('[aria-label="View report"]').first();
    await viewReport.click();
    await expect(
      page.getByRole("heading", { name: "Task Report" }),
    ).toBeVisible({ timeout: 10000 });

    // Close it
    await page.locator("button:has-text('Close')").click();

    // Should be back to Task Queue
    await expect(page.locator("text=Task Queue")).toBeVisible({
      timeout: 5000,
    });
  });

  test("completed task shows result in Output tab", async ({ page }) => {
    await api.createTaskWithReport(agentId, issueId, {
      status: "completed",
      result: "Build succeeded with 0 errors.",
    });

    await page.goto(`/agents/${agentId}`);
    await page.waitForURL(`**/agents/${agentId}`);
    await page.locator('[role="tab"]:has-text("Tasks")').click();
    await page.locator('[aria-label="View report"]').first().click();
    await expect(
      page.getByRole("heading", { name: "Task Report" }),
    ).toBeVisible({ timeout: 10000 });

    // Switch to Output tab
    await page.locator('[role="tab"]:has-text("Output")').click();
    await expect(page.locator("pre")).toBeVisible({ timeout: 5000 });
  });

  test("failed task shows error output", async ({ page }) => {
    await api.createTaskWithReport(agentId, issueId, {
      status: "failed",
      error: "Connection timeout after 30s",
    });

    await page.goto(`/agents/${agentId}`);
    await page.waitForURL(`**/agents/${agentId}`);
    await page.locator('[role="tab"]:has-text("Tasks")').click();
    await page.locator('[aria-label="View report"]').first().click();
    await expect(
      page.getByRole("heading", { name: "Task Report" }),
    ).toBeVisible({ timeout: 10000 });

    // Switch to Output tab — should show error
    await page.locator('[role="tab"]:has-text("Output")').click();
    await expect(page.locator("pre")).toBeVisible({ timeout: 5000 });
  });

  test("Summary tab shows status and issue info", async ({ page }) => {
    await api.createTaskWithReport(agentId, issueId, {
      status: "completed",
      result: "Done.",
    });

    await page.goto(`/agents/${agentId}`);
    await page.waitForURL(`**/agents/${agentId}`);
    await page.locator('[role="tab"]:has-text("Tasks")').click();
    await page.locator('[aria-label="View report"]').first().click();
    await expect(
      page.getByRole("heading", { name: "Task Report" }),
    ).toBeVisible({ timeout: 10000 });

    // Summary tab should be active by default — check for status
    await expect(page.locator("text=completed")).toBeVisible({
      timeout: 5000,
    });
  });

  test("Timeline tab can be navigated to", async ({ page }) => {
    await api.createTaskWithReport(agentId, issueId, {
      status: "completed",
    });

    await page.goto(`/agents/${agentId}`);
    await page.waitForURL(`**/agents/${agentId}`);
    await page.locator('[role="tab"]:has-text("Tasks")').click();
    await page.locator('[aria-label="View report"]').first().click();
    await expect(
      page.getByRole("heading", { name: "Task Report" }),
    ).toBeVisible({ timeout: 10000 });

    // Switch to Timeline tab
    await page.locator('[role="tab"]:has-text("Timeline")').click();

    // Either shows events or empty state
    const hasEvents = await page
      .locator("text=/\\d+ events?/")
      .isVisible()
      .catch(() => false);
    const hasEmpty = await page
      .locator("text=No timeline events yet")
      .isVisible()
      .catch(() => false);

    expect(hasEvents || hasEmpty).toBeTruthy();
  });

  test("tab navigation Summary → Timeline → Output → Summary", async ({
    page,
  }) => {
    await api.createTaskWithReport(agentId, issueId, {
      status: "completed",
      result: "Nav test result.",
    });

    await page.goto(`/agents/${agentId}`);
    await page.waitForURL(`**/agents/${agentId}`);
    await page.locator('[role="tab"]:has-text("Tasks")').click();
    await page.locator('[aria-label="View report"]').first().click();
    await expect(
      page.getByRole("heading", { name: "Task Report" }),
    ).toBeVisible({ timeout: 10000 });

    // Summary → Timeline
    await page.locator('[role="tab"]:has-text("Timeline")').click();
    await expect(page.locator('[role="tab"][data-state="active"]')).toHaveText(
      /Timeline/,
    );

    // Timeline → Output
    await page.locator('[role="tab"]:has-text("Output")').click();
    await expect(page.locator('[role="tab"][data-state="active"]')).toHaveText(
      /Output/,
    );

    // Output → Summary
    await page.locator('[role="tab"]:has-text("Summary")').click();
    await expect(page.locator('[role="tab"][data-state="active"]')).toHaveText(
      /Summary/,
    );
  });

  test("issue title is displayed in report", async ({ page }) => {
    const issueTitle = "E2E Report Issue " + Date.now();
    const issue = await api.createIssue(issueTitle);
    await api.createTaskWithReport(agentId, issue.id, {
      status: "completed",
    });

    await page.goto(`/agents/${agentId}`);
    await page.waitForURL(`**/agents/${agentId}`);
    await page.locator('[role="tab"]:has-text("Tasks")').click();
    await page.locator('[aria-label="View report"]').first().click();
    await expect(
      page.getByRole("heading", { name: "Task Report" }),
    ).toBeVisible({ timeout: 10000 });

    // Issue title should appear in summary
    await expect(page.locator(`text=${issueTitle}`)).toBeVisible({
      timeout: 5000,
    });
  });

  test("task queue shows empty state when no tasks", async ({ page }) => {
    // No tasks created — just navigate
    await page.goto(`/agents/${agentId}`);
    await page.waitForURL(`**/agents/${agentId}`);
    await page.locator('[role="tab"]:has-text("Tasks")').click();

    // Should show empty state
    await expect(page.locator("text=No tasks in queue")).toBeVisible({
      timeout: 10000,
    });
  });
});
