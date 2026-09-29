import { runQueries } from '@api/queries/runs'
import type { Run } from '@api/types'
import { isTerminal } from '@helpers/run-status'
import { useQuery } from '@tanstack/react-query'
import { useTopic } from './useTopic'

// Run detail with live updates: WS `run/{id}` writes into the same cache key; polling stays as a
// fallback until the run reaches a terminal status, then both stop.
export function useRun(slug: string, id: string) {
  const query = useQuery({
    ...runQueries.detail(slug, id),
    refetchInterval: (q) => (isTerminal(q.state.data?.status) ? false : 10_000),
  })
  useTopic<Run, Run>({
    topic: `run/${id}`,
    queryKey: runQueries.detail(slug, id).queryKey,
    enabled: !!query.data && !isTerminal(query.data.status),
    merge: (prev, payload) => ({
      ...prev,
      ...payload,
      // the stream carries the bare run; keep the per-user/aggregated fields from REST
      is_favorite: prev?.is_favorite ?? payload.is_favorite,
      shares: prev?.shares ?? payload.shares,
    }),
  })
  return query
}
