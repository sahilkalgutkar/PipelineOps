import type { JobStatus } from "../types";

const STYLES: Record<JobStatus, string> = {
  healthy: "bg-healthy/10 text-healthy border-healthy/30",
  late: "bg-late/10 text-late border-late/30",
  failed: "bg-failed/10 text-failed border-failed/30",
  unknown: "bg-unknown/10 text-unknown border-unknown/30",
};

const LABELS: Record<JobStatus, string> = {
  healthy: "Healthy",
  late: "Late",
  failed: "Failed",
  unknown: "Unknown",
};

export function JobStatusBadge({ status }: { status: JobStatus }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium ${STYLES[status]}`}
    >
      <span className="h-1.5 w-1.5 rounded-full bg-current" />
      {LABELS[status]}
    </span>
  );
}
