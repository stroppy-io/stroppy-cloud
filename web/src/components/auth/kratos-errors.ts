import { FlowError, type FlowMessage } from '@lib/auth'
import type { TFunction } from 'i18next'

// Kratos answers with English sentences; the SPA shows its own text for every
// message id it knows (`auth.kratos.<id>`, values from the message context)
// and puts it under the input the message belongs to. Unknown ids fall back
// to the Kratos text — never to a generic "something went wrong".

/** One Kratos message in the UI language. */
export function kratosText(t: TFunction, m: FlowMessage): string {
  if (m.id === undefined) return m.text ?? ''
  return t(`auth.kratos.${m.id}`, { ...(m.context ?? {}), defaultValue: m.text ?? String(m.id) })
}

/** The whole failure as one sentence (toasts, page-level alerts). */
export function kratosErrorText(t: TFunction, e: unknown): string {
  if (e instanceof FlowError) {
    const first = e.messages[0] ?? Object.values(e.fields)[0]?.[0]
    if (first) return kratosText(t, first)
    if (e.id) return t(`auth.kratosError.${e.id}`, { defaultValue: e.message })
    return e.message
  }
  if (e instanceof TypeError) return t('auth.kratosError.network')
  return e instanceof Error ? e.message : t('common.errors.generic')
}

/** The same failure as an Error, for `toast.error`. */
export function kratosError(t: TFunction, e: unknown): Error {
  return new Error(kratosErrorText(t, e))
}

/**
 * Puts a Kratos failure into a TanStack form: messages of known inputs under
 * their fields (`nodes` maps a Kratos node name to the form field), the rest
 * into the form-level error shown at the submit button.
 */
export function applyKratosErrors(
  form: { setErrorMap: (m: never) => void },
  e: unknown,
  t: TFunction,
  nodes: Record<string, string>
): void {
  const fields: Record<string, string> = {}
  const rest: string[] = []
  if (e instanceof FlowError) {
    for (const [node, messages] of Object.entries(e.fields)) {
      const text = messages.map((m) => kratosText(t, m)).join(' ')
      if (nodes[node]) fields[nodes[node]] = text
      else rest.push(text)
    }
    rest.unshift(...e.messages.map((m) => kratosText(t, m)))
  }
  const formMessage = rest[0] ?? (Object.keys(fields).length ? undefined : kratosErrorText(t, e))
  form.setErrorMap({ onServer: { form: formMessage, fields } } as never)
}
