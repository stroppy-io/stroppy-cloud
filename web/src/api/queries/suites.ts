import { api, unwrap } from '@api/client'
import type { Paths, Schemas } from '@api/types'
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query'
import { keys } from './keys'

export type SuiteListQuery = NonNullable<
  Paths['/api/v1/t/{slug}/suites']['get']['parameters']['query']
>
export type SuiteRunListQuery = NonNullable<
  Paths['/api/v1/t/{slug}/suite-runs']['get']['parameters']['query']
>
export type ScheduleListQuery = NonNullable<
  Paths['/api/v1/t/{slug}/schedules']['get']['parameters']['query']
>

// ---------- suites ----------
export const suiteQueries = {
  list: (slug: string, q: SuiteListQuery = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.t(slug), 'suites', 'list', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/t/{slug}/suites', {
            params: { path: { slug }, query: { ...q, cursor: pageParam ?? undefined } },
          })
        ),
      initialPageParam: null as string | null,
      getNextPageParam: (last) => last.meta?.next_cursor ?? null,
    }),
  // Flat list for pickers (schedule target, launch dialogs).
  all: (slug: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'suites', 'all'],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/suites', {
            params: { path: { slug }, query: { limit: 200, sort: 'name', order: 'asc' } },
          })
        ),
      staleTime: 30_000,
    }),
  detail: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'suites', 'detail', id],
      queryFn: () =>
        unwrap(api.GET('/api/v1/t/{slug}/suites/{id}', { params: { path: { slug, id } } })),
    }),
  runsOf: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'suites', 'runs', id],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/suites/{id}/runs', {
            params: { path: { slug, id }, query: { limit: 100 } },
          })
        ),
    }),
  preview: (slug: string, body: Schemas['SuiteWrite'] | undefined) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'suites', 'preview', body],
      queryFn: () =>
        unwrap(
          api.POST('/api/v1/t/{slug}/suites:preview', {
            params: { path: { slug } },
            body: body as Schemas['SuiteWrite'],
          })
        ),
      enabled: !!body,
      staleTime: 10_000,
    }),
}

export const suiteMutations = {
  create: (slug: string, body: Schemas['SuiteWrite']) =>
    unwrap(api.POST('/api/v1/t/{slug}/suites', { params: { path: { slug } }, body })),
  patch: (slug: string, id: string, body: Schemas['SuitePatch']) =>
    unwrap(api.PATCH('/api/v1/t/{slug}/suites/{id}', { params: { path: { slug, id } }, body })),
  remove: (slug: string, id: string) =>
    unwrap(api.DELETE('/api/v1/t/{slug}/suites/{id}', { params: { path: { slug, id } } })),
  preview: (slug: string, body: Schemas['SuiteWrite']) =>
    unwrap(api.POST('/api/v1/t/{slug}/suites:preview', { params: { path: { slug } }, body })),
  launch: (slug: string, id: string, body: Schemas['SuiteLaunch'] = {}) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/suites/{id}:launch', { params: { path: { slug, id } }, body })
    ),
  clone: (slug: string, id: string, name?: string) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/suites/{id}:clone', {
        params: { path: { slug, id } },
        body: { name },
      })
    ),
  export: (slug: string, id: string) =>
    unwrap(api.GET('/api/v1/t/{slug}/suites/{id}:export', { params: { path: { slug, id } } })),
  import: (slug: string, body: Schemas['ExportDocument']) =>
    unwrap(api.POST('/api/v1/t/{slug}/suites:import', { params: { path: { slug } }, body })),
}

// ---------- suite runs ----------
export const suiteRunQueries = {
  list: (slug: string, q: SuiteRunListQuery = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.t(slug), 'suite-runs', 'list', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/t/{slug}/suite-runs', {
            params: { path: { slug }, query: { ...q, cursor: pageParam ?? undefined } },
          })
        ),
      initialPageParam: null as string | null,
      getNextPageParam: (last) => last.meta?.next_cursor ?? null,
    }),
  detail: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'suite-runs', 'detail', id],
      queryFn: () =>
        unwrap(api.GET('/api/v1/t/{slug}/suite-runs/{id}', { params: { path: { slug, id } } })),
    }),
  summary: (slug: string, id: string, compareTo?: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'suite-runs', 'summary', id, compareTo ?? null],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/suite-runs/{id}/summary', {
            params: { path: { slug, id }, query: compareTo ? { compare_to: compareTo } : {} },
          })
        ),
    }),
}

export const suiteRunMutations = {
  cancel: (slug: string, id: string) =>
    unwrap(api.POST('/api/v1/t/{slug}/suite-runs/{id}:cancel', { params: { path: { slug, id } } })),
  remove: (slug: string, id: string) =>
    unwrap(api.DELETE('/api/v1/t/{slug}/suite-runs/{id}', { params: { path: { slug, id } } })),
  retryFailed: (slug: string, id: string) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/suite-runs/{id}:retry-failed', {
        params: { path: { slug, id } },
      })
    ),
  share: (slug: string, id: string, body: Schemas['ShareCreate']) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/suite-runs/{id}:share', { params: { path: { slug, id } }, body })
    ),
}

// ---------- schedules ----------
export const scheduleQueries = {
  list: (slug: string, q: ScheduleListQuery = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.t(slug), 'schedules', 'list', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/t/{slug}/schedules', {
            params: { path: { slug }, query: { ...q, cursor: pageParam ?? undefined } },
          })
        ),
      initialPageParam: null as string | null,
      getNextPageParam: (last) => last.meta?.next_cursor ?? null,
    }),
  detail: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'schedules', 'detail', id],
      queryFn: () =>
        unwrap(api.GET('/api/v1/t/{slug}/schedules/{id}', { params: { path: { slug, id } } })),
    }),
  history: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'schedules', 'history', id],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/schedules/{id}/history', {
            params: { path: { slug, id }, query: { limit: 100 } },
          })
        ),
    }),
}

export const scheduleMutations = {
  create: (slug: string, body: Schemas['ScheduleWrite']) =>
    unwrap(api.POST('/api/v1/t/{slug}/schedules', { params: { path: { slug } }, body })),
  patch: (slug: string, id: string, body: Schemas['SchedulePatch']) =>
    unwrap(api.PATCH('/api/v1/t/{slug}/schedules/{id}', { params: { path: { slug, id } }, body })),
  remove: (slug: string, id: string) =>
    unwrap(api.DELETE('/api/v1/t/{slug}/schedules/{id}', { params: { path: { slug, id } } })),
  pause: (slug: string, id: string) =>
    unwrap(api.POST('/api/v1/t/{slug}/schedules/{id}:pause', { params: { path: { slug, id } } })),
  resume: (slug: string, id: string) =>
    unwrap(api.POST('/api/v1/t/{slug}/schedules/{id}:resume', { params: { path: { slug, id } } })),
  runNow: (slug: string, id: string) =>
    unwrap(api.POST('/api/v1/t/{slug}/schedules/{id}:run-now', { params: { path: { slug, id } } })),
}
