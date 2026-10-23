// The only module that knows how the SPA authenticates.
//
// Kratos mode: the session lives in Kratos's own cookie, same-origin through
// the server's /v1 proxy. Login and registration run as BROWSER flows with
// Accept: application/json (API flows are blocked from browsers by CSRF
// protection in Kratos v26): initialize the flow, submit with the CSRF token
// of its ui.nodes, then exchange the cookie session for a short-lived JWT
// (`stroppy` tokenizer template, see deployments/kratos/) that the server
// verifies locally. Refresh happens here, silently.
//
// Dev mode (a local stand without Kratos): a static bearer token accepted
// by the server (`STROPPY_DEV_USERS`), entered on the login page.

const TOKEN_KEY = 'stroppy.token'
// Refresh when less than this is left of the JWT's 10m life.
const REFRESH_MARGIN_MS = 60_000

let kratosBase = ''
let lostHandler: (() => void) | undefined
let refreshInFlight: Promise<string> | undefined

/** Base URL of Kratos as the browser sees it, from the server's public config. */
export function setKratosBase(url: string): void {
  kratosBase = url.replace(/\/$/, '')
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

function base64url(part: string): string {
  return atob(part.replace(/-/g, '+').replace(/_/g, '/'))
}

function decodeExp(jwt: string): number {
  try {
    const payload = JSON.parse(base64url(jwt.split('.')[1] ?? '')) as { exp?: number }
    return (payload.exp ?? 0) * 1000
  } catch {
    return 0
  }
}

/**
 * The bearer for API calls and the WebSocket: the JWT of the cookie session,
 * refreshed when stale. Empty when there is no session (or dev mode's static
 * token, which is the whole answer then).
 */
export async function getAccessToken(force = false): Promise<string> {
  if (!kratosBase) return getToken() // dev mode: the static token
  const current = getToken()
  if (!force && current !== '' && decodeExp(current) - REFRESH_MARGIN_MS > Date.now()) {
    return current
  }
  refreshInFlight ??= exchange().finally(() => {
    refreshInFlight = undefined
  })
  return refreshInFlight
}

/** Cookie session → short-lived JWT; '' when there is no session. */
async function exchange(): Promise<string> {
  try {
    const res = await fetch(`${kratosBase}/sessions/whoami?tokenize_as=stroppy`)
    if (!res.ok) return ''
    const body = (await res.json()) as { tokenized?: string }
    if (!body.tokenized) return ''
    try {
      localStorage.setItem(TOKEN_KEY, body.tokenized)
    } catch {
      // ignore
    }
    return body.tokenized
  } catch {
    return ''
  }
}

/** Dev mode: remember a manually entered static token. */
export function setToken(token: string): void {
  try {
    localStorage.setItem(TOKEN_KEY, token)
  } catch {
    // ignore
  }
}

function forgetToken(): void {
  try {
    localStorage.removeItem(TOKEN_KEY)
  } catch {
    // ignore
  }
}

// --- kratos browser flows ---------------------------------------------------
//
// Every self-service flow (login, registration, verification, recovery,
// settings) is the same shape: start it (or resume it by the `flow` id Kratos
// put into a link or redirect), submit its form with the CSRF token of its
// ui.nodes, read the outcome. Kratos speaks JSON to the SPA because every
// request carries Accept: application/json.

export type FlowKind = 'login' | 'registration' | 'verification' | 'recovery' | 'settings'

interface FlowNode {
  /** The method the node belongs to: `default`, `password`, `oidc`, `code` … */
  group?: string
  attributes?: { name?: string; value?: unknown }
}

/** A Kratos UI message: a stable numeric `id` (translated by the SPA), the English text, its values. */
export type FlowMessage = {
  text?: string
  id?: number
  type?: string
  context?: Record<string, unknown>
}

export type Flow = {
  id: string
  state?: string
  ui: { action: string; nodes: FlowNode[]; messages?: FlowMessage[] }
}

/** What a submit led to: a next form, a done flow, or another page. */
export type FlowOutcome = {
  flow?: Flow
  /** `show_verification_ui` of continue_with: the verification flow to continue. */
  verificationFlow?: string
  /** A page Kratos wants the browser at (recovery → settings), as a URL. */
  redirect?: string
}

/** The human-readable failure of a Kratos flow answer. */
function flowErrorOf(body: unknown, status: number): string {
  const b = body as {
    ui?: { messages?: FlowMessage[]; nodes?: Array<{ messages?: FlowMessage[] }> }
    error?: { reason?: string; message?: string }
  } | null
  const node = (b?.ui?.nodes ?? []).flatMap((n) => n.messages ?? [])
  return (
    b?.ui?.messages?.find((m) => m.type === 'error')?.text ??
    node.map((m) => m.text).filter(Boolean)[0] ??
    b?.error?.reason ??
    b?.error?.message ??
    `HTTP ${status}`
  )
}

const JSON_HEADERS = { Accept: 'application/json' }

/** Starts a browser flow, or resumes the one Kratos referenced by `id`. */
export async function getFlow(kind: FlowKind, id?: string): Promise<Flow> {
  const url = id
    ? `${kratosBase}/self-service/${kind}/flows?id=${encodeURIComponent(id)}`
    : `${kratosBase}/self-service/${kind}/browser`
  const res = await fetch(url, { headers: JSON_HEADERS })
  const body = await res.json().catch(() => null)
  if (!res.ok) throw new FlowError(flowErrorOf(body, res.status), body)
  return body as Flow
}

/** A Kratos refusal with its body (the error id says what to do next). */
export class FlowError extends Error {
  /** Kratos error id (`session_refresh_required`, `self_service_flow_expired` …). */
  readonly id?: string
  readonly redirect?: string
  /** Messages not tied to an input (bad credentials, expired flow). */
  readonly messages: FlowMessage[]
  /** Messages per input, by the Kratos node name (`traits.email`, `password`, `code`). */
  readonly fields: Record<string, FlowMessage[]>
  constructor(message: string, body: unknown) {
    super(message)
    const b = body as {
      error?: { id?: string }
      redirect_browser_to?: string
      ui?: {
        messages?: FlowMessage[]
        nodes?: Array<{ attributes?: { name?: string }; messages?: FlowMessage[] }>
      }
    } | null
    this.id = b?.error?.id
    this.redirect = b?.redirect_browser_to
    this.messages = (b?.ui?.messages ?? []).filter((m) => m.type === 'error')
    this.fields = {}
    for (const n of b?.ui?.nodes ?? []) {
      const name = n.attributes?.name
      const errors = (n.messages ?? []).filter((m) => m.type === 'error')
      if (name && errors.length) this.fields[name] = errors
    }
  }
}

function csrfOf(flow: Flow): string {
  return nodeValue(flow, 'csrf_token')
}

/** The value Kratos pre-filled into a form node ('' when none). */
export function nodeValue(flow: Flow, name: string): string {
  for (const n of flow.ui.nodes) {
    if (n.attributes?.name === name && n.attributes.value != null) return String(n.attributes.value)
  }
  return ''
}

/** The flow's info/success messages (errors are thrown). */
export function flowMessages(flow: Flow): string[] {
  return (flow.ui.messages ?? []).filter((m) => m.type !== 'error').map((m) => m.text ?? '')
}

/** Submits the flow's form; Kratos answers with the next state of it. */
export async function submitFlow(
  flow: Flow,
  payload: Record<string, unknown>
): Promise<FlowOutcome> {
  const res = await fetch(flow.ui.action, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...JSON_HEADERS },
    body: JSON.stringify({ ...payload, csrf_token: csrfOf(flow) }),
  })
  const body = (await res.json().catch(() => null)) as {
    error?: { id?: string }
    redirect_browser_to?: string
    continue_with?: Array<{ action?: string; flow?: { id?: string } }>
    ui?: unknown
    state?: string
  } | null
  // Recovery hands over to a settings flow by asking for a page change.
  if (res.status === 422 && body?.error?.id === 'browser_location_change_required') {
    return { redirect: body.redirect_browser_to }
  }
  // A form that came back with errors is a 400 carrying the flow.
  const errorText = flowErrorOf(body, res.status)
  if (!res.ok) throw new FlowError(errorText, body)
  const verification = body?.continue_with?.find((c) => c.action === 'show_verification_ui')
  return {
    flow: body?.ui ? (body as unknown as Flow) : undefined,
    verificationFlow: verification?.flow?.id,
  }
}

