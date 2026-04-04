import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { ApiClient } from "../client";

// Mock crypto.randomUUID
vi.stubGlobal("crypto", {
  randomUUID: () => "00000000-0000-0000-0000-000000000000",
});

describe("ApiClient", () => {
  let client: ApiClient;
  let fetchSpy: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    client = new ApiClient("http://localhost:3000");
    fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  /** Creates a proper Response-like mock with working json(). */
  function mockResponse(status: number, body?: unknown, statusText = "OK") {
    const jsonBody = body ?? {};
    return {
      ok: status >= 200 && status < 300,
      status,
      statusText,
      json: () => Promise.resolve(jsonBody),
    };
  }

  function mockOnce(status: number, body?: unknown, statusText = "OK") {
    fetchSpy.mockResolvedValueOnce(mockResponse(status, body, statusText));
  }

  describe("fetch behavior", () => {
    it("returns parsed JSON on success", async () => {
      mockOnce(200, { data: "ok" });
      const result = await client.getMe();
      expect(result).toEqual({ data: "ok" });
      expect(fetchSpy).toHaveBeenCalledTimes(1);
    });

    it("throws immediately on 400 without retry", async () => {
      mockOnce(400, { error: "bad request" }, "Bad Request");
      await expect(client.getMe()).rejects.toThrow("API error: bad request");
      expect(fetchSpy).toHaveBeenCalledTimes(1);
    });

    it("throws immediately on 404 without retry", async () => {
      mockOnce(404, { error: "not found" }, "Not Found");
      await expect(client.getMe()).rejects.toThrow("API error: not found");
      expect(fetchSpy).toHaveBeenCalledTimes(1);
    });

    it("does not retry non-GET requests on 500", async () => {
      mockOnce(500, { error: "fail" }, "Internal Server Error");
      await expect(client.sendCode("test@example.com")).rejects.toThrow();
      expect(fetchSpy).toHaveBeenCalledTimes(1);
    });

    it("returns undefined for 204 responses", async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: true,
        status: 204,
        statusText: "No Content",
        json: () => Promise.resolve({}),
      });
      const result = await client.deleteIssue("issue-1");
      expect(result).toBeUndefined();
    });

    it("retries GET on 500 and eventually succeeds", async () => {
      mockOnce(500, { error: "transient" }, "Internal Server Error");
      mockOnce(200, { user: "alice" });
      const result = await client.getMe();
      expect(result).toEqual({ user: "alice" });
      expect(fetchSpy).toHaveBeenCalledTimes(2);
    }, 15000);

    it("retries GET on 500 up to 3 retries then throws", async () => {
      // 1 initial + 3 retries = 4 total calls, all 500
      mockOnce(500, { error: "server error" }, "Internal Server Error");
      mockOnce(500, { error: "server error" }, "Internal Server Error");
      mockOnce(500, { error: "server error" }, "Internal Server Error");
      mockOnce(500, { error: "server error" }, "Internal Server Error");
      await expect(client.getMe()).rejects.toThrow("server error");
      expect(fetchSpy).toHaveBeenCalledTimes(4);
    }, 30000);

    it("retries on 429 then succeeds", async () => {
      mockOnce(429, { error: "rate limited" }, "Too Many Requests");
      mockOnce(200, { user: "alice" });
      const result = await client.getMe();
      expect(result).toEqual({ user: "alice" });
      expect(fetchSpy).toHaveBeenCalledTimes(2);
    }, 15000);
  });

  describe("auth headers", () => {
    it("includes Authorization when token is set", async () => {
      client.setToken("test-token");
      mockOnce(200, { user: "alice" });
      await client.getMe();
      const call = fetchSpy.mock.calls[0] as [string, { headers: Record<string, string | undefined> }];
      const init = call[1];
      expect(init.headers["Authorization"]).toBe("Bearer test-token");
    });

    it("includes X-Workspace-ID when set", async () => {
      client.setWorkspaceId("ws-123");
      mockOnce(200, []);
      await client.listInbox();
      const call2 = fetchSpy.mock.calls[0] as [string, { headers: Record<string, string | undefined> }];
      const init2 = call2[1];
      expect(init2.headers["X-Workspace-ID"]).toBe("ws-123");
    });

    it("omits Authorization when no token", async () => {
      mockOnce(200, []);
      await client.listInbox();
      const call3 = fetchSpy.mock.calls[0] as [string, { headers: Record<string, string | undefined> }];
      const init3 = call3[1];
      expect(init3.headers["Authorization"]).toBeUndefined();
    });
  });
});
