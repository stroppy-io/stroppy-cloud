import type { Run } from '@api/types'

// Run list ordering as the server's `sort` enum (listRuns / listTestRuns / adminListRuns).
// `default`: favorites first (when the caller's favorites are given), then live runs, then the
// newest — `order` is ignored for it. Missing values sort last in either direction; `errors`
// counts a run without errors as 0.

const LIVE = new Set(['pending', 'running', 'cancelling'])

type Key = string | number | undefined | null

function durationSec(r: Run): number | undefined {
  if (!r.started_at) return undefined
  const end = r.finished_at ? Date.parse(r.finished_at) : Date.now()
  return (end - Date.parse(r.started_at)) / 1000
}

const KEYS: Record<string, (r: Run & { tenant?: { slug?: string } }) => Key> = {
  started_at: (r) => r.started_at,
  finished_at: (r) => r.finished_at,
  duration: durationSec,
  tps: (r) => r.summary?.headline?.tps,
  qps: (r) => r.summary?.headline?.qps,
  p50: (r) => r.summary?.headline?.latency_p50_ms,
  p99: (r) => r.summary?.headline?.latency_p99_ms,
  errors: (r) => r.summary?.headline?.errors ?? 0,
  status: (r) => r.status,
  name: (r) => r.name.toLowerCase(),
  db_kind: (r) => r.summary?.db_kind,
  workload: (r) => r.summary?.workload_name?.toLowerCase(),
  topology: (r) => r.summary?.topology_label,
  provider: (r) => r.summary?.provider_profile?.name ?? r.snapshot.provider_profile.name,
  trigger: (r) => r.trigger,
  author: (r) => r.author.display_name?.toLowerCase(),
  created_at: (r) => r.created_at,
  // Runs carry no updated_at; the latest lifecycle stamp stands in.
  updated_at: (r) => r.finished_at ?? r.started_at ?? r.created_at,
  tenant: (r) => r.tenant?.slug,
}

function compare(a: Key, b: Key, dir: 1 | -1): number {
  const missingA = a === undefined || a === null || a === ''
  const missingB = b === undefined || b === null || b === ''
  if (missingA || missingB) return missingA === missingB ? 0 : missingA ? 1 : -1
  if (a === b) return 0
  return (a < b ? -1 : 1) * dir
}

export function sortRuns<R extends Run & { tenant?: { slug?: string } }>(
  runs: R[],
  sort: string | undefined,
  order: 'asc' | 'desc',
  isFavorite: (r: R) => boolean = () => false
): R[] {
  const key = sort ? KEYS[sort] : undefined
  if (!key) {
    // `default` (and unknown keys): favorites → live → newest.
    return [...runs].sort(
      (a, b) =>
        Number(isFavorite(b)) - Number(isFavorite(a)) ||
        Number(LIVE.has(b.status)) - Number(LIVE.has(a.status)) ||
        compare(a.created_at, b.created_at, -1)
    )
  }
  const dir = order === 'asc' ? 1 : -1
  return [...runs].sort(
    (a, b) => compare(key(a), key(b), dir) || compare(a.created_at, b.created_at, -1)
  )
}
