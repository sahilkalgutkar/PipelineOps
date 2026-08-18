import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  api,
  createAlertRule,
  createJob,
  deleteAlertRule,
  fetchCurrentUser,
  getJob,
  listAlertEvents,
  listAlertRules,
  listJobs,
  login,
  logout,
} from "./client";

function setCookie(value: string) {
  document.cookie = value;
}

function clearCookies() {
  document.cookie.split(";").forEach((c) => {
    const name = c.split("=")[0].trim();
    if (name) {
      document.cookie = `${name}=;expires=Thu, 01 Jan 1970 00:00:00 UTC;path=/`;
    }
  });
}

// axios types interceptor.handlers as possibly undefined (it's only ever
// unset if you manually null it out), so pull the first live handler pair
// out with a single assertion instead of repeating `!` at every call site.
function requestInterceptor() {
  return api.interceptors.request.handlers![0].fulfilled!;
}

function responseRejectedInterceptor() {
  return api.interceptors.response.handlers![0].rejected!;
}

describe("api client interceptors", () => {
  const originalLocation = window.location;

  beforeEach(() => {
    // jsdom doesn't implement real navigation, so assigning
    // window.location.href throws. Stub it with a plain object so the
    // interceptor's redirect assignment is just an observable property set.
    Object.defineProperty(window, "location", {
      configurable: true,
      writable: true,
      value: { href: "http://localhost:3000/" },
    });
  });

  afterEach(() => {
    clearCookies();
    Object.defineProperty(window, "location", {
      configurable: true,
      writable: true,
      value: originalLocation,
    });
  });

  it("attaches the X-CSRFToken header on unsafe methods when a csrftoken cookie is present", () => {
    setCookie("csrftoken=abc123");
    const config = requestInterceptor()({ method: "post", headers: {} } as never) as {
      headers: Record<string, string>;
    };
    expect(config.headers["X-CSRFToken"]).toBe("abc123");
  });

  it("does not attach the CSRF header on safe methods like GET", () => {
    setCookie("csrftoken=abc123");
    const config = requestInterceptor()({ method: "get", headers: {} } as never) as {
      headers: Record<string, string>;
    };
    expect(config.headers["X-CSRFToken"]).toBeUndefined();
  });

  it("does not attach a CSRF header when no csrftoken cookie exists", () => {
    const config = requestInterceptor()({ method: "delete", headers: {} } as never) as {
      headers: Record<string, string>;
    };
    expect(config.headers["X-CSRFToken"]).toBeUndefined();
  });

  it("redirects to /login on a 401 response without skipAuthRedirect", async () => {
    await expect(
      responseRejectedInterceptor()({
        response: { status: 401 },
        config: {},
      }),
    ).rejects.toBeDefined();

    expect(window.location.href).toBe("/login");
  });

  it("does not redirect on a 401 response when skipAuthRedirect is set", async () => {
    const hrefBefore = window.location.href;
    await expect(
      responseRejectedInterceptor()({
        response: { status: 401 },
        config: { skipAuthRedirect: true },
      }),
    ).rejects.toBeDefined();

    expect(window.location.href).toBe(hrefBefore);
  });

  it("does not redirect on non-401 errors", async () => {
    const hrefBefore = window.location.href;
    await expect(
      responseRejectedInterceptor()({
        response: { status: 500 },
        config: {},
      }),
    ).rejects.toBeDefined();

    expect(window.location.href).toBe(hrefBefore);
  });
});

describe("api client request helpers", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("fetchCurrentUser GETs /api/auth/me/ with skipAuthRedirect and returns the user", async () => {
    const spy = vi.spyOn(api, "get").mockResolvedValue({ data: { username: "admin" } });
    const result = await fetchCurrentUser();
    expect(spy).toHaveBeenCalledWith("/api/auth/me/", { skipAuthRedirect: true });
    expect(result).toEqual({ username: "admin" });
  });

  it("login POSTs credentials with skipAuthRedirect and returns the user", async () => {
    const spy = vi.spyOn(api, "post").mockResolvedValue({ data: { username: "admin" } });
    const result = await login("admin", "hunter2");
    expect(spy).toHaveBeenCalledWith(
      "/api/auth/login/",
      { username: "admin", password: "hunter2" },
      { skipAuthRedirect: true },
    );
    expect(result).toEqual({ username: "admin" });
  });

  it("logout POSTs to /api/auth/logout/", async () => {
    const spy = vi.spyOn(api, "post").mockResolvedValue({ data: undefined });
    await logout();
    expect(spy).toHaveBeenCalledWith("/api/auth/logout/");
  });

  it("listJobs unwraps the paginated results array", async () => {
    const jobs = [{ id: "1", name: "nightly-etl" }];
    vi.spyOn(api, "get").mockResolvedValue({ data: { count: 1, next: null, previous: null, results: jobs } });
    const result = await listJobs();
    expect(result).toBe(jobs);
  });

  it("getJob fetches a single job by id", async () => {
    const job = { id: "42", name: "nightly-etl" };
    const spy = vi.spyOn(api, "get").mockResolvedValue({ data: job });
    const result = await getJob("42");
    expect(spy).toHaveBeenCalledWith("/api/jobs/42/");
    expect(result).toBe(job);
  });

  it("createJob POSTs the payload and returns the created job", async () => {
    const payload = { name: "new-job" };
    const created = { id: "9", name: "new-job" };
    const spy = vi.spyOn(api, "post").mockResolvedValue({ data: created });
    const result = await createJob(payload);
    expect(spy).toHaveBeenCalledWith("/api/jobs/", payload);
    expect(result).toBe(created);
  });

  it("listAlertRules unwraps the paginated results array", async () => {
    const rules = [{ id: 1, channel: "slack" }];
    vi.spyOn(api, "get").mockResolvedValue({ data: { count: 1, next: null, previous: null, results: rules } });
    const result = await listAlertRules();
    expect(result).toBe(rules);
  });

  it("createAlertRule POSTs the payload", async () => {
    const payload = { job: "1", channel: "email" as const };
    const created = { id: 5, ...payload };
    const spy = vi.spyOn(api, "post").mockResolvedValue({ data: created });
    const result = await createAlertRule(payload);
    expect(spy).toHaveBeenCalledWith("/api/alert-rules/", payload);
    expect(result).toBe(created);
  });

  it("deleteAlertRule DELETEs the rule by id", async () => {
    const spy = vi.spyOn(api, "delete").mockResolvedValue({ data: undefined });
    await deleteAlertRule(7);
    expect(spy).toHaveBeenCalledWith("/api/alert-rules/7/");
  });

  it("listAlertEvents unwraps the paginated results array", async () => {
    const events = [{ id: 1, reason: "missed heartbeat" }];
    vi.spyOn(api, "get").mockResolvedValue({ data: { count: 1, next: null, previous: null, results: events } });
    const result = await listAlertEvents();
    expect(result).toBe(events);
  });
});
