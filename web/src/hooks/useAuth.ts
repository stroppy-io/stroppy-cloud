import { useState, useCallback, useEffect } from "react";
import { login as apiLogin, me as apiMe } from "@/api/client";

interface AuthUser {
  username: string;
  role: string;
}

export function useAuth() {
  const [token, setToken] = useState<string | null>(
    localStorage.getItem("token")
  );
  const [user, setUser] = useState<AuthUser | null>(null);

  // Fetch current user info when token is available.
  useEffect(() => {
    if (!token) {
      setUser(null);
      return;
    }
    apiMe()
      .then(setUser)
      .catch(() => {
        // Token invalid — clear it.
        localStorage.removeItem("token");
        setToken(null);
        setUser(null);
      });
  }, [token]);

  const login = useCallback(async (username: string, password: string) => {
    const res = await apiLogin(username, password);
    localStorage.setItem("token", res.token);
    setToken(res.token);
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem("token");
    setToken(null);
    setUser(null);
  }, []);

  return { token, user, login, logout, isAuthenticated: !!token };
}
