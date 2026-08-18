import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import App from "./App";

const mockFetchCurrentUser = vi.fn();

vi.mock("./api/client", () => ({
  fetchCurrentUser: () => mockFetchCurrentUser(),
  login: vi.fn(),
  logout: vi.fn(),
}));

function renderAppAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  );
}

describe("App", () => {
  beforeEach(() => {
    mockFetchCurrentUser.mockReset();
  });

  it("renders the login page for an unauthenticated visitor at /", async () => {
    mockFetchCurrentUser.mockRejectedValue(new Error("401"));
    renderAppAt("/");
    expect(await screen.findByText("Sign in to view job health.")).toBeInTheDocument();
  });

  it("redirects an unknown route to the dashboard route (which then bounces to login)", async () => {
    mockFetchCurrentUser.mockRejectedValue(new Error("401"));
    renderAppAt("/does-not-exist");
    expect(await screen.findByText("Sign in to view job health.")).toBeInTheDocument();
  });
});
