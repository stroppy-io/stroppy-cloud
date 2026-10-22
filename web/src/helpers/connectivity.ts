import type { ConnectivityPair, RunConnectivity } from '@api/types'

export type ConnectivityTone = 'ok' | 'warning' | 'error'

// The block's verdict: a failed path the cluster or the workload needs stopped the run (error);
// failed side ports only warn; otherwise everything answered.
export function connectivityTone(
  c: Pick<RunConnectivity, 'failed' | 'critical'>
): ConnectivityTone {
  if (c.critical > 0) return 'error'
  if (c.failed > 0) return 'warning'
  return 'ok'
}

// Rows to show: failed paths (critical first), all paths when asked.
export function connectivityRows(c: RunConnectivity, showAll: boolean): ConnectivityPair[] {
  const rank = (p: ConnectivityPair) => (p.outcome === 'ok' ? 2 : p.critical ? 0 : 1)
  const rows = [...c.pairs].sort((a, b) => rank(a) - rank(b))
  return showAll ? rows : rows.filter((p) => p.outcome !== 'ok')
}
