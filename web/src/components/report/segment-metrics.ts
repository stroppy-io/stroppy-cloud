// What a run's result says about one segment, read the same way on the run
// page and in a shared report.

type MetricValue = { value: number; unit?: string }
export type SegmentMetrics = Record<string, MetricValue | undefined>

/** The parts of a result segment these readers use. */
export interface ResultSegment {
  name: string
  metrics?: SegmentMetrics
  /** Stroppy's native run report; its histograms carry the buckets. */
  report?: unknown
  errors?: {
    terminal_errors: number
    failed_iterations: number
    failed_queries: number
    retry_attempts: number
  }
  /** What Stroppy logged as errors, grouped (runs from after the error log). */
  error_log?: ErrorKind[]
}

/** One kind of error a segment hit. */
export interface ErrorKind {
  level: string
  message?: string
  error?: string
  class?: string
  count: number
  first_at?: string
  last_at?: string
  sql?: string
}

interface ReportErrors {
  errors?: { groups?: Array<{ class?: string; operation?: string; count?: number }> }
  metrics?: Record<
    string,
    { series?: Array<{ total?: number; attributes?: Record<string, string> }> } | undefined
  >
}

/**
 * The kinds of error a segment hit: the error log when the run has one;
 * for older runs, Stroppy's error groups (class, operation) with the
 * transactions they happened in.
 */
export function errorKinds(seg: ResultSegment): ErrorKind[] {
  if (seg.error_log?.length) return seg.error_log
  const report = seg.report as ReportErrors | undefined
  const txs = (report?.metrics?.tx_errors_total?.series ?? [])
    .filter((x) => x.total && x.attributes?.tx_name)
    .map((x) => `${x.attributes?.tx_name} ${x.attributes?.tx_action ?? ''}`.trim())
  return (report?.errors?.groups ?? [])
    .filter((g) => (g.count ?? 0) > 0)
    .map((g) => ({
      level: 'error',
      message: g.operation,
      error: txs.length ? txs.join(', ') : undefined,
      class: g.class,
      count: g.count ?? 0,
    }))
}

export interface LatencyRow {
  /** Histogram name without the `_duration` suffix: tx_total, iteration, tpcc_new_order… */
  name: string
  count?: number
  avg?: number
  p50?: number
  p95?: number
  p99?: number
}

interface Histogram {
  type?: string
  bounds?: number[]
  bucket_counts?: number[]
}

// The client's view first, then the database's, then each transaction type.
const ORDER = ['iteration', 'tx_total', 'run_query']

/**
 * Quantile q (0..1) of a histogram, interpolated linearly inside the bucket
 * that holds it, as Prometheus histogram_quantile does. Stroppy reports the
 * bucket's upper bound instead, so 100–250 ms iterations read p50 = p99 = 250.
 * Mirrors pipelines/stroppycfg ReportMetric.percentile.
 */
export function percentileOf(h: Histogram | undefined, q: number): number | undefined {
  const bounds = h?.bounds
  const counts = h?.bucket_counts
  if (!bounds?.length || counts?.length !== bounds.length + 1) return undefined
  const total = counts.reduce((a, b) => a + b, 0)
  if (!total) return undefined
  const rank = q * total
  let seen = 0
  for (let i = 0; i < counts.length; i++) {
    const n = counts[i]
    if (n === 0 || seen + n < rank) {
      seen += n
      continue
    }
    if (i === bounds.length) return bounds[bounds.length - 1]
    const lower = i > 0 ? bounds[i - 1] : 0
    return lower + ((bounds[i] - lower) * (rank - seen)) / n
  }
  return bounds[bounds.length - 1]
}

function histogramOf(report: unknown, name: string): Histogram | undefined {
  const metrics = (report as { metrics?: Record<string, Histogram> } | undefined)?.metrics
  const h = metrics?.[name]
  return h?.type === 'histogram' ? h : undefined
}

/** Every latency histogram of a segment, the load's batches left out. */
export function latencyRows(seg: ResultSegment | undefined): LatencyRow[] {
  const m = seg?.metrics
  if (!m) return []
  const names = new Set<string>()
  for (const key of Object.keys(m)) {
    const match = key.match(/^(.+)_duration_(avg|p50|p95|p99|count)$/)
    if (match && match[1] !== 'insert') names.add(match[1])
  }
  const rank = (n: string) => {
    const i = ORDER.indexOf(n)
    return i < 0 ? ORDER.length : i
  }
  return [...names]
    .sort((a, b) => rank(a) - rank(b) || a.localeCompare(b))
    .map((name) => {
      const v = (s: string) => m[`${name}_duration_${s}`]?.value
      const h = histogramOf(seg.report, `${name}_duration`)
      const p = (q: number, key: string) => percentileOf(h, q) ?? v(key)
      return {
        name,
        count: v('count'),
        avg: v('avg'),
        p50: p(0.5, 'p50'),
        p95: p(0.95, 'p95'),
        p99: p(0.99, 'p99'),
      }
    })
}

/**
 * How long the client spent per iteration outside the database: the
 * iteration minus the transaction, both averages. Undefined when either is
 * missing or the gap is noise (under 5 ms or a fifth of the iteration).
 */
export function clientWait(
  m: SegmentMetrics | undefined
): { ms: number; share: number } | undefined {
  const iteration = m?.iteration_duration_avg?.value
  const tx = m?.tx_total_duration_avg?.value
  if (!iteration || tx === undefined) return undefined
  const ms = iteration - tx
  const share = ms / iteration
  return ms >= 5 && share >= 0.2 ? { ms, share } : undefined
}

/**
 * The client iteration latency of the last segment that measured one, as
 * the run's headline shows it, read from the report's buckets when they are
 * there (runs from before interpolation report bucket bounds).
 */
export function headlineLatency(segments: ResultSegment[] | undefined): LatencyRow | undefined {
  for (const seg of [...(segments ?? [])].reverse()) {
    const row = latencyRows(seg).find((r) => r.name === 'iteration')
    if (row?.p99 !== undefined) return row
  }
  return undefined
}
