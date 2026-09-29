import { api, unwrap } from '@api/client'
import type { LaunchOverrides, LogPage, Paths, Schemas, Share } from '@api/types'
import { sortChronological } from '@helpers/log-lines'
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query'
import { keys } from './keys'

export type RunListQuery = NonNullable<Paths['/api/v1/t/{slug}/runs']['get']['parameters']['query']>
export type LogQuery = NonNullable<
  Paths['/api/v1/t/{slug}/runs/{id}/logs']['get']['parameters']['query']
>
export type MetricsQuery = NonNullable<
  Paths['/api/v1/t/{slug}/runs/{id}/metrics']['get']['parameters']['query']
>

export const runQueries = {
  list: (slug: string, q: RunListQuery = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.t(slug), 'runs', 'list', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/t/{slug}/runs', {
            params: { path: { slug }, query: { ...q, cursor: pageParam ?? undefined } },
          })
        ),
      initialPageParam: null as string | null,
      getNextPageParam: (last) => last.meta?.next_cursor ?? null,
    }),
  facets: (slug: string, q: RunListQuery = {}) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'runs', 'facets', q],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/runs:facets', { params: { path: { slug }, query: q as never } })
        ),
    }),
  detail: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'runs', 'detail', id],
      queryFn: () =>
        unwrap(api.GET('/api/v1/t/{slug}/runs/{id}', { params: { path: { slug, id } } })),
    }),
  overview: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'runs', 'overview', id],
      queryFn: () =>
        unwrap(api.GET('/api/v1/t/{slug}/runs/{id}/overview', { params: { path: { slug, id } } })),
    }),
  tree: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'runs', 'tree', id],
      queryFn: () =>
        unwrap(api.GET('/api/v1/t/{slug}/runs/{id}/tree', { params: { path: { slug, id } } })),
    }),
  events: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'runs', 'events', id],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/runs/{id}/events', {
            params: { path: { slug, id }, query: { limit: 200 } },
          })
        ),
    }),
  logs: (slug: string, id: string, q: LogQuery = {}) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'runs', 'logs', id, q],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/runs/{id}/logs', { params: { path: { slug, id }, query: q } })
        ),
    }),
  // The Logs tab buffer under a filter set: the newest page (tail), or — with `anchor` (a line
  // `seq` from a deep link) — the page ending at that line plus a page of context after it.
  // Paging and the live tail write into this same key.
  logBuffer: (slug: string, id: string, q: LogQuery, anchor?: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'runs', 'logBuffer', id, q, anchor ?? null],
      queryFn: async (): Promise<LogPage> => {
        const fetchPage = (extra: LogQuery) =>
          unwrap(
            api.GET('/api/v1/t/{slug}/runs/{id}/logs', {
              params: { path: { slug, id }, query: { ...q, ...extra } },
            })
          )
        const limit = q.limit ?? 200
        if (!anchor) {
          const tail = await fetchPage({ direction: 'older', limit })
          return { ...tail, data: sortChronological(tail.data) }
        }
        const before = await fetchPage({ cursor: anchor, direction: 'older', limit })
        // The cursor may be exclusive on either side: continue from what we actually got so
        // the anchored line itself lands in the buffer.
        const from = before.newer ?? anchor
        const after = await fetchPage({
          cursor: from,
          direction: 'newer',
          limit: Math.max(50, Math.floor(limit / 2)),
        })
        const seen = new Set(before.data.map((l) => l.seq))
        return {
          data: sortChronological([...before.data, ...after.data.filter((l) => !seen.has(l.seq))]),
          older: before.older,
          newer: after.newer ?? before.newer,
          truncated: before.truncated,
        }
      },
      staleTime: Number.POSITIVE_INFINITY,
      refetchOnWindowFocus: false,
    }),
  logFacets: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'runs', 'logFacets', id],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/runs/{id}/logs:facets', { params: { path: { slug, id } } })
        ),
    }),
  metrics: (slug: string, id: string, q: MetricsQuery = {}) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'runs', 'metrics', id, q],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/runs/{id}/metrics', {
            params: { path: { slug, id }, query: q },
          })
        ),
    }),
  artifacts: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'runs', 'artifacts', id],
      queryFn: () =>
        unwrap(api.GET('/api/v1/t/{slug}/runs/{id}/artifacts', { params: { path: { slug, id } } })),
    }),
  grafanaSession: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'runs', 'grafana', id],
      queryFn: () =>
        unwrap(
          api.POST('/api/v1/t/{slug}/runs/{id}/grafana-session', { params: { path: { slug, id } } })
        ),
      staleTime: 30 * 60_000,
    }),
  compare: (
    slug: string,
    body: { run_ids: string[]; baseline_run_id?: string; deadband_pct?: number }
  ) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'compare', body],
      queryFn: () =>
        unwrap(
          api.POST('/api/v1/t/{slug}/compare', { params: { path: { slug } }, body: body as never })
        ),
      enabled: body.run_ids.length >= 2,
    }),
}

