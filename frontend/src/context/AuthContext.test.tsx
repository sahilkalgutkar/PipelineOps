import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AuthProvider, useAuth } from "./AuthContext";

vi.mock("../api/client", () => ({
  fetchCurrentUser: vi.fn(),
  login: vi.fn(),
  logout: vi.fn(),
}));

import { fetchCurrentUser, login as apiLogin, logout as apiLogout } from "../api/client";

const wrapper = ({ children }: { children: ReactNode }) => <AuthProvider>{children}</AuthProvider>;

describe("AuthContext", () => {
  beforeEach(() => {
    vi.mocked(fetchCurrentUser).mockReset();
    vi.mocked(apiLogin).mockReset();
    vi.mocked(apiLogout).mockReset();
  });

  it("starts loading, then unauthenticated when there is no valid session", async () => {
    vi.mocked(fetchCurrentUser).mockRejectedValue(new Error("401"));
    const { result } = renderHook(() => useAuth(), { wrapper });

    expect(result.current.isLoading).toBe(true);

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isAuthenticated).toBe(false);
    expect(result.current.username).toBeNull();
  });

  it("picks up an existing session cookie on mount", async () => {
    vi.mocked(fetchCurrentUser).mockResolvedValue({ username: "admin" });
    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isAuthenticated).toBe(true);
    expect(result.current.username).toBe("admin");
  });

  it("sets the authenticated user on successful login", async () => {
    vi.mocked(fetchCurrentUser).mockRejectedValue(new Error("401"));
    vi.mocked(apiLogin).mockResolvedValue({ username: "admin" });
    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => {
      await result.current.login("admin", "admin");
    });

    expect(result.current.isAuthenticated).toBe(true);
    expect(result.current.username).toBe("admin");
  });

  it("stays unauthenticated when login is rejected", async () => {
    vi.mocked(fetchCurrentUser).mockRejectedValue(new Error("401"));
    vi.mocked(apiLogin).mockRejectedValue(new Error("invalid credentials"));
    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => {
      await expect(result.current.login("admin", "wrong")).rejects.toThrow();
    });

    expect(result.current.isAuthenticated).toBe(false);
  });

  it("clears the user on logout", async () => {
    vi.mocked(fetchCurrentUser).mockResolvedValue({ username: "admin" });
    vi.mocked(apiLogout).mockResolvedValue(undefined);
    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isAuthenticated).toBe(true));

    await act(async () => {
      await result.current.logout();
    });

    expect(result.current.isAuthenticated).toBe(false);
    expect(result.current.username).toBeNull();
  });

  it("still clears local state if the logout request itself fails", async () => {
    vi.mocked(fetchCurrentUser).mockResolvedValue({ username: "admin" });
    vi.mocked(apiLogout).mockRejectedValue(new Error("network error"));
    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isAuthenticated).toBe(true));

    await act(async () => {
      await expect(result.current.logout()).rejects.toThrow();
    });

    expect(result.current.isAuthenticated).toBe(false);
  });
});
