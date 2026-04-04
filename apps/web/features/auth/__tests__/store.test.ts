import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { useAuthStore } from "../store";

// Mock the api module
vi.mock("@/shared/api", () => {
  const mockApi = {
    setToken: vi.fn(),
    setWorkspaceId: vi.fn(),
    getMe: vi.fn(),
    sendCode: vi.fn(),
    verifyCode: vi.fn(),
  };
  return { api: mockApi };
});

// Mock the cookie helpers
vi.mock("../auth-cookie", () => ({
  setLoggedInCookie: vi.fn(),
  clearLoggedInCookie: vi.fn(),
}));

import { api } from "@/shared/api";
import { setLoggedInCookie, clearLoggedInCookie } from "../auth-cookie";

describe("auth store", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    // Reset store state
    useAuthStore.setState({ user: null, isLoading: true });
  });

  describe("initialize", () => {
    it("sets isLoading false when no token in localStorage", async () => {
      await useAuthStore.getState().initialize();
      const state = useAuthStore.getState();
      expect(state.isLoading).toBe(false);
      expect(state.user).toBeNull();
    });

    it("loads user when token exists", async () => {
      localStorage.setItem("multicode_token", "existing-token");
      const mockUser = { id: "u-1", email: "test@example.com", name: "Test" };
      vi.mocked(api.getMe).mockResolvedValueOnce(mockUser as any);

      await useAuthStore.getState().initialize();

      expect(api.setToken).toHaveBeenCalledWith("existing-token");
      expect(useAuthStore.getState().user).toEqual(mockUser);
      expect(useAuthStore.getState().isLoading).toBe(false);
    });

    it("clears token on failed getMe", async () => {
      localStorage.setItem("multicode_token", "bad-token");
      vi.mocked(api.getMe).mockRejectedValueOnce(new Error("unauthorized"));

      await useAuthStore.getState().initialize();

      expect(api.setToken).toHaveBeenCalledWith(null);
      expect(api.setWorkspaceId).toHaveBeenCalledWith(null);
      expect(localStorage.getItem("multicode_token")).toBeNull();
      expect(useAuthStore.getState().user).toBeNull();
      expect(useAuthStore.getState().isLoading).toBe(false);
    });
  });

  describe("verifyCode", () => {
    it("stores token, sets user, and sets cookie", async () => {
      const mockUser = { id: "u-1", email: "test@example.com", name: "Test" };
      vi.mocked(api.verifyCode).mockResolvedValueOnce({ token: "new-token", user: mockUser } as any);

      const user = await useAuthStore.getState().verifyCode("test@example.com", "123456");

      expect(api.verifyCode).toHaveBeenCalledWith("test@example.com", "123456");
      expect(localStorage.getItem("multicode_token")).toBe("new-token");
      expect(api.setToken).toHaveBeenCalledWith("new-token");
      expect(setLoggedInCookie).toHaveBeenCalled();
      expect(user).toEqual(mockUser);
      expect(useAuthStore.getState().user).toEqual(mockUser);
    });
  });

  describe("logout", () => {
    it("clears all auth state", () => {
      useAuthStore.setState({ user: { id: "u-1", email: "test@example.com", name: "Test" } as any });
      localStorage.setItem("multicode_token", "token");
      localStorage.setItem("multicode_workspace_id", "ws-1");

      useAuthStore.getState().logout();

      expect(localStorage.getItem("multicode_token")).toBeNull();
      expect(localStorage.getItem("multicode_workspace_id")).toBeNull();
      expect(api.setToken).toHaveBeenCalledWith(null);
      expect(api.setWorkspaceId).toHaveBeenCalledWith(null);
      expect(clearLoggedInCookie).toHaveBeenCalled();
      expect(useAuthStore.getState().user).toBeNull();
    });
  });

  describe("setUser", () => {
    it("sets the user directly", () => {
      const user = { id: "u-1", email: "a@b.com", name: "A" } as any;
      useAuthStore.getState().setUser(user);
      expect(useAuthStore.getState().user).toEqual(user);
    });
  });
});
