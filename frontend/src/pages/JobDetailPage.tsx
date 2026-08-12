import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { getJob } from "../api/client";
import { HeartbeatChart } from "../components/HeartbeatChart";
import { JobStatusBadge } from "../components/JobStatusBadge";
import type { JobDetail } from "../types";

export function JobDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [job, setJob] = useState<JobDetail | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;

    async function load() {
      try {
        const data = await getJob(id!);
        if (!cancelled) setJob(data);
      } catch {
        if (!cancelled) setError("Failed to load job.");
      }
    }

    load();
    const interval = setInterval(load, 10000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [id]);

  if (error) {
    return <div className="mx-auto max-w-6xl px-4 py-8 text-sm text-failed">{error}</div>;
  }

  if (!job) {
    return <div className="mx-auto max-w-6xl px-4 py-8 text-sm text-slate-400">Loading...</div>;
  }

  return (
    <div className="mx-auto max-w-6xl px-4 py-8">
      <Link to="/" className="mb-4 inline-block text-sm text-slate-500 hover:underline">
        &larr; Back to dashboard
      </Link>

      <div className="mb-6 flex items-center gap-3">
        <h1 className="text-2xl font-semibold text-slate-900">{job.name}</h1>
        <JobStatusBadge status={job.computed_status} />
      </div>

      {job.description && <p className="mb-6 text-sm text-slate-500">{job.description}</p>}

      <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Stat label="Schedule" value={job.schedule_description || "—"} />
        <Stat label="Expected Interval" value={`${job.expected_interval_seconds}s`} />
        <Stat label="Grace Period" value={`${job.grace_period_seconds}s`} />
        <Stat
          label="Last Heartbeat"
          value={job.last_heartbeat_at ? new Date(job.last_heartbeat_at).toLocaleString() : "never"}
        />
      </div>

      <div className="mb-8 rounded-lg border border-slate-200 bg-white p-4">
        <h2 className="mb-4 text-sm font-medium text-slate-700">Recent Heartbeat Durations</h2>
        <HeartbeatChart heartbeats={job.recent_heartbeats} />
      </div>

      <div className="overflow-hidden rounded-lg border border-slate-200 bg-white">
        <table className="min-w-full divide-y divide-slate-200">
          <thead className="bg-slate-50">
            <tr>
              <th className="px-4 py-2 text-left text-xs font-medium uppercase text-slate-500">
                Received
              </th>
              <th className="px-4 py-2 text-left text-xs font-medium uppercase text-slate-500">
                Status
              </th>
              <th className="px-4 py-2 text-left text-xs font-medium uppercase text-slate-500">
                Duration
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {job.recent_heartbeats.length === 0 && (
              <tr>
                <td className="px-4 py-6 text-sm text-slate-400" colSpan={3}>
                  No heartbeats yet.
                </td>
              </tr>
            )}
            {job.recent_heartbeats.map((hb) => (
              <tr key={hb.id}>
                <td className="px-4 py-2 text-sm text-slate-700">
                  {new Date(hb.received_at).toLocaleString()}
                </td>
                <td className="px-4 py-2 text-sm">
                  <span className={hb.status === "success" ? "text-healthy" : "text-failed"}>
                    {hb.status}
                  </span>
                </td>
                <td className="px-4 py-2 text-sm text-slate-500">
                  {hb.duration_ms != null ? `${hb.duration_ms}ms` : "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-4">
      <div className="text-xs uppercase text-slate-400">{label}</div>
      <div className="mt-1 text-sm font-medium text-slate-900">{value}</div>
    </div>
  );
}
