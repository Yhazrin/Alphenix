import { test, expect } from "@playwright/test";
import { loginAsDefault, openWorkspaceMenu } from "./helpers";

test.describe("Authentication", () => {
  test("login page renders correctly", async ({ page }) => {
    await page.goto("/login");

    await expect(page.locator('input[placeholder="you@example.com"]')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toContainText(
      "Continue",
    );
  });

  test("login and redirect to /issues", async ({ page }) => {
    await loginAsDefault(page);

    await expect(page).toHaveURL(/\/issues/);
    await expect(page.locator("text=All Issues")).toBeVisible();
  });

  test("unauthenticated user is redirected to /login", async ({ page }) => {
    await page.goto("/login");
    await page.evaluate(() => {
      localStorage.removeItem("multicode_token");
      localStorage.removeItem("multicode_workspace_id");
    });

    await page.goto("/issues");
    await page.waitForURL("**/login", { timeout: 10000 });
  });

  test("logout redirects to /login", async ({ page }) => {
    await loginAsDefault(page);

    // Open the workspace dropdown menu
    await openWorkspaceMenu(page);

    // Click Log out
    await page.locator("text=Log out").click();

    await page.waitForURL("**/login", { timeout: 10000 });
    await expect(page).toHaveURL(/\/login/);
  });
});
