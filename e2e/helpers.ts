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
 * Sets the HttpOnly auth cookie in the browser by calling verify-code from
 * the page context, so the app's AuthInitializer picks up the session.
 */
export async function loginAsDefault(page: Page) {
  const { email, slug } = uniqueCredentials();

  // Step 1: Send verification code (server-side)
  const sendRes = await fetch(`${API_BASE}/auth/send-code`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email }),
  });
  if (!sendRes.ok) {
    throw new Error(`send-code failed: ${sendRes.status} ${await sendRes.text()}`);
  }

  // Step 2: Read code from DB
  const pg = await import("pg");
  const DATABASE_URL = process.env.DATABASE_URL ?? "postgres://multicode:multicode@localhost:5432/multicode?sslmode=disable";
  const client = new pg.Client(DATABASE_URL);
  await client.connect();
  let token: string;
  try {
    const result = await client.query(
      "SELECT code FROM verification_code WHERE email = $1 AND used = FALSE AND expires_at > now() ORDER BY created_at DESC LIMIT 1",
      [email]
    );
    if (result.rows.length === 0) {
      throw new Error(`No verification code found for ${email}`);
    }
    const code = result.rows[0].code;

    // Step 3: Call verify-code from the browser so the Set-Cookie header
    // is processed by the browser (HttpOnly cookie named "token").
    // Use a relative URL so the request goes through the Next.js dev proxy
    // (same origin), avoiding SameSite=Lax cookie rejection on cross-origin POSTs.
    await page.goto("/login");
    const verifyResult = await page.evaluate(async ({ email, code }) => {
      const res = await fetch("/auth/verify-code", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, code }),
        credentials: "include",
      });
      return res.json();
    }, { email, code });
    token = verifyResult.token;

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
  } finally {
    await client.end();
  }

  // Step 4: Ensure workspace exists using the JWT token
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

  // Set workspace ID in localStorage so AuthInitializer picks it up
  await page.evaluate((wsId) => {
    localStorage.setItem("multicode_workspace_id", wsId);
  }, workspace.id);

  // Now navigate — the cookie is set, AuthInitializer will authenticate
  await page.goto("/issues");
  await page.waitForURL("**/issues", { timeout: 15000 });
}

/**
 * Create a TestApiClient logged in as the default E2E user.
 * Call api.cleanup() in afterEach to remove test data created during the test.
 */
export async function createTestApi(): Promise<TestApiClient> {
  const api = new TestApiClient();
  const { email, slug } = uniqueCredentials();
  await api.login(email, DEFAULT_E2E_NAME);
  await api.ensureWorkspace("E2E Workspace", slug);
  return api;
}

export async function openWorkspaceMenu(page: Page) {
  // Click the workspace switcher button in the sidebar header
  await page.locator('[data-sidebar="header"] button').first().click();
  // Wait for dropdown to appear
  await page.locator('[role="menu"]').waitFor({ state: "visible", timeout: 5000 });
}
