import { test, expect } from "@playwright/test";
import { loginAsDefault } from "./helpers";

test.describe("Authentication", () => {
  test("login page renders correctly", async ({ page }) => {
    await page.goto("/login", { waitUntil: "networkidle" });

    await expect(page.getByTestId("auth-email-input")).toBeVisible();
    await expect(page.getByTestId("auth-submit-button")).toBeVisible();
  });

  test("login and redirect to /issues", async ({ page }) => {
    await loginAsDefault(page);

    await expect(page).toHaveURL(/\/issues/, { timeout: 20000 });
  });

  test("unauthenticated user is redirected away from /issues", async ({ page }) => {
    // Navigate to /issues with no auth cookies — the dashboard layout
    // detects no user and calls router.replace("/") which aborts the
    // original navigation, producing ERR_ABORTED. Catch it and verify
    // we end up at "/".
    try {
      await page.goto("/issues", { waitUntil: "networkidle" });
    } catch {
      // ERR_ABORTED is expected — the client-side redirect fires before load
    }
    await page.waitForURL("**/", { timeout: 20000 });
  });

  test("logout clears auth state", async ({ page }) => {
    await loginAsDefault(page);

    // Open the workspace switcher dropdown
    await page.getByTestId("workspace-menu-trigger").click();
    await page.locator('[role="menu"]').waitFor({ state: "visible", timeout: 10000 });

    // Click Log out
    await page.getByTestId("auth-logout-button").click();

    // Verify auth state is cleared — the sidebar should disappear
    await expect(page.getByTestId("workspace-menu-trigger")).not.toBeVisible({ timeout: 15000 });
  });
});
