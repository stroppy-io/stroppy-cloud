import { type ApiError, fieldErrorsFrom, isApiError } from '@api/errors'

// TanStack Form error arrays hold Standard Schema issues (`{ message }`) or plain strings (server).
export { firstError } from '@helpers/form'

export interface ServerErrorMap {
  form?: string
  fields: Record<string, string>
}

// Map an API failure onto a form: attributable issues by path, the rest at form level.
export function serverErrors(e: unknown, pathPrefix = ''): ServerErrorMap {
  if (!isApiError(e)) return { form: e instanceof Error ? e.message : String(e), fields: {} }
  const api: ApiError = e
  const all = fieldErrorsFrom(api.validation)
  const fields: Record<string, string> = {}
  for (const [k, v] of Object.entries(all)) {
    const key = pathPrefix && k.startsWith(pathPrefix) ? k.slice(pathPrefix.length) : k
    fields[key] = v
  }
  return { form: Object.keys(fields).length ? undefined : (api.detail ?? api.title), fields }
}

export interface MinimalForm {
  setErrorMap: (map: { onServer: ServerErrorMap }) => void
}

// `form` is any TanStack FormApi; its `setErrorMap` generics do not admit `onServer` without a
// server validator, so the call is widened here in one place.
export function applyServerErrors(form: unknown, e: unknown, pathPrefix = ''): void {
  ;(form as MinimalForm).setErrorMap({ onServer: serverErrors(e, pathPrefix) })
}

// Read the form-level server error out of `state.errorMap.onServer` (typed as unknown by the form).
export function serverFormError(onServer: unknown): string | undefined {
  if (!onServer || typeof onServer !== 'object')
    return typeof onServer === 'string' ? onServer : undefined
  const f = (onServer as { form?: unknown }).form
  return f ? String(f) : undefined
}
