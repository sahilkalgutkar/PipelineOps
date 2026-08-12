import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import type { Heartbeat } from "../types";

interface Props {
  heartbeats: Heartbeat[];
}

export function HeartbeatChart({ heartbeats }: Props) {
  const data = [...heartbeats]
    .reverse()
    .map((hb) => ({
      time: new Date(hb.received_at).toLocaleTimeString(),
      duration_ms: hb.duration_ms ?? 0,
      failed: hb.status === "failure" ? hb.duration_ms ?? 0 : null,
    }));

  if (data.length === 0) {
    return (
      <div className="flex h-64 items-center justify-center text-sm text-slate-400">
        No heartbeats received yet.
      </div>
    );
  }

  return (
    <ResponsiveContainer width="100%" height={256}>
      <LineChart data={data} margin={{ top: 8, right: 16, left: 0, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
        <XAxis dataKey="time" tick={{ fontSize: 12 }} />
        <YAxis tick={{ fontSize: 12 }} label={{ value: "ms", angle: -90, position: "insideLeft" }} />
        <Tooltip />
        <Legend />
        <Line
          type="monotone"
          dataKey="duration_ms"
          name="Duration (ms)"
          stroke="#2563eb"
          strokeWidth={2}
          dot={false}
        />
        <Line
          type="monotone"
          dataKey="failed"
          name="Failure"
          stroke="#dc2626"
          strokeWidth={0}
          dot={{ r: 4 }}
        />
      </LineChart>
    </ResponsiveContainer>
  );
}
