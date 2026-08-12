import { useEffect, useState, type FormEvent } from "react";

import { createAlertRule, deleteAlertRule, listAlertRules, listJobs } from "../api/client";
import type { AlertChannel, AlertRule, Job } from "../types";

export function AlertRulesPage() {
  const [rules, setRules] = useState<AlertRule[]>([]);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [jobId, setJobId] = useState("");
  const [channel, setChannel] = useState<AlertChannel>("slack");
  const [target, setTarget] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function refresh() {
    const [ruleData, jobData] = await Promise.all([listAlertRules(), listJobs()]);
    setRules(ruleData);
    setJobs(jobData);
    if (!jobId && jobData.length > 0) setJobId(jobData[0].id);
    setLoading(false);
  }

  useEffect(() => {
    refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    if (!jobId) return;
    setSubmitting(true);
    try {
      await createAlertRule({ job: jobId, channel, target });
      setTarget("");
      await refresh();
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDelete(id: number) {
    await deleteAlertRule(id);
    await refresh();
  }

  return (
    <div className="mx-auto max-w-6xl px-4 py-8">
      <h1 className="mb-6 text-2xl font-semibold text-slate-900">Alert Rules</h1>

      <form
        onSubmit={handleCreate}
        className="mb-8 grid grid-cols-1 gap-4 rounded-lg border border-slate-200 bg-white p-4 sm:grid-cols-4"
      >
        <div>
          <label className="mb-1 block text-xs font-medium text-slate-500">Job</label>
          <select
            className="w-full rounded-md border border-slate-300 px-2 py-1.5 text-sm"
            value={jobId}
            onChange={(e) => setJobId(e.target.value)}
          >
            {jobs.map((job) => (
              <option key={job.id} value={job.id}>
                {job.name}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-slate-500">Channel</label>
          <select
            className="w-full rounded-md border border-slate-300 px-2 py-1.5 text-sm"
            value={channel}
            onChange={(e) => setChannel(e.target.value as AlertChannel)}
          >
            <option value="slack">Slack</option>
            <option value="email">Email</option>
            <option value="sms">SMS</option>
          </select>
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-slate-500">
            Target (optional override)
          </label>
          <input
            className="w-full rounded-md border border-slate-300 px-2 py-1.5 text-sm"
            placeholder="webhook URL / email / phone"
            value={target}
            onChange={(e) => setTarget(e.target.value)}
          />
        </div>
        <div className="flex items-end">
          <button
            type="submit"
            disabled={submitting || !jobId}
            className="w-full rounded-md bg-slate-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50"
          >
            Add rule
          </button>
        </div>
      </form>

      <div className="overflow-hidden rounded-lg border border-slate-200 bg-white">
        <table className="min-w-full divide-y divide-slate-200">
          <thead className="bg-slate-50">
            <tr>
              <th className="px-4 py-2 text-left text-xs font-medium uppercase text-slate-500">
                Job
              </th>
              <th className="px-4 py-2 text-left text-xs font-medium uppercase text-slate-500">
                Channel
              </th>
              <th className="px-4 py-2 text-left text-xs font-medium uppercase text-slate-500">
                Target
              </th>
              <th className="px-4 py-2" />
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {loading && (
              <tr>
                <td className="px-4 py-6 text-sm text-slate-400" colSpan={4}>
                  Loading...
                </td>
              </tr>
            )}
            {!loading && rules.length === 0 && (
              <tr>
                <td className="px-4 py-6 text-sm text-slate-400" colSpan={4}>
                  No alert rules configured. Missed heartbeats fall back to Slack.
                </td>
              </tr>
            )}
            {rules.map((rule) => (
              <tr key={rule.id}>
                <td className="px-4 py-2 text-sm text-slate-700">{rule.job_name}</td>
                <td className="px-4 py-2 text-sm capitalize text-slate-700">{rule.channel}</td>
                <td className="px-4 py-2 text-sm text-slate-500">{rule.target || "default"}</td>
                <td className="px-4 py-2 text-right">
                  <button
                    onClick={() => handleDelete(rule.id)}
                    className="text-sm text-failed hover:underline"
                  >
                    Remove
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
