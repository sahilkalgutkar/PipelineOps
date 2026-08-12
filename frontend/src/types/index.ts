export type JobStatus = "unknown" | "healthy" | "late" | "failed";

export interface Job {
  id: string;
  name: string;
  description: string;
  schedule_description: string;
  expected_interval_seconds: number;
  grace_period_seconds: number;
  owner: number | null;
  owner_username: string | null;
  is_active: boolean;
  last_heartbeat_at: string | null;
  last_heartbeat_status: JobStatus;
  computed_status: JobStatus;
  created_at: string;
  updated_at: string;
}

export interface Heartbeat {
  id: number;
  job: string;
  status: "success" | "failure";
  duration_ms: number | null;
  metadata: Record<string, unknown>;
  received_at: string;
}

export interface JobDetail extends Job {
  recent_heartbeats: Heartbeat[];
}

export type AlertChannel = "slack" | "email" | "sms";

export interface AlertRule {
  id: number;
  job: string;
  job_name: string;
  channel: AlertChannel;
  target: string;
  is_active: boolean;
  created_at: string;
}

export type AlertEventStatus = "open" | "resolved";

export interface AlertEvent {
  id: number;
  job: string;
  job_name: string;
  rule: number | null;
  reason: string;
  status: AlertEventStatus;
  notified: boolean;
  triggered_at: string;
  resolved_at: string | null;
}

export interface Paginated<T> {
  count: number;
  next: string | null;
  previous: string | null;
  results: T[];
}
