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

  test("issues page loads with card grid", async ({ page }) => {
    await api.createIssue("E2E Card Grid " + Date.now());
    await page.reload();

    await expect(page.locator("text=Issues").first()).toBeVisible();
    await expect(page.getByTestId("issues-card-grid")).toBeVisible();
  });

  test("issue titles appear as card tiles", async ({ page }) => {
    const title = "E2E Tile " + Date.now();
    await api.createIssue(title);
    await page.reload();

    await expect(page.locator("text=Issues").first()).toBeVisible();
    await expect(page.getByText(title).first()).toBeVisible({ timeout: 10000 });
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