import axios from "axios";

import type { AlertEvent, AlertRule, Job, JobDetail, Paginated } from "../types";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8000";

export const api = axios.create({
  baseURL: API_BASE_URL,
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem("pipelineops_token");
  if (token) {
    config.headers.Authorization = `Token ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem("pipelineops_token");
      window.location.href = "/login";
    }
    return Promise.reject(error);
  },
);

export async function login(username: string, password: string): Promise<string> {
  const { data } = await api.post<{ token: string }>("/api/auth/token/", { username, password });
  return data.token;
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
