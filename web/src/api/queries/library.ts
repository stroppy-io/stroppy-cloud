import { api, unwrap } from '@api/client'
import type {
  DatabaseWrite,
  LaunchOverrides,
  Paths,
  Schemas,
  TestWrite,
  WorkloadWrite,
} from '@api/types'
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query'
import { keys } from './keys'

export type DatabaseListQuery = NonNullable<
  Paths['/api/v1/t/{slug}/databases']['get']['parameters']['query']
>
export type WorkloadListQuery = NonNullable<
  Paths['/api/v1/t/{slug}/workloads']['get']['parameters']['query']
>
export type TestListQuery = NonNullable<
  Paths['/api/v1/t/{slug}/tests']['get']['parameters']['query']
>

export type LibraryKind = 'database' | 'workload' | 'test'
export type ExportDocument = Schemas['ExportDocument']

// ---------- databases ----------
export const databaseQueries = {
  list: (slug: string, q: DatabaseListQuery = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.t(slug), 'databases', 'list', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/t/{slug}/databases', {
            params: { path: { slug }, query: { ...q, cursor: pageParam ?? undefined } },
          })
        ),
      initialPageParam: null as string | null,
      getNextPageParam: (last) => last.meta?.next_cursor ?? null,
    }),
  // Flat list for pickers (Combobox), first 200 by name.
  options: (slug: string, search?: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'databases', 'options', search ?? ''],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/databases', {
            params: { path: { slug }, query: { search, sort: 'name', order: 'asc', limit: 200 } },
          })
        ),
    }),
  detail: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'databases', 'detail', id],
      queryFn: () =>
        unwrap(api.GET('/api/v1/t/{slug}/databases/{id}', { params: { path: { slug, id } } })),
    }),
  diff: (slug: string, a: string, b: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'databases', 'diff', a, b],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/databases:diff', {
            params: { path: { slug }, query: { a, b } },
          })
        ),
      enabled: !!a && !!b && a !== b,
    }),
  export: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'databases', 'export', id],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/databases/{id}:export', { params: { path: { slug, id } } })
        ),
    }),
}

// ---------- workloads ----------
export const workloadQueries = {
  list: (slug: string, q: WorkloadListQuery = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.t(slug), 'workloads', 'list', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/t/{slug}/workloads', {
            params: { path: { slug }, query: { ...q, cursor: pageParam ?? undefined } },
          })
        ),
      initialPageParam: null as string | null,
      getNextPageParam: (last) => last.meta?.next_cursor ?? null,
    }),
  options: (slug: string, search?: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'workloads', 'options', search ?? ''],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/workloads', {
            params: { path: { slug }, query: { search, sort: 'name', order: 'asc', limit: 200 } },
          })
        ),
    }),
  detail: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'workloads', 'detail', id],
      queryFn: () =>
        unwrap(api.GET('/api/v1/t/{slug}/workloads/{id}', { params: { path: { slug, id } } })),
    }),
  diff: (slug: string, a: string, b: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'workloads', 'diff', a, b],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/workloads:diff', {
            params: { path: { slug }, query: { a, b } },
          })
        ),
      enabled: !!a && !!b && a !== b,
    }),
  export: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'workloads', 'export', id],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/workloads/{id}:export', { params: { path: { slug, id } } })
        ),
    }),
}

// ---------- tests ----------
export const testQueries = {
  list: (slug: string, q: TestListQuery = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.t(slug), 'tests', 'list', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/t/{slug}/tests', {
            params: { path: { slug }, query: { ...q, cursor: pageParam ?? undefined } },
          })
        ),
      initialPageParam: null as string | null,
      getNextPageParam: (last) => last.meta?.next_cursor ?? null,
    }),
  options: (slug: string, search?: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'tests', 'options', search ?? ''],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/tests', {
            params: { path: { slug }, query: { search, sort: 'name', order: 'asc', limit: 200 } },
          })
        ),
    }),
  detail: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'tests', 'detail', id],
      queryFn: () =>
        unwrap(api.GET('/api/v1/t/{slug}/tests/{id}', { params: { path: { slug, id } } })),
    }),
  runs: (slug: string, id: string, limit = 50) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'tests', 'runs', id, limit],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/tests/{id}/runs', {
            params: { path: { slug, id }, query: { limit } },
          })
        ),
    }),
  diff: (slug: string, a: string, b: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'tests', 'diff', a, b],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/tests:diff', { params: { path: { slug }, query: { a, b } } })
        ),
      enabled: !!a && !!b && a !== b,
    }),
  export: (slug: string, id: string, inlineRefs = false) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'tests', 'export', id, String(inlineRefs)],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/tests/{id}:export', {
            params: { path: { slug, id }, query: { inline_refs: inlineRefs } },
          })
        ),
    }),
}

