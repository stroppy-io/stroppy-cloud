import type { Run } from '@api/types'
import type { RunLive } from './store'
import { iso } from './util'

// Live `Run.summary` fields shared by the seed and the simulation: the workload plan
// (segment budgets), `expected_finish_at`, a live headline and the QPS sparkline.

const UNIT_MS: Record<string, number> = { h: 3_600_000, m: 60_000, s: 1000, ms: 1, us: 1e-3 }

// Go `time.ParseDuration` subset ("1h30m", "10m", "45s", "500ms"); undefined when unparsable.
export function parseGoDuration(v: unknown): number | undefined {
  if (typeof v !== 'string' || !v.trim()) return undefined
  const parts = [...v.trim().matchAll(/(\d+(?:\.\d+)?)(h|ms|us|m|s)/g)]
  if (!parts.length || parts.map((p) => p[0]).join('') !== v.trim()) return undefined
  return parts.reduce((ms, [, n, unit]) => ms + Number(n) * UNIT_MS[unit], 0)
}

interface SegmentPlan {
  name: string
  ms: number
}

// Every segment's warmup + duration, as the server's `segmentBudget`. Undefined when a segment
// has no bounded length (iterations executor), so the finish is unpredictable.
export function workloadPlan(run: Run): SegmentPlan[] | undefined {
  const segments = (run.snapshot.workload.segments ?? []) as {
    name?: string
    warmup?: string
    run?: { duration?: string }
  }[]
  if (!segments.length) return undefined
  const plan: SegmentPlan[] = []
  for (const [i, s] of segments.entries()) {
    const duration = parseGoDuration(s.run?.duration)
    if (!duration) return undefined
    const warmup = s.warmup ? parseGoDuration(s.warmup) : 0
    if (warmup === undefined) return undefined
    plan.push({ name: s.name ?? `segment-${i + 1}`, ms: warmup + duration })
  }
  return plan
}

// Per-segment budgets the simulation runs on: the plan when bounded, else `fallbackMs` split
// evenly (the simulation still has to end an iterations segment somewhere).
export function segmentBudgets(run: Run, fallbackMs: number): number[] {
  const plan = workloadPlan(run)
  if (plan) return plan.map((s) => s.ms)
  const n = Math.max(1, run.snapshot.workload.segments?.length ?? 1)
  return Array.from({ length: n }, () => fallbackMs / n)
}

export interface SegmentChange {
  name: string
  status: 'running' | 'completed'
}

// Walks the workload segments in order from the workload phase start and puts each one in the
// state `nowMs` implies (completed / running / pending), keeping times already recorded.
// Returns the transitions so the simulation can emit events; sets `summary.segment`.
export function progressSegments(
  run: Run,
  live: RunLive,
  nowMs: number,
  budgets: number[]
): SegmentChange[] {
  const phase = live.overview.phases.find((p) => p.id === 'workload')
  const segments = live.overview.workload_segments ?? []
  if (!phase?.started_at) return []
  const changes: SegmentChange[] = []
  let cursor = Date.parse(phase.started_at)
  let current: string | undefined
  let reached = true
  for (const [i, s] of segments.entries()) {
    const was = s.status
    if (!reached) {
      s.status = 'pending'
      s.started_at = null
      s.finished_at = null
      continue
    }
    const start = was === 'running' || was === 'completed' ? Date.parse(s.started_at ?? '') : cursor
    const end = start + (budgets[i] ?? 0)
    if (nowMs < start) {
      s.status = 'pending'
      s.started_at = null
      s.finished_at = null
      reached = false
    } else if (nowMs >= end) {
      s.status = 'completed'
      s.started_at = iso(start)
      s.finished_at = s.finished_at ?? iso(end)
      cursor = Date.parse(s.finished_at)
      if (was !== 'running' && was !== 'completed')
        changes.push({ name: s.name, status: 'running' })
      if (was !== 'completed') changes.push({ name: s.name, status: 'completed' })
    } else {
      s.status = 'running'
      s.started_at = iso(start)
      s.finished_at = null
      current = s.name
      reached = false
      if (was !== 'running') changes.push({ name: s.name, status: 'running' })
    }
  }
  run.summary = { ...run.summary, segment: current }
  return changes
}

