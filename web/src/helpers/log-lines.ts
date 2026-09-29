import type { LogLine } from '@api/types'

// Pure helpers over `LogLine` buffers: identity, dedupe, client-side filter matching, export.

export interface LogLineFilter {
  role?: string[]
  machine?: string[]
  container?: string[]
  stream?: string[]
  phase?: string[]
  level?: string[]
  segment?: string
  q?: string
  start?: string
  end?: string
}

// Stable identity of a line: the server `seq`, else a content fallback.
export function logLineKey(l: LogLine): string {
  return l.seq !== undefined ? `s${l.seq}` : `${l.time}|${l.machine ?? ''}|${l.message}`
}

// Merge `next` into `prev` without duplicates. `prepend` puts the new lines in front.
export function mergeLogLines(prev: LogLine[], next: LogLine[], prepend = false): LogLine[] {
  if (!next.length) return prev
  const seen = new Set(prev.map(logLineKey))
  const fresh: LogLine[] = []
  for (const l of next) {
    const k = logLineKey(l)
    if (seen.has(k)) continue
    seen.add(k)
    fresh.push(l)
  }
  if (!fresh.length) return prev
  return sortChronological(prepend ? [...fresh, ...prev] : [...prev, ...fresh])
}

// The server hands pages newest-first for `direction=older`; the buffer is always oldest-first.
export function sortChronological(lines: LogLine[]): LogLine[] {
  // `time` has second precision, so a newest-first page must be reversed before the stable sort
  // or lines within one second would stay in reverse order.
  const first = lines[0]?.time ?? ''
  const last = lines[lines.length - 1]?.time ?? ''
  const base = first > last ? [...lines].reverse() : [...lines]
  return base.sort((a, b) => {
    const t = a.time.localeCompare(b.time)
    if (t !== 0) return t
    return (a.seq ?? 0) - (b.seq ?? 0)
  })
}

// Does a streamed line pass the active filters? (the tail is not re-fetched per event)
export function logLineMatches(l: LogLine, f: LogLineFilter): boolean {
  const inc = (arr: string[] | undefined, v: string | undefined) =>
    !arr?.length || (v !== undefined && arr.includes(v))
  return (
    inc(f.role, l.role) &&
    inc(f.machine, l.machine) &&
    inc(f.container, l.container) &&
    inc(f.stream, l.stream) &&
    inc(f.phase, l.phase) &&
    inc(f.level, l.level) &&
    (!f.segment || l.segment === f.segment) &&
    (!f.q || l.message.toLowerCase().includes(f.q.toLowerCase())) &&
    (!f.start || l.time >= f.start) &&
    (!f.end || l.time <= f.end)
  )
}

export type LogLevelTone = 'error' | 'warning' | 'info' | 'debug' | 'none'

export function logLevelTone(level: string | undefined): LogLevelTone {
  switch ((level ?? '').toLowerCase()) {
    case 'error':
    case 'err':
    case 'fatal':
    case 'panic':
    case 'critical':
      return 'error'
    case 'warn':
    case 'warning':
      return 'warning'
    case 'info':
    case 'notice':
      return 'info'
    case 'debug':
    case 'trace':
      return 'debug'
    default:
      return 'none'
  }
}

// One plain-text line for exports / clipboard.
export function formatLogLine(l: LogLine): string {
  return [l.time, l.level, l.machine, l.container, l.message]
    .filter((p) => p !== undefined && p !== '')
    .join(' ')
}

// All fields of a line as `key: value` rows (tooltip / details).
export function logLineFields(l: LogLine): [string, string][] {
  const out: [string, string][] = [['time', l.time]]
  if (l.seq !== undefined) out.push(['seq', String(l.seq)])
  for (const k of [
    'level',
    'stream',
    'role',
    'machine',
    'container',
    'phase',
    'segment',
  ] as const) {
    const v = l[k]
    if (v) out.push([k, v])
  }
  for (const [k, v] of Object.entries(l.fields ?? {})) out.push([k, v])
  return out
}
