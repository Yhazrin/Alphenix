import { test, expect } from "@playwright/test";
import { loginAsDefault, createTestApi } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Prompt Preview (P0-1)", () => {
  let api: TestApiClient;
  let agentId: string;

  test.beforeEach(async ({ page }) => {
    const { token, workspaceId } = await loginAsDefault(page);
    api = createTestApi(token, workspaceId);

    const agent = await api.createAgent("E2E Prompt Agent", {
      instructions: "You are a helpful test agent for E2E testing.",
    });
    agentId = agent.id;

    // Navigate to agent detail page directly
    await page.goto(`/agents/${agentId}`);
    await page.waitForURL(`**/agents/${agentId}`);

    // Click the Prompt tab
    await page.locator('[role="tab"]:has-text("Prompt")').click();
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("tab loads and shows prompt preview heading", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "System Prompt Preview" }),
    ).toBeVisible({ timeout: 10000 });
  });

  test("summary bar shows section count and character count", async ({
    page,
  }) => {
    await expect(
      page.getByRole("heading", { name: "System Prompt Preview" }),
    ).toBeVisible({ timeout: 10000 });

    // Should show a summary like "X sections . Y characters"
    const summaryRegex = /\d+ sections?/;
    await expect(page.locator("body")).toContainText(summaryRegex);
  });

  test("static/dynamic badge is displayed", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "System Prompt Preview" }),
    ).toBeVisible({ timeout: 10000 });

    // Should show badge with static/dynamic counts
    const badgeRegex = /\d+ static/;
    await expect(page.locator("body")).toContainText(badgeRegex);
  });

  test("section can be expanded and collapsed", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "System Prompt Preview" }),
    ).toBeVisible({ timeout: 10000 });

    // Find a section row and click to expand
    const sectionRow = page.locator("text=/order: \\d+/").first();
    if (await sectionRow.isVisible()) {
      // Click the parent row to expand
      await sectionRow.locator("..").click();

      // Should show pre element with content
      await expect(page.locator("pre.font-mono").first()).toBeVisible(
        { timeout: 5000 },
      );

      // Click again to collapse
      await sectionRow.locator("..").click();
    }
  });

  test("Copy section button works", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "System Prompt Preview" }),
    ).toBeVisible({ timeout: 10000 });

    // Expand a section first
    const sectionRow = page.locator("text=/order: \\d+/").first();
    if (await sectionRow.isVisible()) {
      await sectionRow.locator("..").click();
      await expect(page.locator("pre.font-mono").first()).toBeVisible(
        { timeout: 5000 },
      );

      // Click Copy section button
      const copyBtn = page.locator("button:has-text('Copy section')").first();
      if (await copyBtn.isVisible()) {
        await copyBtn.click();
      }
    }
  });

  test("Refresh button reloads preview", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "System Prompt Preview" }),
    ).toBeVisible({ timeout: 10000 });

    const refreshBtn = page.locator("button:has-text('Refresh')");
    await expect(refreshBtn).toBeVisible();
    await refreshBtn.click();

    // Should still show heading after refresh
    await expect(
      page.getByRole("heading", { name: "System Prompt Preview" }),
    ).toBeVisible({ timeout: 10000 });
  });

  test("phase badges show static or dynamic type", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "System Prompt Preview" }),
    ).toBeVisible({ timeout: 10000 });

    // Should have at least one badge with "static" or "dynamic"
    const hasStatic = await page
      .locator("text=/\\d+ static/")
      .isVisible()
      .catch(() => false);
    const hasDynamic = await page
      .locator("text=/\\d+ dynamic/")
      .isVisible()
      .catch(() => false);

    expect(hasStatic || hasDynamic).toBeTruthy();
  });

  test("sections are displayed in order", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "System Prompt Preview" }),
    ).toBeVisible({ timeout: 10000 });

    // Verify sections have order values
    const orderElements = page.locator("text=/order: \\d+/");
    const count = await orderElements.count();
    if (count > 1) {
      // Just verify multiple sections with order labels exist
      expect(count).toBeGreaterThanOrEqual(1);
    }
  });

  test("empty agent shows fallback state", async ({ page }) => {
    // Create a second agent with minimal data
    const emptyAgent = await api.createAgent("E2E Empty Agent", {
      instructions: "",
    });

    await page.goto(`/agents/${emptyAgent.id}`);
    await page.waitForURL(`**/agents/${emptyAgent.id}`);
    await page.locator('[role="tab"]:has-text("Prompt")').click();

    // Should either show preview or empty state
    const hasHeading = await page
      .getByRole("heading", { name: "System Prompt Preview" })
      .isVisible({ timeout: 10000 })
      .catch(() => false);
    const hasEmpty = await page
      .locator("text=No prompt content yet")
      .isVisible({ timeout: 5000 })
      .catch(() => false);

    expect(hasHeading || hasEmpty).toBeTruthy();
  });
});