/** The `flow` query parameter of a Kratos redirect URL. */
export function flowIdOf(url: string | undefined): string | undefined {
  if (!url) return undefined
  try {
    return new URL(url, window.location.origin).searchParams.get('flow') ?? undefined
  } catch {
    return undefined
  }
}

/**
 * Signs in with the e-mail and password (cookie session + a fresh JWT). With
 * `flowId` it continues that login flow — the one Kratos opened to link a
 * GitHub sign-in to the account that already has the address.
 */
export async function login(identifier: string, password: string, flowId?: string): Promise<void> {
  const flow = await getFlow('login', flowId)
  await submitFlow(flow, { method: 'password', identifier, password })
  await getAccessToken(true)
}

/** The sign-in providers a flow offers (`github` …), in Kratos order. */
export function oidcProviders(flow: Flow): string[] {
  return flow.ui.nodes
    .filter((n) => n.group === 'oidc' && n.attributes?.name === 'provider')
    .map((n) => String(n.attributes?.value ?? ''))
    .filter(Boolean)
}

/**
 * Leaves for the provider: Kratos answers the submit with the provider's
 * authorization URL, the browser goes there and comes back through Kratos
 * with a session (to `/`), or to the flow's page with its messages.
 */
export async function startOidc(flow: Flow, provider: string): Promise<void> {
  const out = await submitFlow(flow, { method: 'oidc', provider })
  if (out.redirect) window.location.assign(out.redirect)
}

