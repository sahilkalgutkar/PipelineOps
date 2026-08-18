import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { JobDetail } from "../types";
import { JobDetailPage } from "./JobDetailPage";

const mockGetJob = vi.fn();

vi.mock("../api/client", () => ({
  getJob: (id: string) => mockGetJob(id),
}));

function makeJobDetail(overrides: Partial<JobDetail>): JobDetail {
  return {
    id: "42",
    name: "nightly-etl",
    description: "Loads yesterday's data warehouse tables.",
    schedule_description: "0 2 * * *",
    expected_interval_seconds: 86400,
    grace_period_seconds: 300,
    owner: null,
    owner_username: null,
    is_active: true,
    last_heartbeat_at: "2024-01-01T02:00:00Z",
    last_heartbeat_status: "healthy",
    computed_status: "healthy",
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T02:00:00Z",
    recent_heartbeats: [],
    ...overrides,
  };
}

function renderAt(id: string) {
  return render(
    <MemoryRouter initialEntries={[`/jobs/${id}`]}>
      <Routes>
        <Route path="/jobs/:id" element={<JobDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("JobDetailPage", () => {
  beforeEach(() => {
    mockGetJob.mockReset();
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows a loading state before the job resolves", () => {
    mockGetJob.mockReturnValue(new Promise(() => {}));
    renderAt("42");
    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("fetches the job using the route id and renders its details", async () => {
    mockGetJob.mockResolvedValue(makeJobDetail({}));
    renderAt("42");

    expect(await screen.findByText("nightly-etl")).toBeInTheDocument();
    expect(mockGetJob).toHaveBeenCalledWith("42");
    expect(screen.getByText("Loads yesterday's data warehouse tables.")).toBeInTheDocument();
    expect(screen.getByText("0 2 * * *")).toBeInTheDocument();
  });

  it("shows an error message when loading the job fails", async () => {
    mockGetJob.mockRejectedValue(new Error("not found"));
    renderAt("999");
    expect(await screen.findByText("Failed to load job.")).toBeInTheDocument();
  });

  it("shows the empty heartbeat state when there are no recent heartbeats", async () => {
    mockGetJob.mockResolvedValue(makeJobDetail({ recent_heartbeats: [] }));
    renderAt("42");

    await screen.findByText("nightly-etl");
    expect(screen.getByText("No heartbeats yet.")).toBeInTheDocument();
  });

  it("renders recent heartbeat rows with status and duration", async () => {
    mockGetJob.mockResolvedValue(
      makeJobDetail({
        recent_heartbeats: [
          {
            id: 1,
            job: "42",
            status: "success",
            duration_ms: 512,
            metadata: {},
            received_at: "2024-01-01T02:00:00Z",
          },
          {
            id: 2,
            job: "42",
            status: "failure",
            duration_ms: null,
            metadata: {},
            received_at: "2024-01-01T01:00:00Z",
          },
        ],
      }),
    );
    renderAt("42");

    await screen.findByText("nightly-etl");
    expect(screen.getByText("512ms")).toBeInTheDocument();
    expect(screen.getByText("success")).toBeInTheDocument();
    expect(screen.getByText("failure")).toBeInTheDocument();
    // duration_ms null renders as an em dash
    expect(screen.getByText("—")).toBeInTheDocument();
  });
});
