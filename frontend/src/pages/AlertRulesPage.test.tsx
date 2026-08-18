import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AlertRule, Job } from "../types";
import { AlertRulesPage } from "./AlertRulesPage";

const mockListAlertRules = vi.fn();
const mockListJobs = vi.fn();
const mockCreateAlertRule = vi.fn();
const mockDeleteAlertRule = vi.fn();

vi.mock("../api/client", () => ({
  listAlertRules: () => mockListAlertRules(),
  listJobs: () => mockListJobs(),
  createAlertRule: (payload: unknown) => mockCreateAlertRule(payload),
  deleteAlertRule: (id: number) => mockDeleteAlertRule(id),
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

function makeRule(overrides: Partial<AlertRule>): AlertRule {
  return {
    id: 1,
    job: "1",
    job_name: "nightly-etl",
    channel: "slack",
    target: "",
    is_active: true,
    created_at: "2024-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("AlertRulesPage", () => {
  beforeEach(() => {
    mockListAlertRules.mockReset();
    mockListJobs.mockReset();
    mockCreateAlertRule.mockReset();
    mockDeleteAlertRule.mockReset();
  });

  it("shows the empty state once rules and jobs resolve with no rules configured", async () => {
    mockListAlertRules.mockResolvedValue([]);
    mockListJobs.mockResolvedValue([makeJob({})]);
    render(<AlertRulesPage />);

    expect(
      await screen.findByText("No alert rules configured. Missed heartbeats fall back to Slack."),
    ).toBeInTheDocument();
  });

  it("renders existing rules in the table", async () => {
    mockListAlertRules.mockResolvedValue([
      makeRule({ id: 1, job_name: "nightly-etl", channel: "email", target: "ops@example.com" }),
    ]);
    mockListJobs.mockResolvedValue([makeJob({})]);
    render(<AlertRulesPage />);

    expect(await screen.findByText("ops@example.com")).toBeInTheDocument();
    expect(screen.getByText("email")).toBeInTheDocument();
  });

  it("shows 'default' as the target when a rule has no override target", async () => {
    mockListAlertRules.mockResolvedValue([makeRule({ target: "" })]);
    mockListJobs.mockResolvedValue([makeJob({})]);
    render(<AlertRulesPage />);

    expect(await screen.findByText("default")).toBeInTheDocument();
  });

  it("submits a new rule with the selected job, channel, and target, then refreshes", async () => {
    mockListAlertRules.mockResolvedValueOnce([]).mockResolvedValueOnce([
      makeRule({ id: 2, job_name: "nightly-etl", channel: "slack", target: "#alerts" }),
    ]);
    mockListJobs.mockResolvedValue([makeJob({ id: "1", name: "nightly-etl" })]);
    mockCreateAlertRule.mockResolvedValue(makeRule({ id: 2 }));

    render(<AlertRulesPage />);
    await screen.findByText("No alert rules configured. Missed heartbeats fall back to Slack.");

    await userEvent.type(
      screen.getByPlaceholderText("webhook URL / email / phone"),
      "#alerts",
    );
    await userEvent.click(screen.getByRole("button", { name: "Add rule" }));

    await waitFor(() =>
      expect(mockCreateAlertRule).toHaveBeenCalledWith({
        job: "1",
        channel: "slack",
        target: "#alerts",
      }),
    );
    expect(await screen.findByText("#alerts")).toBeInTheDocument();
    expect(mockListAlertRules).toHaveBeenCalledTimes(2);
  });

  it("deletes a rule and refreshes when Remove is clicked", async () => {
    mockListAlertRules
      .mockResolvedValueOnce([makeRule({ id: 5, job_name: "nightly-etl" })])
      .mockResolvedValueOnce([]);
    mockListJobs.mockResolvedValue([makeJob({})]);
    mockDeleteAlertRule.mockResolvedValue(undefined);

    render(<AlertRulesPage />);
    await screen.findByText("default");

    await userEvent.click(screen.getByText("Remove"));

    await waitFor(() => expect(mockDeleteAlertRule).toHaveBeenCalledWith(5));
    expect(
      await screen.findByText("No alert rules configured. Missed heartbeats fall back to Slack."),
    ).toBeInTheDocument();
  });
});