/**
 * Registers an account; Kratos signs it in right away. Returns the
 * verification flow to continue on /verify (the confirmation code is mailed).
 */
export async function register(
  email: string,
  name: string,
  password: string
): Promise<string | undefined> {
  const flow = await getFlow('registration')
  const out = await submitFlow(flow, { method: 'password', password, traits: { email, name } })
  await getAccessToken(true)
  return out.verificationFlow
}

/** Verification: mails a (new) code to the address. */
export async function sendVerificationCode(flow: Flow, email: string): Promise<Flow | undefined> {
  return (await submitFlow(flow, { method: 'code', email })).flow
}

/** Verification: the code from the mail; true once the address is confirmed. */
export async function submitVerificationCode(flow: Flow, code: string): Promise<boolean> {
  const out = await submitFlow(flow, { method: 'code', code })
  if (out.flow?.state !== 'passed_challenge') {
    throw new FlowError(
      out.flow ? flowErrorOf(out.flow, 400) : 'verification failed',
      out.flow ?? null
    )
  }
  await getAccessToken(true) // the JWT now carries email_verified
  return true
}

/** Recovery: mails a code to the address (Kratos answers the same for unknown ones). */
export async function sendRecoveryCode(flow: Flow, email: string): Promise<Flow | undefined> {
  return (await submitFlow(flow, { method: 'code', email })).flow
}

/** Recovery: the code from the mail; returns the settings flow for the new password. */
export async function submitRecoveryCode(flow: Flow, code: string): Promise<string> {
  const out = await submitFlow(flow, { method: 'code', code })
  const settings = flowIdOf(out.redirect)
  if (!settings)
    throw new FlowError(out.flow ? flowErrorOf(out.flow, 400) : 'recovery failed', out.flow ?? null)
  await getAccessToken(true)
  return settings
}

/** Settings: a new password for the signed-in account. */
export async function changePassword(flow: Flow, password: string): Promise<void> {
  await submitFlow(flow, { method: 'password', password })
}

/** A Kratos user-facing error (`/auth/error?id=`): its id, HTTP status and English text. */
export async function flowErrorDetails(
  id: string
): Promise<{ id?: string; status: number; text?: string }> {
  const res = await fetch(`${kratosBase}/self-service/errors?id=${encodeURIComponent(id)}`, {
    headers: JSON_HEADERS,
  })
  const body = (await res.json().catch(() => null)) as {
    error?: { id?: string; code?: number; message?: string; reason?: string }
  } | null
  return {
    id: body?.error?.id,
    status: body?.error?.code ?? res.status,
    text: body?.error?.reason ?? body?.error?.message,
  }
}

/** The claims of the current JWT the UI cares about (kratos mode). */
export function tokenClaims(): { email?: string; emailVerified?: boolean } {
  if (!kratosBase) return {}
  try {
    const payload = JSON.parse(base64url(getToken().split('.')[1] ?? '')) as {
      email?: string
      email_verified?: boolean
    }
    return { email: payload.email, emailVerified: payload.email_verified }
  } catch {
    return {}
  }
}

/** Revokes the Kratos session and forgets the JWT. */
export async function logout(): Promise<void> {
  forgetToken()
  if (!kratosBase) return
  try {
    // Cookie sessions log out in two steps: the browser flow yields a
    // logout_url, following it revokes the session.
    const res = await fetch(`${kratosBase}/self-service/logout/browser`, {
      headers: { Accept: 'application/json' },
    })
    if (!res.ok) return
    const body = (await res.json()) as { logout_url?: string }
    if (body.logout_url) {
      // Following it revokes the session; its redirect back is not needed.
      await fetch(body.logout_url, { redirect: 'manual' })
    }
  } catch {
    // the session dies server-side on its own timescale; locally it is gone
  }
}

/**
 * The state of the Kratos registration flow for THIS browser:
 * `open` — the form works; `closed` — the platform turned sign-up off;
 * `signedin` — a session already exists (Kratos refuses to initialize a
 * registration flow for a signed-in visitor — the app is the place to be).
 */
export type RegistrationStatus = 'open' | 'closed' | 'signedin'

export async function registrationStatus(): Promise<RegistrationStatus> {
  if (!kratosBase) return 'closed'
  try {
    const res = await fetch(`${kratosBase}/self-service/registration/browser`, {
      headers: { Accept: 'application/json' },
    })
    if (res.ok) return 'open'
    const body = (await res.json().catch(() => null)) as { error?: { id?: string } } | null
    if (body?.error?.id === 'session_already_available') return 'signedin'
    return 'closed'
  } catch {
    return 'closed'
  }
}

// --- losing auth ------------------------------------------------------------

export function onAuthLost(handler: () => void): void {
  lostHandler = handler
}

export function notifyAuthLost(): void {
  forgetToken()
  lostHandler?.()
}
