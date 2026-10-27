import { api, unwrap } from '@api/client'
import type { Example, LaunchOverrides, Paths, Schemas, Share } from '@api/types'
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query'
import { keys } from './keys'

export type RatingPage = Schemas['RatingPage']
export type ShareSnapshot = Schemas['ShareSnapshot']
export type GrafanaSession = Schemas['GrafanaSession']
export type ExportFormat = 'md' | 'json' | 'csv' | 'pdf'

export type RatingQuery = NonNullable<
  Paths['/api/v1/t/{slug}/rating']['get']['parameters']['query']
>
export type ShareListQuery = NonNullable<
  Paths['/api/v1/t/{slug}/shares']['get']['parameters']['query']
>

// ---------------------------------------------------------------------------------------------

export const dashboardQueries = {
  get: (slug: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'dashboard'],
      queryFn: () => unwrap(api.GET('/api/v1/t/{slug}/dashboard', { params: { path: { slug } } })),
      refetchInterval: 15_000,
    }),
}

// ---------------------------------------------------------------------------------------------

// The rating's list filters are `style: form, explode: false` in the contract
// (`kind=postgres,mysql`); the server rejects the repeated-key form.
const commaArrays = { array: { style: 'form', explode: false } } as const

export const ratingQueries = {
  tenant: (slug: string, q: RatingQuery = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.t(slug), 'rating', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/t/{slug}/rating', {
            params: { path: { slug }, query: { ...q, cursor: pageParam ?? undefined } },
            querySerializer: commaArrays,
          })
        ),
      initialPageParam: null as string | null,
      getNextPageParam: (last) => last.meta?.next_cursor ?? null,
    }),
  global: (q: RatingQuery = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.public(), 'rating', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/public/rating', {
            params: { query: { ...q, cursor: pageParam ?? undefined } },
            querySerializer: commaArrays,
          })
        ),
      initialPageParam: null as string | null,
      getNextPageParam: (last) => last.meta?.next_cursor ?? null,
    }),
}

// ---------------------------------------------------------------------------------------------

export const shareQueries = {
  list: (slug: string, q: ShareListQuery = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.t(slug), 'shares', 'list', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/t/{slug}/shares', {
            params: { path: { slug }, query: { ...q, cursor: pageParam ?? undefined, limit: 50 } },
          })
        ),
      initialPageParam: null as string | null,
      getNextPageParam: (last) => last.meta?.next_cursor ?? null,
    }),
  detail: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'shares', 'detail', id],
      queryFn: () =>
        unwrap(api.GET('/api/v1/t/{slug}/shares/{id}', { params: { path: { slug, id } } })),
    }),
}

export const shareMutations = {
  patch: (
    slug: string,
    id: string,
    body: { ttl?: string | null; scope?: Share['scope']; title?: string | null }
  ) =>
    unwrap(
      api.PATCH('/api/v1/t/{slug}/shares/{id}', {
        params: { path: { slug, id } },
        body: body as never,
      })
    ),
  revoke: (slug: string, id: string) =>
    unwrap(api.DELETE('/api/v1/t/{slug}/shares/{id}', { params: { path: { slug, id } } })),
  rebuild: (slug: string, id: string) =>
    unwrap(api.POST('/api/v1/t/{slug}/shares/{id}:rebuild', { params: { path: { slug, id } } })),
}

// ---------------------------------------------------------------------------------------------

export const publicQueries = {
  share: (token: string) =>
    queryOptions({
      queryKey: [...keys.public(), 'share', token],
      queryFn: () =>
        unwrap(api.GET('/api/v1/public/share/{token}', { params: { path: { token } } })),
      staleTime: 5 * 60_000,
      retry: false,
    }),
  shareMetrics: (token: string, q: { start?: string; end?: string } = {}, enabled = true) =>
    queryOptions({
      queryKey: [...keys.public(), 'share', token, 'metrics', q],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/public/share/{token}/metrics', {
            params: { path: { token }, query: q },
          })
        ),
      staleTime: 5 * 60_000,
      enabled,
    }),
  shareMetricNames: (token: string, q: { start?: string; end?: string } = {}) =>
    queryOptions({
      queryKey: [...keys.public(), 'share', token, 'metricNames', q],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/public/share/{token}/metrics:names', {
            params: { path: { token }, query: q },
          })
        ),
      staleTime: 5 * 60_000,
    }),
  shareMetricSeries: (
    token: string,
    q: Paths['/api/v1/public/share/{token}/metrics:series']['get']['parameters']['query']
  ) =>
    queryOptions({
      queryKey: [...keys.public(), 'share', token, 'metricSeries', q],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/public/share/{token}/metrics:series', {
            params: { path: { token }, query: q },
          })
        ),
      staleTime: 5 * 60_000,
    }),
  grafanaSession: (token: string, enabled = true) =>
    queryOptions({
      queryKey: [...keys.public(), 'share', token, 'grafana'],
      queryFn: () =>
        unwrap(
          api.POST('/api/v1/public/share/{token}/grafana-session', { params: { path: { token } } })
        ),
      staleTime: 30 * 60_000,
      enabled,
    }),
}

export const publicMutations = {
  // Returns the raw body as text for every format; the caller turns it into a Blob download.
  export: async (token: string, format: ExportFormat): Promise<{ text: string; mime: string }> => {
    const { data, error, response } = await api.GET('/api/v1/public/share/{token}/export', {
      params: { path: { token }, query: { format } },
      parseAs: 'text',
    })
    if (error !== undefined || !response.ok) {
      const { toApiError } = await import('@api/errors')
      // parseAs 'text' hands the problem+json over as a string.
      let parsed: unknown = error
      if (typeof error === 'string') {
        try {
          parsed = JSON.parse(error)
        } catch {
          parsed = undefined
        }
      }
      throw await toApiError(response, parsed)
    }
    return {
      text: typeof data === 'string' ? data : JSON.stringify(data, null, 2),
      mime: response.headers.get('content-type') ?? 'text/plain',
    }
  },
}

// ---------------------------------------------------------------------------------------------

export const exampleQueries = {
  list: (q: { kind?: Example['kind']; db_kind?: Example['db_kind'] } = {}) =>
    queryOptions({
      queryKey: [...keys.catalog(), 'examples', q],
      queryFn: () => unwrap(api.GET('/api/v1/catalog/examples', { params: { query: q } })),
      staleTime: Number.POSITIVE_INFINITY,
    }),
}

export const exampleMutations = {
  clone: (slug: string, exampleId: string, name?: string) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/examples/{exampleId}:clone', {
        params: { path: { slug, exampleId } },
        body: { name },
      })
    ),
  quickRun: (slug: string, exampleId: string, body: LaunchOverrides = {}) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/examples/{exampleId}:quick-run', {
        params: { path: { slug, exampleId } },
        body,
      })
    ),
}

// ---------------------------------------------------------------------------------------------
// Provider profiles (read-only subset used by the dashboard and the examples quick-run modal).

export const providerQueries = {
  list: (slug: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'providers', 'list'],
      queryFn: () => unwrap(api.GET('/api/v1/t/{slug}/providers', { params: { path: { slug } } })),
    }),
  quotas: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'providers', 'quotas', id],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/providers/{id}/quotas', { params: { path: { slug, id } } })
        ),
      staleTime: 60_000,
    }),
}
