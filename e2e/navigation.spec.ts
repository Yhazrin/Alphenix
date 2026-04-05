import { test, expect } from "@playwright/test";
import { loginAsDefault } from "./helpers";

test.describe("Navigation", () => {
  test.beforeEach(async ({ page }) => {
    await loginAsDefault(page);
  });

  test("sidebar navigation works", async ({ page }) => {
    // Click Inbox — sidebar uses data-sidebar="sidebar" on the outer element
    await page.locator('[data-sidebar="sidebar"] a', { hasText: "Inbox" }).click();
    await page.waitForURL("**/inbox");
    await expect(page).toHaveURL(/\/inbox/);

    // Click Agents
    await page.locator('[data-sidebar="sidebar"] a', { hasText: "Agents" }).click();
    await page.waitForURL("**/agents");
    await expect(page).toHaveURL(/\/agents/);

    // Click Issues (use href to avoid matching "My Issues")
    await page.locator('[data-sidebar="sidebar"] a[href="/issues"]').click();
    await page.waitForURL("**/issues");
    await expect(page).toHaveURL(/\/issues/);
  });

  test("settings page loads via sidebar", async ({ page }) => {
    // Settings is a sidebar nav link, not in the workspace dropdown
    await page.locator('[data-sidebar="sidebar"] a', { hasText: "Settings" }).click();
    await page.waitForURL("**/settings");

    await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
    // Default tab is Profile (My Account section)
    await expect(page.locator("text=Profile").first()).toBeVisible();
  });

  test("agents page shows agent list", async ({ page }) => {
    await page.locator('[data-sidebar="sidebar"] a', { hasText: "Agents" }).click();
    await page.waitForURL("**/agents");

    // Should show "Agents" heading
    await expect(page.locator("text=Agents").first()).toBeVisible();
  });
});
