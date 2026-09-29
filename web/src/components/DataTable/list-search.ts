import { z } from 'zod'

// Shared pieces of list-route search schemas: page size, auto-refresh interval, sort order.
// Every list keeps its whole state in the URL (§6 of web/AGENTS.md).

export const PAGE_SIZES = [25, 50, 100] as const
export type PageSize = (typeof PAGE_SIZES)[number]
export const DEFAULT_PAGE_SIZE: PageSize = 50

export const REFRESH_INTERVALS = ['off', '5s', '15s', '30s', '1m'] as const
export type RefreshInterval = (typeof REFRESH_INTERVALS)[number]

export const REFRESH_MS: Record<RefreshInterval, number> = {
  off: 0,
  '5s': 5_000,
  '15s': 15_000,
  '30s': 30_000,
  '1m': 60_000,
}

export const orderSchema = z.enum(['asc', 'desc'])

export const sizeSchema = z
  .union([z.literal(25), z.literal(50), z.literal(100)])
  .default(DEFAULT_PAGE_SIZE)
  .catch(DEFAULT_PAGE_SIZE)

export function refreshSchema(def: RefreshInterval) {
  return z.enum(REFRESH_INTERVALS).default(def).catch(def)
}

// Optional string list (`?status=a&status=b`), invalid → absent.
export const csvSchema = z.array(z.string()).optional().catch(undefined)

// Free text and the `name` column filter both hit the server `search` param.
export function joinSearch(...parts: (string | undefined)[]): string | undefined {
  const s = parts
    .map((p) => p?.trim() ?? '')
    .filter(Boolean)
    .join(' ')
  return s || undefined
}

// Number of seconds → Go duration string accepted by `duration_min/max`.
export function secondsToDuration(sec: number | undefined): string | undefined {
  return sec === undefined ? undefined : `${Math.max(0, Math.round(sec))}s`
}

// The author checklist is radio-like (`single`): the server `author` param takes one id.
export function authorParam(ids: string[] | undefined): string | undefined {
  return ids?.[0]
}
