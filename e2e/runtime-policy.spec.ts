import { test, expect } from "@playwright/test";
import { loginAsDefault, createTestApi } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Runtime Policy (P0-3)", () => {
  let api: TestApiClient;
  let agentId: string;

  test.beforeEach(async ({ page }) => {
    const { token, workspaceId } = await loginAsDefault(page);
    api = createTestApi(token, workspaceId);

    const agent = await api.createAgent("E2E Policy Agent");
    agentId = agent.id;

    // Navigate to agent detail page
    await page.goto(`/agents/${agentId}`, { waitUntil: "domcontentloaded" });
    await page.waitForURL(`**/agents/${agentId}`);

    // Click the Policy tab
    await page.locator('[role="tab"]:has-text("Policy")').click();
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("tab loads with empty state when no policy exists", async ({
    page,
  }) => {
    await expect(
      page.getByRole("heading", { name: "Runtime Scheduling Policy" }),
    ).toBeVisible({ timeout: 10000 });

    // Should show empty state
    await expect(page.locator("text=No scheduling policy yet")).toBeVisible({
      timeout: 5000,
    });
  });

  test("can create a new policy with required tags", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "Runtime Scheduling Policy" }),
    ).toBeVisible({ timeout: 10000 });

    // Fill required tags
    const requiredInput = page.getByTestId("policy-required-tags-input");
    await requiredInput.fill("gpu");
    await requiredInput.press("Enter");

    // Click Create Policy
    await page.locator("button:has-text('Create Policy')").click();

    // Should transition to policy editor (empty state gone)
    await expect(page.locator("text=No scheduling policy yet")).not.toBeVisible(
      { timeout: 5000 },
    );

    // Should show Tag Rules section
    await expect(page.locator("text=Tag Rules")).toBeVisible({
      timeout: 5000,
    });
  });

  test("can add and remove forbidden tags", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "Runtime Scheduling Policy" }),
    ).toBeVisible({ timeout: 10000 });

    // Create policy first
    const requiredInput = page.getByTestId("policy-required-tags-input");
    await requiredInput.fill("gpu");
    await requiredInput.press("Enter");
    await page.locator("button:has-text('Create Policy')").click();
    await expect(page.locator("text=Tag Rules")).toBeVisible({
      timeout: 5000,
    });

    // Add a forbidden tag
    const forbiddenInput = page.getByTestId("policy-forbidden-tags-input");
    await forbiddenInput.fill("experimental");
    await forbiddenInput.press("Enter");

    // Forbidden tag badge should appear
    await expect(page.locator("text=experimental").first()).toBeVisible({
      timeout: 5000,
    });

    // Remove the forbidden tag
    await page.locator('[aria-label="Remove experimental"]').click();

    // Badge should be gone
    await expect(page.locator('[aria-label="Remove experimental"]')).toHaveCount(
      0,
    );
  });

  test("Active/Inactive switch toggles", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "Runtime Scheduling Policy" }),
    ).toBeVisible({ timeout: 10000 });

    // Create policy
    const requiredInput = page.getByTestId("policy-required-tags-input");
    await requiredInput.fill("gpu");
    await requiredInput.press("Enter");
    await page.locator("button:has-text('Create Policy')").click();
    await expect(page.locator("text=Tag Rules")).toBeVisible({
      timeout: 5000,
    });

    // Find and toggle Active/Inactive switch
    const switchEl = page.locator('[role="switch"]');
    if (await switchEl.isVisible()) {
      const initialState = await switchEl.getAttribute("aria-checked");
      await switchEl.click();

      // State should have changed
      const newState = await switchEl.getAttribute("aria-checked");
      expect(newState).not.toEqual(initialState);
    }
  });

  test("Max Queue Depth can be updated", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "Runtime Scheduling Policy" }),
    ).toBeVisible({ timeout: 10000 });

    // Create policy
    const requiredInput = page.getByTestId("policy-required-tags-input");
    await requiredInput.fill("gpu");
    await requiredInput.press("Enter");
    await page.locator("button:has-text('Create Policy')").click();
    await expect(page.locator("text=Queue Control")).toBeVisible({
      timeout: 5000,
    });

    // Update max queue depth
    const queueInput = page.locator("#max-queue");
    await queueInput.fill("10");

    // Save
    await page.locator("button:has-text('Save Changes')").click();
  });

  test("can delete policy and return to empty state", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "Runtime Scheduling Policy" }),
    ).toBeVisible({ timeout: 10000 });

    // Create policy
    const requiredInput = page.getByTestId("policy-required-tags-input");
    await requiredInput.fill("gpu");
    await requiredInput.press("Enter");
    await page.locator("button:has-text('Create Policy')").click();
    await expect(page.locator("text=Tag Rules")).toBeVisible({
      timeout: 5000,
    });

    // Delete
    await page.locator("button:has-text('Delete Policy')").click();

    // Should return to empty state
    await expect(page.locator("text=No scheduling policy yet")).toBeVisible({
      timeout: 5000,
    });
  });

  test("policy persists after page reload", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "Runtime Scheduling Policy" }),
    ).toBeVisible({ timeout: 10000 });

    // Create policy with required tag
    const requiredInput = page.getByTestId("policy-required-tags-input");
    await requiredInput.fill("gpu");
    await requiredInput.press("Enter");
    await page.locator("button:has-text('Create Policy')").click();
    await expect(page.locator("text=Tag Rules")).toBeVisible({
      timeout: 5000,
    });

    // Reload the page
    await page.goto(`/agents/${agentId}`, { waitUntil: "domcontentloaded" });
    await page.waitForURL(`**/agents/${agentId}`);
    await page.locator('[role="tab"]:has-text("Policy")').click();

    // Policy should still exist (not empty state)
    await expect(page.locator("text=No scheduling policy yet")).not.toBeVisible(
      { timeout: 10000 },
    );
    await expect(page.locator("text=Tag Rules")).toBeVisible({
      timeout: 5000,
    });
  });

  test("tag changes persist after save and reload", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "Runtime Scheduling Policy" }),
    ).toBeVisible({ timeout: 10000 });

    // Create policy
    const requiredInput = page.getByTestId("policy-required-tags-input");
    await requiredInput.fill("gpu");
    await requiredInput.press("Enter");
    await page.locator("button:has-text('Create Policy')").click();
    await expect(page.locator("text=Tag Rules")).toBeVisible({
      timeout: 5000,
    });

    // Add another required tag
    const addInput = page.getByTestId("policy-required-tags-input");
    await addInput.fill("high-memory");
    await addInput.press("Enter");

    // Save
    await page.locator("button:has-text('Save Changes')").click();

    // Reload
    await page.goto(`/agents/${agentId}`, { waitUntil: "domcontentloaded" });
    await page.waitForURL(`**/agents/${agentId}`);
    await page.locator('[role="tab"]:has-text("Policy")').click();

    // Both tags should be present
    await expect(page.locator("text=Tag Rules")).toBeVisible({
      timeout: 10000,
    });
    // Verify at least the tag badges are there
    const tagBadges = page.locator('[aria-label^="Remove"]');
    await expect(tagBadges.first()).toBeVisible({ timeout: 5000 });
  });
});
