import { i18n } from '@lib/i18n'
import type { Problem, ValidationResult } from './types'

// RFC 9457 problem+json as thrown by every API call that fails.
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly title: string
  readonly detail?: string
  readonly validation?: ValidationResult
  readonly problem?: Problem
  /** Stable key of why it failed (`tenant_slug_taken`); its text is ours, see `errorText`. */
  readonly reason?: string
  readonly params: Record<string, string>

  constructor(problem: Partial<Problem> & { status: number }) {
    super(problem.detail ?? problem.title ?? `HTTP ${problem.status}`)
    this.name = 'ApiError'
    this.status = problem.status
    this.code = problem.code ?? `http_${problem.status}`
    this.title = problem.title ?? `HTTP ${problem.status}`
    this.detail = problem.detail
    this.validation = problem.validation
    this.problem = problem as Problem
    this.reason = problem.reason
    this.params = (problem.params ?? {}) as Record<string, string>
  }
}

/**
 * The ApiError of a failed response. `parsed` is the body openapi-fetch has
 * already read (the response stream is consumed by then); a raw fetch passes
 * nothing and the body is read here.
 */
export async function toApiError(res: Response, parsed?: unknown): Promise<ApiError> {
  let body: unknown = parsed
  if (body === undefined) {
    try {
      body = await res.clone().json()
    } catch {
      body = undefined
    }
  }
  if (body && typeof body === 'object' && 'status' in body) {
    return new ApiError(body as Problem)
  }
  return new ApiError({
    status: res.status,
    title: res.statusText || `HTTP ${res.status}`,
    code: `http_${res.status}`,
  })
}

export function isApiError(e: unknown): e is ApiError {
  return e instanceof ApiError
}

/**
 * The UI text of a failure's reason (`common.errors.reasons.<reason>`, values from
 * `params`), or undefined when the reason is absent or has no text yet — callers then
 * fall back to the server's English `detail`.
 */
export function reasonText(e: ApiError): string | undefined {
  if (!e.reason) return undefined
  const key = `common.errors.reasons.${e.reason}`
  return i18n.exists(key) ? i18n.t(key, e.params) : undefined
}

const CODES = new Set([
  'unauthenticated',
  'forbidden',
  'not_found',
  'conflict',
  'invalid',
  'validation_failed',
  'limit_exceeded',
  'unavailable',
  'internal',
])

function statusCode(status: number): string {
  if (status === 401) return 'unauthenticated'
  if (status === 403) return 'forbidden'
  if (status === 404) return 'not_found'
  if (status === 409) return 'conflict'
  if (status === 422 || status === 400) return 'invalid'
  if (status === 429) return 'limit_exceeded'
  if (status === 502 || status === 503 || status === 504) return 'unavailable'
  return 'internal'
}

/** What KIND of failure it is, in the UI language (by the stable `code`, else the status). */
export function codeText(e: ApiError): string {
  const code = CODES.has(e.code) ? e.code : statusCode(e.status)
  return i18n.t(`common.errors.codes.${code}`)
}

/** The best one-line UI text of a failure: its reason, else its kind. */
export function errorText(e: ApiError): string {
  return reasonText(e) ?? codeText(e)
}

type FieldIssue = NonNullable<ValidationResult['errors']>[number]

/**
 * One validation issue as UI text: schemapb checks (`ERROR_CODE_GTE_VIOLATED`) by their
 * code with the constraint; domain checks (fit, resources) carry numbers in their own
 * message, which is shown as is.
 */
export function issueText(v: FieldIssue): string {
  const code = v.code.replace(/^ERROR_CODE_/, '').toLowerCase()
  const key = `common.errors.checks.${code}`
  if (v.code.startsWith('ERROR_CODE_') && i18n.exists(key)) {
    return i18n.t(key, { constraint: v.constraint ?? '' })
  }
  return v.message ?? `${v.code}${v.constraint ? ` (${v.constraint})` : ''}`
}

// Map validation errors to a flat `{ 'a.b[0].c': message }` map for forms.
// Server paths are dotted (`params.replicas`); slashes and bracket indexes are normalised.
export function fieldErrorsFrom(v?: ValidationResult | null): Record<string, string> {
  const out: Record<string, string> = {}
  for (const e of v?.errors ?? []) {
    if (e.severity === 'WARNING') continue
    const path = normalizePath(e.path)
    if (!out[path]) out[path] = issueText(e)
  }
  return out
}

export function normalizePath(p: string): string {
  return p
    .replace(/^\/+/, '')
    .replace(/\//g, '.')
    .replace(/\.(\d+)(?=\.|$)/g, '[$1]')
}
