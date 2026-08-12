import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { listJobs } from "../api/client";
import { JobStatusBadge } from "../components/JobStatusBadge";
import type { Job } from "../types";

function timeAgo(iso: string | null): string {
  if (!iso) return "never";
  const seconds = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86400)}d ago`;
}

export function DashboardPage() {
  const [jobs, setJobs] = useState<Job[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const data = await listJobs();
        if (!cancelled) setJobs(data);
      } catch {
        if (!cancelled) setError("Failed to load jobs.");
      }
    }

    load();
    const interval = setInterval(load, 15000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, []);

  const counts = (jobs ?? []).reduce(
    (acc, job) => {
      acc[job.computed_status] += 1;
      return acc;
    },
    { healthy: 0, late: 0, failed: 0, unknown: 0 },
  );

  return (
    <div className="mx-auto max-w-6xl px-4 py-8">
      <h1 className="mb-6 text-2xl font-semibold text-slate-900">Job Health</h1>

      <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-4">
        {(["healthy", "late", "failed", "unknown"] as const).map((status) => (
          <div key={status} className="rounded-lg border border-slate-200 bg-white p-4">
            <div className="mb-1">
              <JobStatusBadge status={status} />
            </div>
            <div className="text-2xl font-semibold text-slate-900">{counts[status]}</div>
          </div>
        ))}
      </div>

      {error && <p className="text-sm text-failed">{error}</p>}

      <div className="overflow-hidden rounded-lg border border-slate-200 bg-white">
        <table className="min-w-full divide-y divide-slate-200">
          <thead className="bg-slate-50">
            <tr>
              <th className="px-4 py-2 text-left text-xs font-medium uppercase text-slate-500">
                Job
              </th>
              <th className="px-4 py-2 text-left text-xs font-medium uppercase text-slate-500">
                Status
              </th>
              <th className="px-4 py-2 text-left text-xs font-medium uppercase text-slate-500">
                Last Heartbeat
              </th>
              <th className="px-4 py-2 text-left text-xs font-medium uppercase text-slate-500">
                Expected Interval
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {jobs === null && (
              <tr>
                <td className="px-4 py-6 text-sm text-slate-400" colSpan={4}>
                  Loading...
                </td>
              </tr>
            )}
            {jobs?.length === 0 && (
              <tr>
                <td className="px-4 py-6 text-sm text-slate-400" colSpan={4}>
                  No jobs registered yet.
                </td>
              </tr>
            )}
            {jobs?.map((job) => (
              <tr key={job.id} className="hover:bg-slate-50">
                <td className="px-4 py-3 text-sm font-medium text-slate-900">
                  <Link to={`/jobs/${job.id}`} className="hover:underline">
                    {job.name}
                  </Link>
                </td>
                <td className="px-4 py-3">
                  <JobStatusBadge status={job.computed_status} />
                </td>
                <td className="px-4 py-3 text-sm text-slate-500">
                  {timeAgo(job.last_heartbeat_at)}
                </td>
                <td className="px-4 py-3 text-sm text-slate-500">
                  every {job.expected_interval_seconds}s
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
