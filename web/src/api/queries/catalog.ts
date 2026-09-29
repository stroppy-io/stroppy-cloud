import { api, unwrap } from '@api/client'
import type { Example } from '@api/types'
import { queryOptions } from '@tanstack/react-query'
import { keys } from './keys'

const forever = { staleTime: Number.POSITIVE_INFINITY, gcTime: 24 * 3600_000 }

export const catalogQueries = {
  databases: () =>
    queryOptions({
      queryKey: [...keys.catalog(), 'databases'],
      queryFn: () => unwrap(api.GET('/api/v1/catalog/databases')),
      ...forever,
    }),
  providers: () =>
    queryOptions({
      queryKey: [...keys.catalog(), 'providers'],
      queryFn: () => unwrap(api.GET('/api/v1/catalog/providers')),
      ...forever,
    }),
  stroppy: () =>
    queryOptions({
      queryKey: [...keys.catalog(), 'stroppy'],
      queryFn: () => unwrap(api.GET('/api/v1/catalog/stroppy')),
      ...forever,
    }),
  metrics: () =>
    queryOptions({
      queryKey: [...keys.catalog(), 'metrics'],
      queryFn: () => unwrap(api.GET('/api/v1/catalog/metrics')),
      ...forever,
    }),
  examples: (q: { kind?: Example['kind']; db_kind?: Example['db_kind'] } = {}) =>
    queryOptions({
      queryKey: [...keys.catalog(), 'examples', q],
      queryFn: () => unwrap(api.GET('/api/v1/catalog/examples', { params: { query: q } })),
      ...forever,
    }),
  schemas: (namespace?: string) =>
    queryOptions({
      queryKey: [...keys.catalog(), 'schemas', namespace],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/catalog/schemas', { params: { query: namespace ? { namespace } : {} } })
        ),
      ...forever,
    }),
  schema: (schemaId: string) =>
    queryOptions({
      queryKey: [...keys.catalog(), 'schema', schemaId],
      queryFn: () =>
        unwrap(api.GET('/api/v1/catalog/schemas/{schemaId}', { params: { path: { schemaId } } })),
      ...forever,
    }),
}

export const catalogMutations = {
  validate: (schemaId: string, value: Record<string, unknown>) =>
    unwrap(
      api.POST('/api/v1/catalog/schemas/{schemaId}:validate', {
        params: { path: { schemaId } },
        body: { value },
      })
    ),
  render: (schemaId: string, value: Record<string, unknown>, template?: string) =>
    unwrap(
      api.POST('/api/v1/catalog/schemas/{schemaId}:render', {
        params: { path: { schemaId } },
        body: { value, template },
      })
    ),
  cloneExample: (slug: string, exampleId: string, name?: string) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/examples/{exampleId}:clone', {
        params: { path: { slug, exampleId } },
        body: { name } as never,
      })
    ),
  quickRunExample: (slug: string, exampleId: string, body: Record<string, unknown> = {}) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/examples/{exampleId}:quick-run', {
        params: { path: { slug, exampleId } },
        body: body as never,
      })
    ),
}
