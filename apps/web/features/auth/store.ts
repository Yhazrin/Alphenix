"use client";

import { create } from "zustand";
import type { User } from "@/shared/types";
import { authApi } from "@/shared/api";

interface AuthState {
  user: User | null;
  isLoading: boolean;

  initialize: () => Promise<void>;
  sendCode: (email: string) => Promise<void>;
  verifyCode: (email: string, code: string) => Promise<User>;
  logout: () => Promise<void>;
  setUser: (user: User) => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoading: true,

  // Cookie-driven initialization: just call getMe().
  // The browser sends the HttpOnly "token" cookie automatically.
  initialize: async () => {
    try {
      const user = await authApi.getMe();
      set({ user, isLoading: false });
    } catch {
      set({ user: null, isLoading: false });
    }
  },

  sendCode: async (email: string) => {
    await authApi.sendCode(email);
  },

  verifyCode: async (email: string, code: string) => {
    const { user } = await authApi.verifyCode(email, code);
    // The server sets the HttpOnly "token" cookie in the response.
    // No localStorage needed.
    set({ user });
    return user;
  },

  logout: async () => {
    try {
      await authApi.logout();
    } catch {
      // Best-effort; clear local state regardless.
    }
    set({ user: null });
  },

  setUser: (user: User) => {
    set({ user });
  },
}));
