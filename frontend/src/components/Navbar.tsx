import { Link, useLocation } from "react-router-dom";

import { useAuth } from "../context/AuthContext";

const LINKS = [
  { to: "/", label: "Dashboard" },
  { to: "/alert-rules", label: "Alert Rules" },
];

export function Navbar() {
  const { isAuthenticated, logout } = useAuth();
  const location = useLocation();

  if (!isAuthenticated) return null;

  return (
    <nav className="border-b border-slate-200 bg-white">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-4 py-3">
        <div className="flex items-center gap-8">
          <span className="text-lg font-semibold text-slate-900">PipelineOps</span>
          <div className="flex gap-4">
            {LINKS.map((link) => (
              <Link
                key={link.to}
                to={link.to}
                className={`text-sm font-medium ${
                  location.pathname === link.to
                    ? "text-slate-900"
                    : "text-slate-500 hover:text-slate-800"
                }`}
              >
                {link.label}
              </Link>
            ))}
          </div>
        </div>
        <button
          onClick={logout}
          className="text-sm font-medium text-slate-500 hover:text-slate-800"
        >
          Log out
        </button>
      </div>
    </nav>
  );
}
