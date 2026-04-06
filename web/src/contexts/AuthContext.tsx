import {
  createContext,
  useState,
  useCallback,
  useEffect,
  type ReactNode,
} from "react";
import type { AuthUser } from "@/api/types";
import {
  loginAPI,
  logoutAPI,
  meAPI,
  refreshToken,
  selectTenantAPI,
  setAccessToken,
  SessionExpiredError,
} from "@/api/client";

export interface AuthContextValue {
  user: AuthUser | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
  selectTenant: (tenantId: string) => Promise<void>;
}

export const AuthContext = createContext<AuthContextValue>({
  user: null,
  isAuthenticated: false,
  isLoading: true,
  login: async () => {},
  logout: async () => {},
  refresh: async () => {},
  selectTenant: async () => {},
});

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const fetchUser = useCallback(async () => {
    try {
      const u = await meAPI();
      setUser(u);
    } catch (e) {
      setUser(null);
      setAccessToken(null);
      if (e instanceof SessionExpiredError) return;
      // Rethrow non-auth errors
    }
  }, []);

  // Global handler: catch unhandled SessionExpiredError from any API call.
  useEffect(() => {
    const handler = (event: PromiseRejectionEvent) => {
      if (event.reason instanceof SessionExpiredError) {
        event.preventDefault();
        setAccessToken(null);
        setUser(null);
      }
    };
    window.addEventListener("unhandledrejection", handler);
    return () => window.removeEventListener("unhandledrejection", handler);
  }, []);

  // On mount: try to restore session from httpOnly cookie.
  useEffect(() => {
    (async () => {
      try {
        const r = await refreshToken();
        setAccessToken(r.access_token);
        await fetchUser();
      } catch {
        setAccessToken(null);
        setUser(null);
      } finally {
        setIsLoading(false);
      }
    })();
  }, [fetchUser]);

  const login = useCallback(
    async (username: string, password: string) => {
      const r = await loginAPI(username, password);
      setAccessToken(r.access_token);
      await fetchUser();
    },
    [fetchUser]
  );

  const logout = useCallback(async () => {
    try {
      await logoutAPI();
    } catch {
      // best-effort
    }
    setAccessToken(null);
    setUser(null);
  }, []);

  const refresh = useCallback(async () => {
    try {
      const r = await refreshToken();
      setAccessToken(r.access_token);
      await fetchUser();
    } catch {
      setAccessToken(null);
      setUser(null);
    }
  }, [fetchUser]);

  const selectTenant = useCallback(
    async (tenantId: string) => {
      const r = await selectTenantAPI(tenantId);
      setAccessToken(r.access_token);
      await fetchUser();
    },
    [fetchUser]
  );

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user,
        isLoading,
        login,
        logout,
        refresh,
        selectTenant,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
