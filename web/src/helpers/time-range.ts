import { dateTime, type RawTimeRange, rangeUtil, type TimeRange } from '@grafana/data'

// Telemetry time window shared by the Logs / Metrics / Events tabs. The URL carries the raw
// Grafana form (`from`/`to`): relative expressions (`now-15m`, `now`) or epoch milliseconds.
// Nothing in the URL = the run's own window (started_at → finished_at, or now while live).

export interface RawWindow {
  from?: string
  to?: string
}

export interface RunWindow {
  startedAt?: string | null
  finishedAt?: string | null
}

const EPOCH_MS = /^\d{10,}$/

// Raw string from the URL → a value Grafana's picker understands.
function normalizeRaw(v: string): string {
  if (v.startsWith('now') || EPOCH_MS.test(v)) return v
  const ms = Date.parse(v)
  return Number.isNaN(ms) ? 'now' : String(ms)
}

// The run's own window as a raw range (whole run).
export function runRawWindow(run: RunWindow): RawTimeRange {
  const from = run.startedAt ? String(new Date(run.startedAt).getTime()) : 'now-1h'
  const to = run.finishedAt ? String(new Date(run.finishedAt).getTime()) : 'now'
  return { from, to }
}

// URL raw values (or the run window when absent) → raw range.
export function rawFromSearch(search: RawWindow, run: RunWindow): RawTimeRange {
  const fallback = runRawWindow(run)
  return {
    from: search.from ? normalizeRaw(search.from) : fallback.from,
    to: search.to ? normalizeRaw(search.to) : fallback.to,
  }
}

// Raw range → resolved TimeRange for the picker and the queries (browser time zone).
export function resolveRange(raw: RawTimeRange): TimeRange {
  const bound = (v: RawTimeRange['from']) =>
    typeof v === 'string' && EPOCH_MS.test(v) ? dateTime(Number(v)) : v
  // Absolute bounds become DateTime in `raw` too, otherwise the picker prints the epoch string.
  const bounded = { from: bound(raw.from), to: bound(raw.to) }
  const r = rangeUtil.convertRawToRange(bounded, 'browser')
  return { from: r.from, to: r.to, raw: bounded }
}

// Values of a `MultiSelect` change → plain string list (undefined entries dropped).
export function selectedValues<T>(items: { value?: T }[]): T[] {
  return items.map((o) => o.value).filter((v): v is T => v !== undefined)
}

// A picked TimeRange → URL raw strings (relative stays relative, absolute becomes epoch ms).
export function rawToSearch(range: TimeRange): { from: string; to: string } {
  const enc = (v: RawTimeRange['from']) => (typeof v === 'string' ? v : String(v.valueOf()))
  return { from: enc(range.raw.from), to: enc(range.raw.to) }
}

// Does the raw range equal the run's own window? (then the URL carries nothing)
export function isRunWindow(raw: RawTimeRange, run: RunWindow): boolean {
  const own = runRawWindow(run)
  const enc = (v: RawTimeRange['from']) => (typeof v === 'string' ? v : String(v.valueOf()))
  return enc(raw.from) === own.from && enc(raw.to) === own.to
}

// ISO bounds for the API. `end` is omitted while the window ends at "now" so the server
// (and the live stream) keep extending it.
export function rangeToApi(range: TimeRange): { start: string; end?: string } {
  const toNow = range.raw.to === 'now'
  return { start: range.from.toISOString(), end: toNow ? undefined : range.to.toISOString() }
}

export function isRangeLive(range: TimeRange): boolean {
  return range.raw.to === 'now'
}

// Grafana Explore semantics: zoom out doubles the span around the center; move shifts by half.
export function zoomOutRange(range: TimeRange): TimeRange {
  const from = range.from.valueOf()
  const to = range.to.valueOf()
  const half = (to - from) / 2
  const nextFrom = from - half
  const nextTo = Math.min(to + half, Date.now())
  return resolveRange({ from: String(Math.round(nextFrom)), to: String(Math.round(nextTo)) })
}

export function moveRange(range: TimeRange, dir: -1 | 1): TimeRange {
  const from = range.from.valueOf()
  const to = range.to.valueOf()
  const step = ((to - from) / 2) * dir
  let nextFrom = from + step
  let nextTo = to + step
  const now = Date.now()
  if (nextTo > now) {
    nextTo = now
    nextFrom = now - (to - from)
  }
  return resolveRange({ from: String(Math.round(nextFrom)), to: String(Math.round(nextTo)) })
}

// Refresh picker values live in the URL as Grafana interval strings; '' = off.
export const REFRESH_INTERVALS = ['5s', '10s', '30s', '1m'] as const
export type RefreshInterval = (typeof REFRESH_INTERVALS)[number]

export function refreshToMs(value: string | undefined): number | false {
  if (!value) return false
  try {
    const ms = rangeUtil.intervalToMs(value)
    return ms > 0 ? ms : false
  } catch {
    return false
  }
}
