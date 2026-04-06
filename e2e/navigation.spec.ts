import { test, expect } from "@playwright/test";
import { loginAsDefault } from "./helpers";

test.describe("Navigation", () => {
  test.beforeEach(async ({ page }) => {
    await loginAsDefault(page);
  });

  test("sidebar navigation works", async ({ page }) => {
    // Click Inbox
    await page.getByTestId("sidebar-nav-inbox").click();
    await page.waitForURL("**/inbox");
    await expect(page).toHaveURL(/\/inbox/);

    // Click Agents
    await page.getByTestId("sidebar-nav-agents").click();
    await page.waitForURL("**/agents");
    await expect(page).toHaveURL(/\/agents/);

    // Click Issues
    await page.getByTestId("sidebar-nav-issues").click();
    await page.waitForURL("**/issues");
    await expect(page).toHaveURL(/\/issues/);
  });

  test("settings page loads via sidebar", async ({ page }) => {
    await page.getByTestId("sidebar-nav-settings").click();
    await page.waitForURL("**/settings");

    await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
    // Default tab is Profile (My Account section)
    await expect(page.getByTestId("settings-tab-profile")).toBeVisible();
  });

  test("agents page shows agent list", async ({ page }) => {
    await page.getByTestId("sidebar-nav-agents").click();
    await page.waitForURL("**/agents");

    // Should show "Agents" heading
    await expect(page.locator("text=Agents").first()).toBeVisible();
  });
});
