// The only module that knows how the SPA authenticates.
// Dev mode: a static bearer token accepted by the server (`STROPPY_DEV_USERS`).
// IAM later replaces the implementation here; nothing else changes.
const STORAGE_KEY = 'stroppy.token'

let lostHandler: (() => void) | undefined

export function getToken(): string {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) return stored
  } catch {
    // storage unavailable (private mode) — fall through
  }
  return import.meta.env.VITE_DEV_TOKEN ?? 'dev'
}

export function setToken(token: string): void {
  try {
    localStorage.setItem(STORAGE_KEY, token)
  } catch {
    // ignore
  }
}

export function clearToken(): void {
  try {
    localStorage.removeItem(STORAGE_KEY)
  } catch {
    // ignore
  }
}

export function onAuthLost(handler: () => void): void {
  lostHandler = handler
}

export function notifyAuthLost(): void {
  lostHandler?.()
}
