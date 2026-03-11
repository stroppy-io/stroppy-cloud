import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from "react"
import { api } from "./api"
import type { User } from "@/proto/api/api_pb.ts"

interface AuthState {
  user: User | null
  loading: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  const loadUser = useCallback(async () => {
    const token = localStorage.getItem("access_token")
    if (!token) {
      setLoading(false)
      return
    }
    try {
      const res = await api.getCurrentUser({})
      setUser(res.user ?? null)
    } catch {
      localStorage.removeItem("access_token")
      localStorage.removeItem("refresh_token")
    }
    setLoading(false)
  }, [])

  useEffect(() => { loadUser() }, [loadUser])

  const login = useCallback(async (username: string, password: string) => {
    const res = await api.login({ username, password })
    localStorage.setItem("access_token", res.accessToken)
    localStorage.setItem("refresh_token", res.refreshToken)
    setUser(res.user ?? null)
  }, [])

  const logout = useCallback(async () => {
    try { await api.logout({}) } catch { /* ignore */ }
    localStorage.removeItem("access_token")
    localStorage.removeItem("refresh_token")
    setUser(null)
  }, [])

  return (
    <AuthContext.Provider value={{ user, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within AuthProvider")
  return ctx
}
