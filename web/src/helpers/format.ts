export function formatBytes(bytes: number | undefined | null, digits = 1): string {
  if (bytes === undefined || bytes === null || Number.isNaN(bytes)) return '—'
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.min(sizes.length - 1, Math.floor(Math.log(Math.abs(bytes)) / Math.log(k)))
  return `${Number.parseFloat((bytes / k ** i).toFixed(digits))} ${sizes[i]}`
}

export function formatNumber(n: number | undefined | null, digits = 0): string {
  if (n === undefined || n === null || Number.isNaN(n)) return '—'
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: digits }).format(n)
}

export function formatCompact(n: number | undefined | null): string {
  if (n === undefined || n === null || Number.isNaN(n)) return '—'
  return new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 }).format(
    n
  )
}

export function formatPercent(n: number | undefined | null, digits = 1): string {
  if (n === undefined || n === null || Number.isNaN(n)) return '—'
  return `${n.toFixed(digits)}%`
}

// Go duration string ("1h2m3s", "90s", "250ms") or seconds → seconds; undefined when unparsable.
export function durationSeconds(input: string | number | null | undefined): number | undefined {
  if (input === null || input === undefined || input === '') return undefined
  if (typeof input === 'number') return Number.isNaN(input) ? undefined : input
  const m = input.match(/(\d+(?:\.\d+)?)(ms|h|m|s)/g)
  if (!m) return undefined
  let sec = 0
  for (const part of m) {
    const [, v, u] = part.match(/(\d+(?:\.\d+)?)(ms|h|m|s)/) ?? []
    const n = Number(v)
    sec += u === 'h' ? n * 3600 : u === 'm' ? n * 60 : u === 's' ? n : n / 1000
  }
  return sec
}

// Go duration string or seconds → human string ("1h 2m", "3m 20s", "45s").
export function formatDuration(input: string | number | null | undefined): string {
  const raw = durationSeconds(input)
  if (raw === undefined) return typeof input === 'string' && input ? input : '—'
  const sec = Math.round(raw)
  const h = Math.floor(sec / 3600)
  const mi = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  if (h) return mi ? `${h}h ${mi}m` : `${h}h`
  if (mi) return s ? `${mi}m ${s}s` : `${mi}m`
  return `${s}s`
}

export function elapsedSince(iso: string | null | undefined, until?: string | null): number {
  if (!iso) return 0
  const end = until ? new Date(until).getTime() : Date.now()
  return Math.max(0, (end - new Date(iso).getTime()) / 1000)
}

export function formatMetric(value: number | undefined | null, unit?: string): string {
  if (value === undefined || value === null || Number.isNaN(value)) return '—'
  const u = unit === 'bytes/s' ? 'Bps' : unit === '%' ? 'percent' : unit === '1/s' ? 'ops' : unit
  switch (u) {
    case 'ms':
      return value >= 1000
        ? `${(value / 1000).toFixed(2)} s`
        : `${value.toFixed(value < 10 ? 1 : 0)} ms`
    case 'percent':
      return formatPercent(value, value < 10 ? 2 : 1)
    case 'Bps':
      return `${formatBytes(value)}/s`
    case 'bytes':
      return formatBytes(value)
    case 'tps':
    case 'ops':
      return `${formatNumber(value, value < 100 ? 1 : 0)} ${unit}`
    case 'count':
      return formatNumber(value)
    default:
      return u ? `${formatNumber(value, 2)} ${u}` : formatNumber(value, 2)
  }
}

export function truncateMiddle(s: string, max = 24): string {
  if (s.length <= max) return s
  const half = Math.floor((max - 1) / 2)
  return `${s.slice(0, half)}…${s.slice(-half)}`
}

export function shortId(id: string): string {
  return id.length > 12 ? id.slice(0, 8) : id
}
