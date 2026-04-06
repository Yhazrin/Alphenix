import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Comments", () => {
  let api: TestApiClient;

  test.beforeEach(async ({ page }) => {
    const { token, workspaceId } = await loginAsDefault(page);
    api = createTestApi(token, workspaceId);
    await api.createIssue("E2E Comment Test " + Date.now());
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("can add a comment on an issue", async ({ page }) => {
    // Wait for issues to load and click first one
    const issueLink = page.locator('a[href^="/issues/"]').first();
    await expect(issueLink).toBeVisible({ timeout: 5000 });
    await issueLink.click();
    await page.waitForURL(/\/issues\/[\w-]+/);

    // Wait for issue detail to load
    await expect(page.locator("text=Properties")).toBeVisible();

    // Type a comment — uses Tiptap contenteditable, not <input>
    const commentText = "E2E comment " + Date.now();
    const commentInput = page.locator(
      '[data-placeholder="Leave a comment..."]',
    );
    await commentInput.click();
    await page.keyboard.type(commentText);

    // Submit the comment
    await page.getByTestId("comment-submit-button").click();

    // Comment should appear in the activity section
    await expect(page.locator(`text=${commentText}`)).toBeVisible({
      timeout: 5000,
    });
  });

  test("comment submit button is disabled when empty", async ({ page }) => {
    const issueLink = page.locator('a[href^="/issues/"]').first();
    await expect(issueLink).toBeVisible({ timeout: 5000 });
    await issueLink.click();
    await page.waitForURL(/\/issues\/[\w-]+/);

    await expect(page.locator("text=Properties")).toBeVisible();

    // Submit button should be disabled when input is empty
    const submitBtn = page.getByTestId("comment-submit-button");
    await expect(submitBtn).toBeDisabled();
  });
});
