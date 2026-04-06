import { test, expect } from "@playwright/test";
import { loginAsDefault } from "./helpers";

test.describe("Settings", () => {
  test("updating workspace name reflects in sidebar immediately", async ({
    page,
  }) => {
    await loginAsDefault(page);

    // Read the current workspace name from the sidebar header
    const sidebarName = page.getByTestId("workspace-menu-trigger");
    const originalName = await sidebarName.innerText();

    // Navigate to settings via sidebar nav link
    await page.getByTestId("sidebar-nav-settings").click();
    await page.waitForURL("**/settings");

    // Switch to General/Workspace tab (default is Profile)
    await page.getByTestId("settings-tab-workspace").click();

    // Change workspace name
    const nameInput = page.getByTestId("settings-workspace-name-input");
    await nameInput.clear();
    const newName = "Renamed WS " + Date.now();
    await nameInput.fill(newName);

    // Save
    await page.getByTestId("settings-workspace-save-btn").click();

    // Wait for confirmation toast — use getByText to avoid strict mode issues with duplicate toast DOM
    await expect(page.getByText("Workspace settings saved").first()).toBeVisible({ timeout: 5000 });

    // Sidebar should reflect the new name WITHOUT page refresh
    await expect(sidebarName).toContainText(newName);

    // Restore original name so other tests aren't affected
    await nameInput.clear();
    await nameInput.fill(originalName.trim());
    await page.getByTestId("settings-workspace-save-btn").click();
    await expect(page.getByText("Workspace settings saved").first()).toBeVisible({ timeout: 5000 });
  });
});
