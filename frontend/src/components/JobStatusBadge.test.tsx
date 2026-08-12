import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { JobStatus } from "../types";
import { JobStatusBadge } from "./JobStatusBadge";

describe("JobStatusBadge", () => {
  const cases: Array<[JobStatus, string]> = [
    ["healthy", "Healthy"],
    ["late", "Late"],
    ["failed", "Failed"],
    ["unknown", "Unknown"],
  ];

  it.each(cases)("renders the label for status %s", (status, label) => {
    render(<JobStatusBadge status={status} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it("applies the failed color classes for a failed job", () => {
    render(<JobStatusBadge status="failed" />);
    expect(screen.getByText("Failed")).toHaveClass("text-failed");
  });
});
