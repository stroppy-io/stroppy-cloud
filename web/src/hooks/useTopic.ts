import { getWs } from '@api/ws'
import { type QueryKey, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'

export interface UseTopicOptions<T, Q> {
  topic: string
  queryKey: QueryKey
  enabled?: boolean
  cursor?: string | null
  // Merge an event payload into the cached query data. Default: replace.
  merge?: (prev: Q | undefined, payload: T) => Q
  // Coalesce bursts (logs/metrics) into one cache write per `batchMs`.
  batchMs?: number
}

// Subscribes to a WS topic and writes events into the react-query cache under the same
// key the REST query uses, so consumers read one `useQuery` and never see the socket.
export function useTopic<T, Q = T>({
  topic,
  queryKey,
  enabled = true,
  cursor,
  merge,
  batchMs = 0,
}: UseTopicOptions<T, Q>): void {
  const qc = useQueryClient()
  const keyRef = useRef(queryKey)
  keyRef.current = queryKey
  const mergeRef = useRef(merge)
  mergeRef.current = merge

  useEffect(() => {
    if (!enabled) return
    let unsub: (() => void) | undefined
    let cancelled = false
    let pending: T[] = []
    let timer: number | undefined
    const flush = () => {
      timer = undefined
      const batch = pending
      pending = []
      if (!batch.length) return
      qc.setQueryData<Q>(keyRef.current, (prev) => {
        let next = prev
        for (const p of batch)
          next = mergeRef.current ? mergeRef.current(next, p) : (p as unknown as Q)
        return next
      })
    }
    void getWs().then((ws) => {
      if (cancelled) return
      unsub = ws.subscribe<T>({
        topic,
        cursor,
        onEvent: (payload) => {
          pending.push(payload)
          if (batchMs > 0) {
            if (timer === undefined) timer = window.setTimeout(flush, batchMs)
          } else flush()
        },
      })
    })
    return () => {
      cancelled = true
      if (timer !== undefined) window.clearTimeout(timer)
      unsub?.()
    }
  }, [topic, enabled, cursor, batchMs, qc])
}
