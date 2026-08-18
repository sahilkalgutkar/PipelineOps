import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { LoginPage } from "./LoginPage";

const mockUseAuth = vi.fn();

vi.mock("../context/AuthContext", () => ({
  useAuth: () => mockUseAuth(),
}));

function renderLogin() {
  return render(
    <MemoryRouter initialEntries={["/login"]}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/" element={<div>Dashboard screen</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("LoginPage", () => {
  beforeEach(() => {
    mockUseAuth.mockReset();
  });

  it("renders nothing while auth state is loading", () => {
    mockUseAuth.mockReturnValue({ login: vi.fn(), isAuthenticated: false, isLoading: true });
    const { container } = renderLogin();
    expect(container).toBeEmptyDOMElement();
  });

  it("redirects to / when already authenticated", () => {
    mockUseAuth.mockReturnValue({ login: vi.fn(), isAuthenticated: true, isLoading: false });
    renderLogin();
    expect(screen.getByText("Dashboard screen")).toBeInTheDocument();
  });

  it("submits the form with the entered credentials and navigates on success", async () => {
    const login = vi.fn().mockResolvedValue(undefined);
    mockUseAuth.mockReturnValue({ login, isAuthenticated: false, isLoading: false });
    renderLogin();

    await userEvent.type(screen.getByLabelText("Username"), "admin");
    await userEvent.type(screen.getByLabelText("Password"), "hunter2");
    await userEvent.click(screen.getByRole("button", { name: "Sign in" }));

    expect(login).toHaveBeenCalledWith("admin", "hunter2");
    await waitFor(() => expect(screen.getByText("Dashboard screen")).toBeInTheDocument());
  });

  it("shows an error message and stays on the page when login fails", async () => {
    const login = vi.fn().mockRejectedValue(new Error("invalid credentials"));
    mockUseAuth.mockReturnValue({ login, isAuthenticated: false, isLoading: false });
    renderLogin();

    await userEvent.type(screen.getByLabelText("Username"), "admin");
    await userEvent.type(screen.getByLabelText("Password"), "wrong");
    await userEvent.click(screen.getByRole("button", { name: "Sign in" }));

    expect(await screen.findByText("Invalid username or password.")).toBeInTheDocument();
    expect(screen.queryByText("Dashboard screen")).not.toBeInTheDocument();
  });
});
