import { isApiError } from '@api/errors'
import { i18n } from '@lib/i18n'

// Human description of any failure, for toasts and inline alerts. The title says WHAT kind of
// failure it is (by the server's stable `code`, never by parsing `detail`); the detail is the
// server's own explanation; issues are the first validation messages; the request id lets a
// support person find the log line.
export interface ErrorDescription {
  title: string
  detail?: string
  issues: string[]
  requestId?: string
  status?: number
}

const CODE_KEYS = [
  'unauthenticated',
  'forbidden',
  'not_found',
  'conflict',
  'invalid',
  'validation_failed',
  'limit_exceeded',
  'unavailable',
  'internal',
] as const

function statusKey(status: number): string {
  if (status === 401) return 'unauthenticated'
  if (status === 403) return 'forbidden'
  if (status === 404) return 'not_found'
  if (status === 409) return 'conflict'
  if (status === 422 || status === 400) return 'invalid'
  if (status === 429) return 'limit_exceeded'
  if (status === 502 || status === 503 || status === 504) return 'unavailable'
  return 'internal'
}

export function describeError(e: unknown): ErrorDescription {
  const t = i18n.t.bind(i18n)
  if (isApiError(e)) {
    const key = (CODE_KEYS as readonly string[]).includes(e.code) ? e.code : statusKey(e.status)
    const issues = (e.validation?.errors ?? [])
      .filter((v) => v.severity !== 'WARNING')
      .slice(0, 3)
      .map((v) => `${v.path ? `${v.path}: ` : ''}${v.message ?? v.code}`)
    return {
      title: t(`common.errors.codes.${key}`),
      // The server's `title` is the generic HTTP phrase; its `detail` is the useful part.
      detail: e.detail && e.detail !== e.title ? e.detail : undefined,
      issues,
      requestId: e.problem?.request_id,
      status: e.status,
    }
  }
  if (e instanceof TypeError) return { title: t('common.errors.network'), issues: [] }
  if (e instanceof Error)
    return { title: t('common.errors.generic'), detail: e.message, issues: [] }
  if (typeof e === 'string') return { title: e, issues: [] }
  return { title: t('common.errors.generic'), issues: [] }
}
