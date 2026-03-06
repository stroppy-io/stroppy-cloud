import { atom, computed } from "nanostores"

export const $accessToken = atom<string>(localStorage.getItem("stroppy-access-token") ?? "")
export const $refreshToken = atom<string>(localStorage.getItem("stroppy-refresh-token") ?? "")
export const $isAuthenticated = computed($accessToken, (token) => token.length > 0)

$accessToken.subscribe((token) => {
  if (token) {
    localStorage.setItem("stroppy-access-token", token)
  } else {
    localStorage.removeItem("stroppy-access-token")
  }
})

$refreshToken.subscribe((token) => {
  if (token) {
    localStorage.setItem("stroppy-refresh-token", token)
  } else {
    localStorage.removeItem("stroppy-refresh-token")
  }
})

async function rpc<T>(procedure: string, body: unknown): Promise<T> {
  const resp = await fetch(procedure, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  })
  if (!resp.ok) {
    const text = await resp.text()
    throw new Error(text || resp.statusText)
  }
  return resp.json() as Promise<T>
}

interface TokenResponse {
  accessToken: string
  refreshToken: string
  expiresAt: string
}

export async function login(username: string, password: string): Promise<void> {
  const resp = await rpc<TokenResponse>("/api.v1.AuthAPI/Login", { username, password })
  $accessToken.set(resp.accessToken)
  $refreshToken.set(resp.refreshToken)
}

export async function refreshAccessToken(): Promise<boolean> {
  const token = $refreshToken.get()
  if (!token) return false

  try {
    const resp = await rpc<TokenResponse>("/api.v1.AuthAPI/RefreshToken", { refreshToken: token })
    $accessToken.set(resp.accessToken)
    $refreshToken.set(resp.refreshToken)
    return true
  } catch {
    logout()
    return false
  }
}

export function logout() {
  $accessToken.set("")
  $refreshToken.set("")
}
