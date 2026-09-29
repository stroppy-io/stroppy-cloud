import { i18n } from '@lib/i18n'

export function relativeTime(iso: string | null | undefined, now = Date.now()): string {
  if (!iso) return '—'
  const diff = (now - new Date(iso).getTime()) / 1000
  const abs = Math.abs(diff)
  const rtf = new Intl.RelativeTimeFormat(i18n.language, { numeric: 'auto' })
  const sign = diff >= 0 ? -1 : 1
  if (abs < 45) return rtf.format(sign * Math.round(abs), 'second')
  if (abs < 3600) return rtf.format(sign * Math.round(abs / 60), 'minute')
  if (abs < 86400) return rtf.format(sign * Math.round(abs / 3600), 'hour')
  if (abs < 86400 * 30) return rtf.format(sign * Math.round(abs / 86400), 'day')
  return new Date(iso).toLocaleDateString(i18n.language)
}

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString(i18n.language, { dateStyle: 'medium', timeStyle: 'short' })
}

export function formatTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleTimeString(i18n.language, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

export function formatDate(iso: string | null | undefined): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString(i18n.language, { dateStyle: 'medium' })
}
