import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AuthProvider, useAuth } from "./AuthContext";

vi.mock("../api/client", () => ({
  login: vi.fn(),
}));

import { login as apiLogin } from "../api/client";

const STORAGE_KEY = "pipelineops_token";
const wrapper = ({ children }: { children: ReactNode }) => <AuthProvider>{children}</AuthProvider>;

describe("AuthContext", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.mocked(apiLogin).mockReset();
  });

  it("starts unauthenticated when there is no stored token", () => {
    const { result } = renderHook(() => useAuth(), { wrapper });
    expect(result.current.isAuthenticated).toBe(false);
    expect(result.current.token).toBeNull();
  });

  it("picks up a token already in localStorage on mount", () => {
    localStorage.setItem(STORAGE_KEY, "existing-token");
    const { result } = renderHook(() => useAuth(), { wrapper });
    expect(result.current.isAuthenticated).toBe(true);
    expect(result.current.token).toBe("existing-token");
  });

  it("stores the token and flips isAuthenticated on successful login", async () => {
    vi.mocked(apiLogin).mockResolvedValue("new-token");
    const { result } = renderHook(() => useAuth(), { wrapper });

    await act(async () => {
      await result.current.login("admin", "admin");
    });

    expect(result.current.token).toBe("new-token");
    expect(result.current.isAuthenticated).toBe(true);
    expect(localStorage.getItem(STORAGE_KEY)).toBe("new-token");
  });

  it("does not store a token when login rejects", async () => {
    vi.mocked(apiLogin).mockRejectedValue(new Error("invalid credentials"));
    const { result } = renderHook(() => useAuth(), { wrapper });

    await act(async () => {
      await expect(result.current.login("admin", "wrong")).rejects.toThrow();
    });

    expect(result.current.isAuthenticated).toBe(false);
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
  });

  it("clears the token on logout", async () => {
    localStorage.setItem(STORAGE_KEY, "existing-token");
    const { result } = renderHook(() => useAuth(), { wrapper });

    act(() => {
      result.current.logout();
    });

    await waitFor(() => expect(result.current.isAuthenticated).toBe(false));
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
  });
});