// ---------- mutations ----------
export const libraryMutations = {
  // databases
  createDatabase: (slug: string, body: DatabaseWrite) =>
    unwrap(api.POST('/api/v1/t/{slug}/databases', { params: { path: { slug } }, body })),
  patchDatabase: (slug: string, id: string, body: Schemas['DatabasePatch']) =>
    unwrap(api.PATCH('/api/v1/t/{slug}/databases/{id}', { params: { path: { slug, id } }, body })),
  deleteDatabase: (slug: string, id: string, inlineUsages = false) =>
    unwrap(
      api.DELETE('/api/v1/t/{slug}/databases/{id}', {
        params: { path: { slug, id }, query: inlineUsages ? { inline_usages: true } : {} },
      })
    ),
  previewDatabase: (slug: string, body: DatabaseWrite) =>
    unwrap(api.POST('/api/v1/t/{slug}/databases:preview', { params: { path: { slug } }, body })),
  cloneDatabase: (slug: string, id: string, name?: string) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/databases/{id}:clone', {
        params: { path: { slug, id } },
        body: { name },
      })
    ),
  importDatabase: (slug: string, body: ExportDocument) =>
    unwrap(api.POST('/api/v1/t/{slug}/databases:import', { params: { path: { slug } }, body })),

  // workloads
  createWorkload: (slug: string, body: WorkloadWrite) =>
    unwrap(api.POST('/api/v1/t/{slug}/workloads', { params: { path: { slug } }, body })),
  patchWorkload: (slug: string, id: string, body: Schemas['WorkloadPatch']) =>
    unwrap(api.PATCH('/api/v1/t/{slug}/workloads/{id}', { params: { path: { slug, id } }, body })),
  deleteWorkload: (slug: string, id: string, inlineUsages = false) =>
    unwrap(
      api.DELETE('/api/v1/t/{slug}/workloads/{id}', {
        params: { path: { slug, id }, query: inlineUsages ? { inline_usages: true } : {} },
      })
    ),
  previewWorkload: (slug: string, body: WorkloadWrite) =>
    unwrap(api.POST('/api/v1/t/{slug}/workloads:preview', { params: { path: { slug } }, body })),
  cloneWorkload: (slug: string, id: string, name?: string) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/workloads/{id}:clone', {
        params: { path: { slug, id } },
        body: { name },
      })
    ),
  importWorkload: (slug: string, body: ExportDocument) =>
    unwrap(api.POST('/api/v1/t/{slug}/workloads:import', { params: { path: { slug } }, body })),

  // tests
  createTest: (slug: string, body: TestWrite) =>
    unwrap(api.POST('/api/v1/t/{slug}/tests', { params: { path: { slug } }, body })),
  // `finalize` asks the server to promote the draft to `ready` (fails with validation when it does not fit).
  patchTest: (slug: string, id: string, body: Schemas['TestPatch'], finalize = false) =>
    unwrap(
      api.PATCH('/api/v1/t/{slug}/tests/{id}', {
        params: { path: { slug, id } },
        body: (finalize ? { ...body, finalize: true } : body) as never,
      })
    ),
  deleteTest: (slug: string, id: string) =>
    unwrap(api.DELETE('/api/v1/t/{slug}/tests/{id}', { params: { path: { slug, id } } })),
  validateTest: (slug: string, body: TestWrite) =>
    unwrap(api.POST('/api/v1/t/{slug}/tests:validate', { params: { path: { slug } }, body })),
  // Re-validate a saved test in place (clears `stale`, recomputes status).
  revalidateTest: (slug: string, id: string) =>
    unwrap(
      api.PATCH('/api/v1/t/{slug}/tests/{id}', {
        params: { path: { slug, id } },
        body: { revalidate: true } as never,
      })
    ),
  launchTest: (slug: string, id: string, body: LaunchOverrides = {}) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/tests/{id}:launch', { params: { path: { slug, id } }, body })
    ),
  cloneTest: (slug: string, id: string, name?: string) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/tests/{id}:clone', {
        params: { path: { slug, id } },
        body: { name },
      })
    ),
  importTest: (slug: string, body: ExportDocument) =>
    unwrap(api.POST('/api/v1/t/{slug}/tests:import', { params: { path: { slug } }, body })),
}

// Common shape of a library row for pickers, regardless of kind.
export type LibraryEntity = Schemas['Entity'] & {
  kind?: string
  version?: string
  protocol?: string
  stroppy_version?: string
  status?: string
}

function pickerOptions(kind: LibraryKind, slug: string) {
  const path =
    kind === 'database'
      ? '/api/v1/t/{slug}/databases'
      : kind === 'workload'
        ? '/api/v1/t/{slug}/workloads'
        : '/api/v1/t/{slug}/tests'
  return queryOptions({
    queryKey: [...keys.t(slug), `${kind}s`, 'options', ''],
    queryFn: async (): Promise<{ data: LibraryEntity[] }> => {
      const r = await unwrap(
        api.GET(path as '/api/v1/t/{slug}/databases', {
          params: { path: { slug }, query: { sort: 'name', order: 'asc', limit: 200 } },
        })
      )
      return r as unknown as { data: LibraryEntity[] }
    },
  })
}

// Kind-generic accessors so shared widgets (import modal, actions menu) stay one component.
export const libraryByKind = {
  database: {
    clone: libraryMutations.cloneDatabase,
    remove: libraryMutations.deleteDatabase,
    import: libraryMutations.importDatabase,
    export: databaseQueries.export,
    diff: databaseQueries.diff,
    options: (slug: string) => pickerOptions('database', slug),
  },
  workload: {
    clone: libraryMutations.cloneWorkload,
    remove: libraryMutations.deleteWorkload,
    import: libraryMutations.importWorkload,
    export: workloadQueries.export,
    diff: workloadQueries.diff,
    options: (slug: string) => pickerOptions('workload', slug),
  },
  test: {
    clone: libraryMutations.cloneTest,
    remove: (slug: string, id: string) => libraryMutations.deleteTest(slug, id),
    import: libraryMutations.importTest,
    export: (slug: string, id: string) => testQueries.export(slug, id),
    diff: testQueries.diff,
    options: (slug: string) => pickerOptions('test', slug),
  },
} as const