export const runMutations = {
  patch: (slug: string, id: string, body: Schemas['RunPatch']) =>
    unwrap(api.PATCH('/api/v1/t/{slug}/runs/{id}', { params: { path: { slug, id } }, body })),
  remove: (slug: string, id: string) =>
    unwrap(api.DELETE('/api/v1/t/{slug}/runs/{id}', { params: { path: { slug, id } } })),
  cancel: (slug: string, id: string) =>
    unwrap(api.POST('/api/v1/t/{slug}/runs/{id}:cancel', { params: { path: { slug, id } } })),
  keepExtend: (slug: string, id: string, duration: string) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/runs/{id}:keep-extend', {
        params: { path: { slug, id } },
        body: { duration },
      })
    ),
  keepRelease: (slug: string, id: string) =>
    unwrap(api.POST('/api/v1/t/{slug}/runs/{id}:keep-release', { params: { path: { slug, id } } })),
  rerun: (slug: string, id: string, body: LaunchOverrides & { resume?: boolean } = {}) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/runs/{id}:rerun', {
        params: { path: { slug, id } },
        body: body as never,
      })
    ),
  saveAsTest: (
    slug: string,
    id: string,
    body: { name: string; save_database_as?: string; save_workload_as?: string }
  ) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/runs/{id}:save-as-test', { params: { path: { slug, id } }, body })
    ),
  share: (slug: string, id: string, body: Schemas['ShareCreate']) =>
    unwrap(api.POST('/api/v1/t/{slug}/runs/{id}:share', { params: { path: { slug, id } }, body })),
  shareComparison: (
    slug: string,
    body: {
      run_ids: string[]
      baseline_run_id?: string
      ttl?: string
      scope?: Share['scope']
      title?: string
    }
  ) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/compare:share', {
        params: { path: { slug } },
        body: body as never,
      })
    ),
  logsRaw: (
    slug: string,
    id: string,
    body: { query: string; start?: string; end?: string; limit?: number }
  ) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/runs/{id}/logs:raw', { params: { path: { slug, id } }, body })
    ),
  metricsRaw: (
    slug: string,
    id: string,
    body: { query: string; start: string; end: string; step?: string }
  ) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/runs/{id}/metrics:raw', { params: { path: { slug, id } }, body })
    ),
  artifactUrl: (slug: string, id: string, artifactId: string) =>
    unwrap(
      api.GET('/api/v1/t/{slug}/runs/{id}/artifacts/{artifactId}', {
        params: { path: { slug, id, artifactId } },
      })
    ),
  export: (slug: string, id: string, format: 'json' | 'md' | 'pdf' | 'csv') =>
    unwrap(
      api.GET('/api/v1/t/{slug}/runs/{id}/export', {
        params: { path: { slug, id }, query: { format } as never },
      })
    ),
  favorite: (
    slug: string,
    kind: 'run' | 'database' | 'workload' | 'test' | 'suite' | 'schedule',
    id: string,
    on: boolean
  ) =>
    on
      ? unwrap(
          api.PUT('/api/v1/t/{slug}/favorites/{kind}/{id}', {
            params: { path: { slug, kind, id } },
          })
        )
      : unwrap(
          api.DELETE('/api/v1/t/{slug}/favorites/{kind}/{id}', {
            params: { path: { slug, kind, id } },
          })
        ),
}
