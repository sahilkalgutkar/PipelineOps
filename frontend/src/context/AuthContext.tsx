import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

import { fetchCurrentUser, login as apiLogin, logout as apiLogout } from "../api/client";

interface AuthContextValue {
  username: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [username, setUsername] = useState<string | null>(null);
  // There's no token to read synchronously anymore — the session lives in
  // an httpOnly cookie the JS can't see, so the only way to know whether a
  // returning visitor is still logged in is to ask the server.
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    fetchCurrentUser()
      .then((user) => setUsername(user.username))
      .catch(() => setUsername(null))
      .finally(() => setIsLoading(false));
  }, []);

  async function login(username: string, password: string) {
    const user = await apiLogin(username, password);
    setUsername(user.username);
  }

  async function logout() {
    try {
      await apiLogout();
    } finally {
      setUsername(null);
    }
  }

  return (
    <AuthContext.Provider
      value={{ username, isAuthenticated: !!username, isLoading, login, logout }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
