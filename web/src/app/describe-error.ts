import { codeText, isApiError, issueText, reasonText } from '@api/errors'
import { i18n } from '@lib/i18n'

// Human description of any failure, for toasts and inline alerts. The title says WHAT kind of
// failure it is: the server's `reason` in the user's language, else its stable `code`, never by
// parsing `detail` (shown only when there is no translated reason); issues are the first validation messages; the request id lets a
// support person find the log line.
export interface ErrorDescription {
  title: string
  detail?: string
  issues: string[]
  requestId?: string
  status?: number
}

export function describeError(e: unknown): ErrorDescription {
  const t = i18n.t.bind(i18n)
  if (isApiError(e)) {
    const issues = (e.validation?.errors ?? [])
      .filter((v) => v.severity !== 'WARNING')
      .slice(0, 3)
      .map((v) => `${v.path ? `${v.path}: ` : ''}${issueText(v)}`)
    // The reason says WHY in the user's language; without one the server's English
    // `detail` is the only explanation there is.
    const reason = reasonText(e)
    return {
      title: reason ?? codeText(e),
      detail: reason ? undefined : e.detail && e.detail !== e.title ? e.detail : undefined,
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
