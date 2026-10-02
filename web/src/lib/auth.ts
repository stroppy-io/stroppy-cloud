// The only module that knows how the SPA authenticates.
//
// Kratos mode: login/registration run against the Kratos API flows and give
// an opaque session token; it is exchanged for a short-lived session JWT
// (`stroppy` tokenizer template, see deployments/kratos/) that the server
// verifies locally. Refresh happens here, silently, before the token stales.
//
// Dev mode (a local stand without Kratos): a static bearer token accepted
// by the server (`STROPPY_DEV_USERS`), entered on the login page.

const SESSION_KEY = 'stroppy.kratos.session'
const TOKEN_KEY = 'stroppy.token'
// Refresh when less than this is left of the JWT's 10m life.
const REFRESH_MARGIN_MS = 60_000

let kratosBase = ''
let lostHandler: (() => void) | undefined
let refreshInFlight: Promise<string> | undefined

// --- kratos endpoints -------------------------------------------------------

/** Base URL of Kratos public, from the server's public config. */
export function setKratosBase(url: string): void {
  kratosBase = url.replace(/\/$/, '')
}

// --- session ----------------------------------------------------------------

function readSession(): string {
  try {
    return localStorage.getItem(SESSION_KEY) ?? ''
  } catch {
    return ''
  }
}

function writeSession(sessionToken: string): void {
  try {
    localStorage.setItem(SESSION_KEY, sessionToken)
  } catch {
    // ignore
  }
}

/** Whether there is a Kratos session to exchange (dev mode has none). */
export function hasSession(): boolean {
  return readSession() !== ''
}

// --- token ------------------------------------------------------------------

/** The current bearer token (JWT in kratos mode, static in dev mode). */
export function getToken(): string {
  try {
    const stored = localStorage.getItem(TOKEN_KEY)
    if (stored) return stored
  } catch {
    // storage unavailable (private mode) — fall through
  }
  return import.meta.env.VITE_DEV_TOKEN ?? 'dev'
}

type Exchange = { jwt: string; exp: number }

function decodeExp(jwt: string): number {
  try {
    const payload = JSON.parse(atob(jwt.split('.')[1] ?? '')) as { exp?: number }
    return (payload.exp ?? 0) * 1000
  } catch {
    return 0
  }
}

async function exchange(sessionToken: string): Promise<Exchange> {
  const res = await fetch(`${kratosBase}/sessions/whoami?tokenize_as=stroppy`, {
    headers: { 'X-Session-Token': sessionToken },
  })
  if (!res.ok) throw new Error(`whoami: ${res.status}`)
  const body = (await res.json()) as { tokenized?: string }
  if (!body.tokenized) throw new Error('whoami: no tokenized jwt')
  const jwt = body.tokenized
  return { jwt, exp: decodeExp(jwt) }
}

/**
 * The bearer for API calls and the WebSocket, refreshing the JWT when it is
 * stale. Empty string when there is nothing to send.
 */
export async function getAccessToken(): Promise<string> {
  const session = readSession()
  if (!session) return getToken() // dev mode: the static token
  const current = getToken()
  if (current !== '' && decodeExp(current) - REFRESH_MARGIN_MS > Date.now()) return current
  refreshInFlight ??= exchange(session)
    .then(({ jwt }) => {
      storeToken(jwt)
      return jwt
    })
    .finally(() => {
      refreshInFlight = undefined
    })
  return refreshInFlight
}

function storeToken(jwt: string): void {
  try {
    localStorage.setItem(TOKEN_KEY, jwt)
  } catch {
    // ignore
  }
}

// --- kratos flows -----------------------------------------------------------

type FlowMessages = { messages?: Array<{ text?: string }> }

/** The human-readable failure of a Kratos flow call. */
async function flowError(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as FlowMessages & {
      error?: { reason?: string; message?: string }
    }
    return (
      body.messages?.[0]?.text ?? body.error?.reason ?? body.error?.message ?? `HTTP ${res.status}`
    )
  } catch {
    return `HTTP ${res.status}`
  }
}

async function newFlow(kind: 'login' | 'registration'): Promise<string> {
  const res = await fetch(`${kratosBase}/self-service/${kind}/api`, { method: 'POST' })
  if (!res.ok) throw new Error(await flowError(res))
  const body = (await res.json()) as { id: string }
  return body.id
}

/**
 * Whether the Kratos registration flow is open. A closed platform turns the
 * flow endpoint off; the probe initializes a throwaway flow (GET) to tell.
 */
export async function registrationOpen(): Promise<boolean> {
  if (!kratosBase) return false
  try {
    const res = await fetch(`${kratosBase}/self-service/registration/api`)
    return res.ok
  } catch {
    return false
  }
}

/** Signs in with the e-mail and password; stores the session and a fresh JWT. */
export async function login(identifier: string, password: string): Promise<void> {
  const flow = await newFlow('login')
  const res = await fetch(`${kratosBase}/self-service/login/api?flow=${flow}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ method: 'password', identifier, password }),
  })
  if (!res.ok) throw new Error(await flowError(res))
  const body = (await res.json()) as { session_token: string }
  writeSession(body.session_token)
  await getAccessToken()
}

/** Registers an account; Kratos logs it in right away (e-mail to confirm). */
export async function register(email: string, name: string, password: string): Promise<void> {
  const flow = await newFlow('registration')
  const res = await fetch(`${kratosBase}/self-service/registration/api?flow=${flow}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ method: 'password', password, traits: { email, name } }),
  })
  if (!res.ok) throw new Error(await flowError(res))
  const body = (await res.json()) as { session_token: string }
  writeSession(body.session_token)
  await getAccessToken()
}

/** Revokes the Kratos session and forgets everything. */
export async function logout(): Promise<void> {
  const session = readSession()
  clearAll()
  if (session && kratosBase) {
    await fetch(`${kratosBase}/self-service/logout/api`, {
      method: 'DELETE',
      headers: { 'X-Session-Token': session },
    }).catch(() => undefined) // the session dies locally regardless
  }
}

function clearAll(): void {
  try {
    localStorage.removeItem(SESSION_KEY)
    localStorage.removeItem(TOKEN_KEY)
  } catch {
    // ignore
  }
}

/** Dev mode: remember a manually entered static token. */
export function setToken(token: string): void {
  clearAll()
  storeToken(token)
}

// --- losing auth ------------------------------------------------------------

export function onAuthLost(handler: () => void): void {
  lostHandler = handler
}

export function notifyAuthLost(): void {
  clearAll()
  lostHandler?.()
}
