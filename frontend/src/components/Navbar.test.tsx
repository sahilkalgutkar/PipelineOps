import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { Navbar } from "./Navbar";

const mockUseAuth = vi.fn();

vi.mock("../context/AuthContext", () => ({
  useAuth: () => mockUseAuth(),
}));

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Navbar />
    </MemoryRouter>,
  );
}

describe("Navbar", () => {
  beforeEach(() => {
    mockUseAuth.mockReset();
  });

  it("renders nothing when the user is not authenticated", () => {
    mockUseAuth.mockReturnValue({ isAuthenticated: false, logout: vi.fn() });
    const { container } = renderAt("/");
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nav links when authenticated", () => {
    mockUseAuth.mockReturnValue({ isAuthenticated: true, logout: vi.fn() });
    renderAt("/");
    expect(screen.getByText("Dashboard")).toBeInTheDocument();
    expect(screen.getByText("Alert Rules")).toBeInTheDocument();
    expect(screen.getByText("Log out")).toBeInTheDocument();
  });

  it("highlights the active route link", () => {
    mockUseAuth.mockReturnValue({ isAuthenticated: true, logout: vi.fn() });
    renderAt("/alert-rules");
    expect(screen.getByText("Alert Rules")).toHaveClass("text-slate-900");
    expect(screen.getByText("Dashboard")).not.toHaveClass("text-slate-900");
  });

  it("calls logout when the log out button is clicked", async () => {
    const logout = vi.fn();
    mockUseAuth.mockReturnValue({ isAuthenticated: true, logout });
    renderAt("/");
    await userEvent.click(screen.getByText("Log out"));
    expect(logout).toHaveBeenCalledTimes(1);
  });
});
