import { test, expect } from "@playwright/test";
import { loginAsDefault, createTestApi } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Issues", () => {
  let api: TestApiClient;

  test.beforeEach(async ({ page }) => {
    const { token, workspaceId } = await loginAsDefault(page);
    api = createTestApi(token, workspaceId);
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("issues page loads with board view", async ({ page }) => {
    // Create a test issue so board columns render (empty board shows no columns)
    await api.createIssue("E2E Board Fixture " + Date.now());
    await page.reload();

    await expect(page.locator("text=Issues").first()).toBeVisible();

    // Board columns use div[role="region"] with aria-label containing column name
    await expect(page.locator('[role="region"][aria-label*="Backlog column"]')).toBeVisible();
    await expect(page.locator('[role="region"][aria-label*="Todo column"]')).toBeVisible();
    await expect(page.locator('[role="region"][aria-label*="In Progress column"]')).toBeVisible();
  });

  test("can switch between board and list view", async ({ page }) => {
    // Create a test issue so board columns render
    await api.createIssue("E2E View Fixture " + Date.now());
    await page.reload();

    await expect(page.locator("text=Issues").first()).toBeVisible();

    // Switch to list view — open dropdown then click List option
    await page.getByTestId("issues-view-toggle").click();
    await page.locator('[role="menuitem"]', { hasText: "List" }).click();
    await expect(page.locator("text=Issues").first()).toBeVisible();

    // Switch back to board view
    await page.getByTestId("issues-view-toggle").click();
    await page.locator('[role="menuitem"]', { hasText: "Board" }).click();
    await expect(page.locator('[role="region"][aria-label*="Backlog column"]')).toBeVisible();
  });

  test("can create a new issue", async ({ page }) => {
    // New Issue button
    await page.getByTestId("new-issue-button").click();

    const title = "E2E Created " + Date.now();
    // Title uses Tiptap contenteditable, not <input>
    const titleEditor = page.getByRole("textbox", { name: "Issue title" });
    await titleEditor.click();
    await page.keyboard.type(title);
    await page.locator("button", { hasText: "Create Issue" }).click();

    // New issue should appear on the page
    await expect(page.locator(`text=${title}`).first()).toBeVisible({
      timeout: 10000,
    });
  });

  test("can navigate to issue detail page", async ({ page }) => {
    // Create a known issue via API so the test controls its own fixture
    const issue = await api.createIssue("E2E Detail Test " + Date.now());

    // Reload to see the new issue
    await page.reload();
    await expect(page.locator("text=Issues").first()).toBeVisible();

    // Navigate to the issue detail
    const issueLink = page.locator(`a[href="/issues/${issue.id}"]`);
    await expect(issueLink).toBeVisible({ timeout: 5000 });
    await issueLink.click();

    await page.waitForURL(/\/issues\/[\w-]+/);

    // Should show Properties panel
    await expect(page.locator("text=Properties")).toBeVisible();
    // Should show breadcrumb link back to Issues
    await expect(
      page.locator("a", { hasText: "Issues" }).first(),
    ).toBeVisible();
  });

  test("can cancel issue creation", async ({ page }) => {
    await page.getByTestId("new-issue-button").click();

    // Title editor should be visible in the modal
    const titleEditor = page.getByRole("textbox", { name: "Issue title" });
    await expect(titleEditor).toBeVisible();

    // Close the modal via the X button (no "Cancel" button exists)
    await page.locator('[aria-label="Close"]').click();

    await expect(titleEditor).not.toBeVisible();
    await expect(page.getByTestId("new-issue-button")).toBeVisible();
  });
});