// Mirrors `run.ExpectedFinishAt` (internal/domain/run/headline.go): the workload phase start plus
// every budget, re-anchored on the last finished segment's finish or the running one's start.
export function expectedFinishAt(run: Run, live: RunLive): string | undefined {
  if (['completed', 'failed', 'cancelled'].includes(run.status)) return undefined
  const phase = live.overview.phases.find((p) => p.id === 'workload')
  const plan = workloadPlan(run)
  if (!phase?.started_at || !plan) return undefined
  let anchor = Date.parse(phase.started_at)
  let next = 0
  for (const [i, s] of (live.overview.workload_segments ?? []).entries()) {
    if (i >= plan.length) break
    if (s.finished_at && i + 1 >= next) {
      anchor = Date.parse(s.finished_at)
      next = i + 1
    } else if (s.started_at && !s.finished_at && i >= next) {
      anchor = Date.parse(s.started_at)
      next = i
    }
  }
  return iso(plan.slice(next).reduce((at, s) => at + s.ms, anchor))
}

// About `n` points in time order: mean per equal time bucket, empty buckets skipped. Works for
// the seed's sparse curves and the simulation's 1 Hz samples alike.
export function downsample(points: number[][], n = 30): { t: number; v: number }[] {
  if (points.length <= n) return points.map(([t, v]) => ({ t, v }))
  const t0 = points[0][0]
  const span = Math.max(1, points[points.length - 1][0] - t0)
  const buckets: { t: number; sum: number; count: number }[] = []
  for (const [t, v] of points) {
    const i = Math.min(n - 1, Math.floor(((t - t0) / span) * n))
    buckets[i] ??= { t, sum: 0, count: 0 }
    buckets[i].t = t
    buckets[i].sum += v
    buckets[i].count += 1
  }
  return buckets.filter(Boolean).map((b) => ({ t: b.t, v: b.sum / b.count }))
}

const tailMean = (points: number[][] | undefined, n = 10): number | undefined => {
  if (!points?.length) return undefined
  const tail = points.slice(-n)
  return tail.reduce((s, [, v]) => s + v, 0) / tail.length
}

const round1 = (v: number) => Math.round(v * 10) / 10

// Refreshes a non-terminal run's live summary from its live record. The headline and sparkline
// appear once the workload produced samples; `expected_finish_at` once the workload started.
export function syncLiveSummary(run: Run, live: RunLive): void {
  if (['completed', 'failed', 'cancelled'].includes(run.status)) return
  const summary = { ...run.summary }
  const finish = expectedFinishAt(run, live)
  if (finish) summary.expected_finish_at = finish
  else delete summary.expected_finish_at
  const workloadStarted = live.overview.phases.some((p) => p.id === 'workload' && p.started_at)
  const w = (key: string) => workloadWindow(live.metrics[key] ?? [], live)
  const qpsPoints = workloadStarted ? w('queries_per_second') : []
  if (qpsPoints.length) {
    summary.headline = {
      ...summary.headline,
      qps: Math.round(tailMean(qpsPoints) ?? 0),
      tps: Math.round(tailMean(w('tps')) ?? 0),
      latency_p50_ms: round1(tailMean(w('latency_p50_ms')) ?? 0),
      latency_p95_ms: round1(tailMean(w('latency_p95_ms')) ?? 0),
      latency_p99_ms: round1(tailMean(w('latency_p99_ms')) ?? 0),
      errors: w('errors').reduce((s, [, v]) => s + v, 0),
    }
    summary.qps_series = downsample(qpsPoints)
  } else {
    delete summary.headline
    delete summary.qps_series
  }
  run.summary = summary
}

// Samples inside the workload phase only (the seed's curves cover it; the simulation keeps
// appending while the phase runs).
export function workloadWindow(points: number[][], live: RunLive): number[][] {
  const phase = live.overview.phases.find((p) => p.id === 'workload')
  const from = phase?.started_at ? Date.parse(phase.started_at) : 0
  const to = phase?.finished_at ? Date.parse(phase.finished_at) : Number.POSITIVE_INFINITY
  return points.filter(([t]) => t >= from && t <= to)
}
