import { codeText, fieldErrorsFrom, isApiError, reasonText } from '@api/errors'
import { type AnyFieldApi, revalidateLogic } from '@tanstack/react-form'

// TanStack Form collects errors as `unknown[]` (zod issues `{message}` or plain strings from
// `setErrorMap`). Return the first human message for a grafana `Field`.
// Pass `field.state.meta`: client-side errors are shown only once the field was blurred
// ("reward early, punish late"); server errors (`errorMap.onServer`) show immediately.
export interface FieldMetaLike {
  errors?: ReadonlyArray<unknown>
  isTouched?: boolean
  isBlurred?: boolean
  errorMap?: { onServer?: unknown; onSubmit?: unknown }
}

export function firstError(
  input: ReadonlyArray<unknown> | FieldMetaLike | undefined
): string | undefined {
  if (!input) return undefined
  if (Array.isArray(input)) return firstMessage(input)
  const meta = input as FieldMetaLike
  const server =
    firstMessage(toArray(meta.errorMap?.onServer)) ?? firstMessage(toArray(meta.errorMap?.onSubmit))
  if (server) return server
  if (!meta.isBlurred && !meta.isTouched) return undefined
  return firstMessage(meta.errors)
}

function toArray(v: unknown): ReadonlyArray<unknown> {
  if (v === undefined || v === null) return []
  return Array.isArray(v) ? v : [v]
}

function firstMessage(errors: ReadonlyArray<unknown> | undefined): string | undefined {
  for (const e of errors ?? []) {
    if (!e) continue
    if (typeof e === 'string') return e
    if (typeof e === 'object' && 'message' in e) {
      const m = (e as { message?: unknown }).message
      if (typeof m === 'string' && m) return m
    }
  }
  return undefined
}

export interface ServerErrors {
  form?: string
  fields: Record<string, string>
}

// Map an `ApiError` onto form fields: `validation.errors[].path` → field, the rest → form-level.
export function serverErrors(e: unknown, fallback: string): ServerErrors {
  if (!isApiError(e)) return { form: e instanceof Error ? e.message : fallback, fields: {} }
  const fields = fieldErrorsFrom(e.validation)
  const form = Object.keys(fields).length
    ? undefined
    : // Never the HTTP title ("Service Unavailable"): the reason, else the kind of failure.
      (reasonText(e) ?? (e.detail ? `${codeText(e)}: ${e.detail}` : codeText(e)))
  return { form, fields }
}

// Push an `ApiError` into a TanStack form: `validation.errors[].path` land on the fields,
// the remainder on `errorMap.onServer` (rendered next to the submit button).
// `setErrorMap`'s `onServer` slot is typed by the (absent) server validator, hence the cast.
export function applyServerErrors(
  form: { setErrorMap: (m: never) => void },
  e: unknown,
  fallback: string
): ServerErrors {
  const s = serverErrors(e, fallback)
  form.setErrorMap({ onServer: { form: s.form, fields: s.fields } } as never)
  return s
}

export function formError(v: unknown): string | undefined {
  return typeof v === 'string' && v ? v : undefined
}

// Go-style duration such as `0s`, `30m`, `2h`, `1h30m`.
export const GO_DURATION = /^(\d+h)?(\d+m)?(\d+s)?$/

export function isGoDuration(v: string): boolean {
  return v !== '' && GO_DURATION.test(v)
}

// Russian letters as the server transliterates them (tenant.Slugify).
const CYRILLIC: Record<string, string> = {
  а: 'a',
  б: 'b',
  в: 'v',
  г: 'g',
  д: 'd',
  е: 'e',
  ё: 'e',
  ж: 'zh',
  з: 'z',
  и: 'i',
  й: 'y',
  к: 'k',
  л: 'l',
  м: 'm',
  н: 'n',
  о: 'o',
  п: 'p',
  р: 'r',
  с: 's',
  т: 't',
  у: 'u',
  ф: 'f',
  х: 'kh',
  ц: 'ts',
  ч: 'ch',
  ш: 'sh',
  щ: 'shch',
  ъ: '',
  ы: 'y',
  ь: '',
  э: 'e',
  ю: 'yu',
  я: 'ya',
}

export function slugify(name: string): string {
  return [...name.toLowerCase()]
    .map((c) => CYRILLIC[c] ?? c)
    .join('')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^[^a-z]+|-$/g, '')
    .slice(0, 40)
    .replace(/-$/, '')
}

export const SLUG_RE = /^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])?$/

/**
 * Validation timing of every form ("reward early, punish late"): a field is
 * checked when it loses focus until the first submit, on every change after
 * it. A field already showing an error is re-checked as it is typed in, so
 * the error goes away the moment the value is fixed — otherwise it would
 * vanish only on the blur of the submit click, the form would shrink under
 * the pointer and swallow that click.
 */
export const REWARD_EARLY = {
  validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
  listeners: {
    onChange: ({ fieldApi }: { fieldApi: AnyFieldApi }) => {
      if (fieldApi.state.meta.errors.length) void fieldApi.validate('blur')
    },
  },
}
