import type { User, UpdateMeRequest } from "@/shared/types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "";

export interface LoginResponse {
  token: string;
  user: User;
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
    credentials: "include",
  });
  if (!res.ok) {
    const body = await res.text().catch(() => res.statusText);
    throw new Error(`API error: ${res.status} ${body}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const authApi = {
  async sendCode(email: string): Promise<void> {
    await apiFetch("/auth/send-code", {
      method: "POST",
      body: JSON.stringify({ email }),
    });
  },

  async verifyCode(email: string, code: string): Promise<LoginResponse> {
    return apiFetch("/auth/verify-code", {
      method: "POST",
      body: JSON.stringify({ email, code }),
    });
  },

  async getMe(): Promise<User> {
    return apiFetch("/api/me");
  },

  async updateMe(data: UpdateMeRequest): Promise<User> {
    return apiFetch("/api/me", {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  },
};
