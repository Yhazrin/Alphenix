import { type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

const DEFAULT_E2E_NAME = "E2E User";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? `http://localhost:${process.env.PORT ?? "8080"}`;

// Each call gets a unique email to avoid the server's 10s/email rate limit
// on /auth/send-code.  Combining pid + timestamp + counter ensures uniqueness
// across parallel Playwright workers (separate processes) and sequential calls.
let counter = 0;
function uniqueCredentials() {
  const n = `${process.pid}-${Date.now()}-${counter++}`;
  return {
    email: `e2e+${n}@multicode.ai`,
    slug: `ws-${n}`,
  };
}

/**
 * Log in as the default E2E user and ensure the workspace exists first.
 * Sets the auth cookie via same-origin fetch through the Next.js proxy,
 * then sets localStorage and navigates to /issues.
 */
export async function loginAsDefault(page: Page) {
  const { email, slug } = uniqueCredentials();

  // Step 1: Send verification code (server-side, retry on 429 rate limit)
  let sendOk = false;
  for (let attempt = 0; attempt < 3 && !sendOk; attempt++) {
    const sendRes = await fetch(`${API_BASE}/auth/send-code`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
    });
    if (sendRes.ok) {
      sendOk = true;
    } else if (sendRes.status === 429) {
      console.warn(`[helpers] send-code rate-limited for ${email}, attempt ${attempt + 1}/3`);
      await new Promise((r) => setTimeout(r, 11000));
    } else {
      const body = await sendRes.text();
      throw new Error(`send-code failed: ${sendRes.status} ${body}`);
    }
  }

  // Step 2: Verify using the master code (888888).
  // Use page.request.post through the Next.js proxy to get the JWT.
  // Then manually add cookies to the browser context on the frontend origin,
  // since Playwright's API client sets cookies on the proxy target origin.
  const baseUrl = process.env.PLAYWRIGHT_BASE_URL || "http://localhost:3000";
  await page.goto("/login");
  const verifyRes = await page.request.post(`${baseUrl}/auth/verify-code`, {
    headers: { "Content-Type": "application/json" },
    data: JSON.stringify({ email, code: "888888" }),
  });
  const verifyResult = await verifyRes.json();
  const token = verifyResult.token;

  if (!token) {
    throw new Error(`verify-code returned no token: ${JSON.stringify(verifyResult)}`);
  }

  // Set the HttpOnly "token" cookie and "multicode_logged_in" cookie on the
  // frontend origin so the browser sends them on subsequent page loads.
  const url = new URL(baseUrl);
  await page.context().addCookies([
    {
      name: "token",
      value: token,
      domain: url.hostname,
      path: "/",
      httpOnly: true,
      sameSite: "Lax",
    },
    {
      name: "multicode_logged_in",
      value: "1",
      domain: url.hostname,
      path: "/",
      httpOnly: false,
      sameSite: "Lax",
    },
  ]);

  // Update user name if needed
  if (verifyResult.user?.name !== DEFAULT_E2E_NAME) {
    await fetch(`${API_BASE}/api/me`, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${token}`,
      },
      body: JSON.stringify({ name: DEFAULT_E2E_NAME }),
    });
  }

  // Step 3: Ensure workspace exists using the JWT token
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "Authorization": `Bearer ${token}`,
  };
  const wsListRes = await fetch(`${API_BASE}/api/workspaces`, { headers });
  const workspaces = await wsListRes.json();
  let workspace = workspaces.find((w: { slug: string }) => w.slug === slug) ?? workspaces[0];
  if (!workspace) {
    const createRes = await fetch(`${API_BASE}/api/workspaces`, {
      method: "POST",
      headers,
      body: JSON.stringify({ name: "E2E Workspace", slug }),
    });
    if (createRes.ok) {
      workspace = await createRes.json();
    } else {
      const refreshed = await fetch(`${API_BASE}/api/workspaces`, { headers }).then(r => r.json());
      workspace = refreshed.find((w: { slug: string }) => w.slug === slug) ?? refreshed[0];
    }
  }
  if (!workspace) throw new Error(`Failed to ensure workspace ${slug}`);

  // Set workspace ID in localStorage — navigate to /login for same-origin access
  await page.goto("/login");
  await page.evaluate((wsId) => {
    localStorage.setItem("multicode_workspace_id", wsId);
  }, workspace.id);

  // Now navigate — the cookie is set, AuthInitializer will authenticate
  await page.goto("/issues");
  await page.waitForURL("**/issues", { timeout: 15000 });

  return { token, workspaceId: workspace.id };
}

/**
 * Create a TestApiClient reusing credentials from loginAsDefault.
 * This ensures the API client and browser session share the same workspace.
 * Call api.cleanup() in afterEach to remove test data created during the test.
 */
export function createTestApi(token: string, workspaceId: string): TestApiClient {
  const api = new TestApiClient();
  api.setToken(token);
  api.setWorkspaceId(workspaceId);
  return api;
}

export async function openWorkspaceMenu(page: Page) {
  // Click the workspace switcher button in the sidebar header
  await page.locator('[data-sidebar="header"] button').first().click();
  // Wait for dropdown to appear
  await page.locator('[role="menu"]').waitFor({ state: "visible", timeout: 5000 });
}
