import type { Problem, ValidationResult } from './types'

// RFC 9457 problem+json as thrown by every API call that fails.
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly title: string
  readonly detail?: string
  readonly validation?: ValidationResult
  readonly problem?: Problem

  constructor(problem: Partial<Problem> & { status: number }) {
    super(problem.detail ?? problem.title ?? `HTTP ${problem.status}`)
    this.name = 'ApiError'
    this.status = problem.status
    this.code = problem.code ?? `http_${problem.status}`
    this.title = problem.title ?? `HTTP ${problem.status}`
    this.detail = problem.detail
    this.validation = problem.validation
    this.problem = problem as Problem
  }
}

export async function toApiError(res: Response): Promise<ApiError> {
  let body: unknown
  try {
    body = await res.clone().json()
  } catch {
    body = undefined
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

// Map schemapb validation errors to a flat `{ 'a.b[0].c': message }` map for forms.
// Server paths are dotted (`params.replicas`); slashes and bracket indexes are normalised.
export function fieldErrorsFrom(v?: ValidationResult | null): Record<string, string> {
  const out: Record<string, string> = {}
  for (const e of v?.errors ?? []) {
    if (e.severity === 'WARNING') continue
    const path = normalizePath(e.path)
    if (!out[path]) out[path] = e.message ?? `${e.code}${e.constraint ? ` (${e.constraint})` : ''}`
  }
  return out
}

export function normalizePath(p: string): string {
  return p
    .replace(/^\/+/, '')
    .replace(/\//g, '.')
    .replace(/\.(\d+)(?=\.|$)/g, '[$1]')
}
