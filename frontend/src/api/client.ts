import axios from "axios";

import type { AlertEvent, AlertRule, Job, JobDetail, Paginated } from "../types";

// Lets client.ts flag a request as "don't hard-redirect to /login on a 401
// from this one" — used for the login attempt itself and the on-mount
// whoami check, both of which expect a 401 as a normal, non-fatal outcome.
declare module "axios" {
  export interface AxiosRequestConfig {
    skipAuthRedirect?: boolean;
  }
}

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8000";
const UNSAFE_METHODS = new Set(["post", "put", "patch", "delete"]);

export const api = axios.create({
  baseURL: API_BASE_URL,
  // Auth is a session cookie now, not a bearer token, so every request
  // needs to actually carry cookies cross-origin.
  withCredentials: true,
});

function readCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp(`(?:^|;\\s*)${name}=([^;]*)`));
  return match ? decodeURIComponent(match[1]) : null;
}

api.interceptors.request.use((config) => {
  if (config.method && UNSAFE_METHODS.has(config.method)) {
    const csrfToken = readCookie("csrftoken");
    if (csrfToken) {
      config.headers["X-CSRFToken"] = csrfToken;
    }
  }
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401 && !error.config?.skipAuthRedirect) {
      window.location.href = "/login";
    }
    return Promise.reject(error);
  },
);

export interface CurrentUser {
  username: string;
}

export async function fetchCurrentUser(): Promise<CurrentUser> {
  const { data } = await api.get<CurrentUser>("/api/auth/me/", { skipAuthRedirect: true });
  return data;
}

export async function login(username: string, password: string): Promise<CurrentUser> {
  const { data } = await api.post<CurrentUser>(
    "/api/auth/login/",
    { username, password },
    { skipAuthRedirect: true },
  );
  return data;
}

export async function logout(): Promise<void> {
  await api.post("/api/auth/logout/");
}

export async function listJobs(): Promise<Job[]> {
  const { data } = await api.get<Paginated<Job>>("/api/jobs/");
  return data.results;
}

export async function getJob(id: string): Promise<JobDetail> {
  const { data } = await api.get<JobDetail>(`/api/jobs/${id}/`);
  return data;
}

export async function createJob(payload: Partial<Job>): Promise<Job> {
  const { data } = await api.post<Job>("/api/jobs/", payload);
  return data;
}

export async function listAlertRules(): Promise<AlertRule[]> {
  const { data } = await api.get<Paginated<AlertRule>>("/api/alert-rules/");
  return data.results;
}

export async function createAlertRule(payload: Partial<AlertRule>): Promise<AlertRule> {
  const { data } = await api.post<AlertRule>("/api/alert-rules/", payload);
  return data;
}

export async function deleteAlertRule(id: number): Promise<void> {
  await api.delete(`/api/alert-rules/${id}/`);
}

export async function listAlertEvents(): Promise<AlertEvent[]> {
  const { data } = await api.get<Paginated<AlertEvent>>("/api/alert-events/");
  return data.results;
}
