import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { Job } from "../types";
import { DashboardPage } from "./DashboardPage";

const mockListJobs = vi.fn();

vi.mock("../api/client", () => ({
  listJobs: () => mockListJobs(),
}));

function makeJob(overrides: Partial<Job>): Job {
  return {
    id: "1",
    name: "nightly-etl",
    description: "",
    schedule_description: "",
    expected_interval_seconds: 3600,
    grace_period_seconds: 60,
    owner: null,
    owner_username: null,
    is_active: true,
    last_heartbeat_at: null,
    last_heartbeat_status: "unknown",
    computed_status: "unknown",
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
    ...overrides,
  };
}

function renderDashboard() {
  return render(
    <MemoryRouter>
      <DashboardPage />
    </MemoryRouter>,
  );
}

describe("DashboardPage", () => {
  beforeEach(() => {
    mockListJobs.mockReset();
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows a loading row before jobs resolve", () => {
    mockListJobs.mockReturnValue(new Promise(() => {}));
    renderDashboard();
    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("shows an empty state once jobs resolve to an empty list", async () => {
    mockListJobs.mockResolvedValue([]);
    renderDashboard();
    expect(await screen.findByText("No jobs registered yet.")).toBeInTheDocument();
  });

  it("renders job rows and tallies status counts", async () => {
    mockListJobs.mockResolvedValue([
      makeJob({ id: "1", name: "nightly-etl", computed_status: "healthy" }),
      makeJob({ id: "2", name: "hourly-sync", computed_status: "failed" }),
      makeJob({ id: "3", name: "weekly-report", computed_status: "healthy" }),
    ]);
    renderDashboard();

    await screen.findByText("nightly-etl");
    expect(screen.getByText("hourly-sync")).toBeInTheDocument();
    expect(screen.getByText("weekly-report")).toBeInTheDocument();

    // Healthy count tile should read 2, Failed count tile should read 1.
    const tiles = screen.getAllByText(/^\d+$/);
    const counts = tiles.map((el) => el.textContent);
    expect(counts).toEqual(["2", "0", "1", "0"]); // healthy, late, failed, unknown
  });

  it("shows an error message when loading jobs fails", async () => {
    mockListJobs.mockRejectedValue(new Error("network error"));
    renderDashboard();
    expect(await screen.findByText("Failed to load jobs.")).toBeInTheDocument();
  });

  it("shows 'never' for a job with no heartbeat and formats interval seconds", async () => {
    mockListJobs.mockResolvedValue([
      makeJob({ id: "1", name: "nightly-etl", last_heartbeat_at: null, expected_interval_seconds: 900 }),
    ]);
    renderDashboard();

    await screen.findByText("nightly-etl");
    expect(screen.getByText("never")).toBeInTheDocument();
    expect(screen.getByText("every 900s")).toBeInTheDocument();
  });
});
