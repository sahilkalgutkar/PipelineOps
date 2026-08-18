import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { Heartbeat } from "../types";
import { HeartbeatChart } from "./HeartbeatChart";

function makeHeartbeat(overrides: Partial<Heartbeat>): Heartbeat {
  return {
    id: 1,
    job: "job-1",
    status: "success",
    duration_ms: 120,
    metadata: {},
    received_at: "2024-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("HeartbeatChart", () => {
  it("shows the empty state when there are no heartbeats", () => {
    render(<HeartbeatChart heartbeats={[]} />);
    expect(screen.getByText("No heartbeats received yet.")).toBeInTheDocument();
  });

  it("does not show the empty state when heartbeats are present", () => {
    render(<HeartbeatChart heartbeats={[makeHeartbeat({})]} />);
    expect(screen.queryByText("No heartbeats received yet.")).not.toBeInTheDocument();
  });

  it("renders the chart container (not the empty state) for a mix of success and failure heartbeats", () => {
    const { container } = render(
      <HeartbeatChart
        heartbeats={[
          makeHeartbeat({ id: 1, status: "failure", duration_ms: 300 }),
          makeHeartbeat({ id: 2, status: "success", duration_ms: 120 }),
        ]}
      />,
    );
    expect(screen.queryByText("No heartbeats received yet.")).not.toBeInTheDocument();
    expect(container.querySelector(".recharts-responsive-container")).not.toBeNull();
  });
});
