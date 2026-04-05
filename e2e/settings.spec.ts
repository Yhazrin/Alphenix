import { test, expect } from "@playwright/test";
import { loginAsDefault } from "./helpers";

test.describe("Settings", () => {
  test("updating workspace name reflects in sidebar immediately", async ({
    page,
  }) => {
    await loginAsDefault(page);

    // Read the current workspace name from the sidebar header
    const sidebarName = page.locator('[data-sidebar="header"] button').first();
    const originalName = await sidebarName.innerText();

    // Navigate to settings via sidebar nav link
    await page.locator('[data-sidebar="sidebar"] a', { hasText: "Settings" }).click();
    await page.waitForURL("**/settings");

    // Switch to General/Workspace tab (default is Profile)
    await page.getByTestId("settings-tab-workspace").click();

    // Change workspace name
    const nameInput = page.locator('input#workspace-name');
    await nameInput.clear();
    const newName = "Renamed WS " + Date.now();
    await nameInput.fill(newName);

    // Save
    await page.locator("button", { hasText: "Save" }).click();

    // Wait for confirmation toast
    await expect(page.locator("text=Workspace settings saved").first()).toBeVisible({ timeout: 5000 });

    // Sidebar should reflect the new name WITHOUT page refresh
    await expect(sidebarName).toContainText(newName);

    // Restore original name so other tests aren't affected
    await nameInput.clear();
    await nameInput.fill(originalName.trim());
    await page.locator("button", { hasText: "Save" }).click();
    await expect(page.locator("text=Workspace settings saved").first()).toBeVisible({ timeout: 5000 });
  });
});
