import { describe, it, expect, vi, beforeEach } from "vitest";
import { useAuthStore } from "../store";

// Mock authApi
vi.mock("@/shared/api", () => ({
  authApi: {
    getMe: vi.fn(),
    sendCode: vi.fn(),
    verifyCode: vi.fn(),
    logout: vi.fn(),
  },
}));

// Mock the cookie helpers
vi.mock("../auth-cookie", () => ({
  setLoggedInCookie: vi.fn(),
  clearLoggedInCookie: vi.fn(),
}));

import { authApi } from "@/shared/api";
import { setLoggedInCookie, clearLoggedInCookie } from "../auth-cookie";

describe("auth store", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    useAuthStore.setState({ user: null, isLoading: true });
  });

  describe("initialize", () => {
    it("loads user from cookie-based session", async () => {
      const mockUser = { id: "u-1", email: "test@example.com", name: "Test" };
      vi.mocked(authApi.getMe).mockResolvedValueOnce(mockUser as any);

      await useAuthStore.getState().initialize();

      expect(authApi.getMe).toHaveBeenCalled();
      expect(useAuthStore.getState().user).toEqual(mockUser);
      expect(useAuthStore.getState().isLoading).toBe(false);
    });

    it("sets isLoading false on failed getMe", async () => {
      vi.mocked(authApi.getMe).mockRejectedValueOnce(new Error("unauthorized"));

      await useAuthStore.getState().initialize();

      expect(useAuthStore.getState().user).toBeNull();
      expect(useAuthStore.getState().isLoading).toBe(false);
    });
  });

  describe("verifyCode", () => {
    it("sets user after successful verification", async () => {
      const mockUser = { id: "u-1", email: "test@example.com", name: "Test" };
      vi.mocked(authApi.verifyCode).mockResolvedValueOnce({ token: "new-token", user: mockUser } as any);

      const user = await useAuthStore.getState().verifyCode("test@example.com", "123456");

      expect(authApi.verifyCode).toHaveBeenCalledWith("test@example.com", "123456");
      expect(user).toEqual(mockUser);
      expect(useAuthStore.getState().user).toEqual(mockUser);
      // Cookie is set by server response, not by client code
    });
  });

  describe("logout", () => {
    it("clears all auth state", async () => {
      vi.mocked(authApi.logout).mockResolvedValueOnce(undefined);
      useAuthStore.setState({ user: { id: "u-1", email: "test@example.com", name: "Test" } as any });

      await useAuthStore.getState().logout();

      expect(authApi.logout).toHaveBeenCalled();
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
