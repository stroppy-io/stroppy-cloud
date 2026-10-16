// The metric explorer's pure parts: what a picked chart is in the URL, the unit a name implies,
// how a series is named and how lists sort.

import type { components } from '@api/schema'

type MetricName = components['schemas']['RunMetricNames']['data'][number]
type SeriesItem = components['schemas']['RunMetricSeries']['series'][number]
export type ExplorerFn = components['schemas']['MetricSeriesFn']
export type ExplorerAgg = components['schemas']['MetricSeriesAgg']

/** One chart of the explorer: a stored name and how it is drawn. */
export interface Pick {
  name: string
  fn?: ExplorerFn
  by: string[]
  agg?: ExplorerAgg
}

// `~` never appears in a Prometheus or OTLP name, and survives the URL as is.
const SEP = '~'

export function encodePick(p: Pick): string {
  return [p.name, p.fn ?? '', p.by.join(','), p.agg ?? ''].join(SEP)
}

export function decodePick(s: string): Pick | undefined {
  const [name, fn, by, agg] = s.split(SEP)
  if (!name) return undefined
  return {
    name,
    fn: fn === 'value' || fn === 'rate' ? fn : undefined,
    by: by ? by.split(',').filter(Boolean) : [],
    agg: agg === 'sum' || agg === 'avg' || agg === 'max' || agg === 'min' ? agg : undefined,
  }
}

/**
 * The display unit a name implies (Prometheus naming: base units in the suffix). A rate of a
 * byte counter is bytes per second; a rate of a seconds counter is a share of one second.
 */
export function unitOf(name: string, fn: ExplorerFn): string | undefined {
  const base = name.replace(/(_total|\.total)$/, '')
  if (/_bytes$|\.bytes$|_bytes_/.test(base)) return fn === 'rate' ? 'Bps' : 'bytes'
  if (/_seconds$|\.seconds$/.test(base)) return fn === 'rate' ? 'percentunit' : 's'
  if (/_ratio$/.test(base)) return 'percentunit'
  if (/percent$/.test(base)) return 'percent'
  if (/_ms$|_milliseconds$/.test(base)) return 'ms'
  return fn === 'rate' ? 'ops' : undefined
}

/** A series' legend: its own labels, then where it comes from. */
export function seriesLabel(s: SeriesItem, metric: string): string {
  const own = Object.entries(s.labels)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => `${k}=${v}`)
  const where = [s.machine, s.container && s.container !== s.machine ? s.container : undefined]
    .filter(Boolean)
    .join(' · ')
  const parts = [...own, where].filter(Boolean)
  return parts.length ? parts.join(', ') : metric
}

export type NameSort = 'name' | '-name' | 'series' | '-series' | 'origin'

export function sortNames(list: MetricName[], sort: NameSort): MetricName[] {
  const out = [...list]
  const byName = (a: MetricName, b: MetricName) => a.name.localeCompare(b.name)
  switch (sort) {
    case '-name':
      return out.sort((a, b) => byName(b, a))
    case 'series':
      return out.sort((a, b) => b.series - a.series || byName(a, b))
    case '-series':
      return out.sort((a, b) => a.series - b.series || byName(a, b))
    case 'origin':
      return out.sort((a, b) => a.origin.localeCompare(b.origin) || byName(a, b))
    default:
      return out.sort(byName)
  }
}

export interface NameFilter {
  q?: string
  origin?: string[]
  kind?: string[]
  role?: string[]
  machine?: string[]
  container?: string[]
}

const anyIn = (have: string[], want?: string[]) =>
  !want?.length || have.some((v) => want.includes(v))

/** Search matches the name and its containers; every other filter is an any-of. */
export function filterNames(list: MetricName[], f: NameFilter): MetricName[] {
  const q = f.q?.trim().toLowerCase()
  return list.filter(
    (m) =>
      (!q ||
        m.name.toLowerCase().includes(q) ||
        m.containers.some((c) => c.toLowerCase().includes(q))) &&
      (!f.origin?.length || f.origin.includes(m.origin)) &&
      (!f.kind?.length || f.kind.includes(m.kind)) &&
      anyIn(m.roles, f.role) &&
      anyIn(m.machines, f.machine) &&
      anyIn(m.containers, f.container)
  )
}

/** Facet values with how many names have them, most frequent first. */
export function facetOf(list: MetricName[], pick: (m: MetricName) => string[]) {
  const counts = new Map<string, number>()
  for (const m of list) for (const v of pick(m)) counts.set(v, (counts.get(v) ?? 0) + 1)
  return [...counts.entries()]
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .map(([value, count]) => ({ value, count }))
}

export type SeriesSortKey =
  | 'label'
  | 'machine'
  | 'container'
  | 'avg'
  | 'min'
  | 'max'
  | 'last'
  | 'p95'

export function sortSeries<T extends SeriesItem & { label: string }>(
  list: T[],
  key: SeriesSortKey,
  desc: boolean
): T[] {
  const value = (s: T): string | number => {
    switch (key) {
      case 'label':
        return s.label
      case 'machine':
        return s.machine ?? ''
      case 'container':
        return s.container ?? ''
      default:
        return s.aggregates?.[key] ?? Number.NEGATIVE_INFINITY
    }
  }
  return [...list].sort((a, b) => {
    const x = value(a)
    const y = value(b)
    const c =
      typeof x === 'number' && typeof y === 'number' ? x - y : String(x).localeCompare(String(y))
    return desc ? -c : c
  })
}
