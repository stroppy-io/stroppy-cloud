import { api, unwrap } from '@api/client'
import type { TenantRole } from '@api/types'
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query'
import { keys } from './keys'

export const tenantQueries = {
  list: () =>
    queryOptions({
      queryKey: [...keys.me(), 'tenants'],
      queryFn: () => unwrap(api.GET('/api/v1/tenants')),
    }),
  detail: (slug: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'tenant'],
      queryFn: () => unwrap(api.GET('/api/v1/tenants/{slug}', { params: { path: { slug } } })),
    }),
  members: (slug: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'members'],
      queryFn: () =>
        unwrap(api.GET('/api/v1/tenants/{slug}/members', { params: { path: { slug } } })),
    }),
  invites: (slug: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'invites'],
      queryFn: () =>
        unwrap(api.GET('/api/v1/tenants/{slug}/invites', { params: { path: { slug } } })),
    }),
  tokens: (slug: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'tokens'],
      queryFn: () =>
        unwrap(api.GET('/api/v1/tenants/{slug}/tokens', { params: { path: { slug } } })),
    }),
  audit: (
    slug: string,
    q: { action?: string; actor?: string; since?: string; cursor?: string; limit?: number } = {}
  ) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'audit', q],
      queryFn: () =>
        unwrap(api.GET('/api/v1/t/{slug}/audit', { params: { path: { slug }, query: q } })),
    }),
  // Cursor-paginated audit for the settings screen ("load more").
  auditPages: (slug: string, q: { action?: string; actor?: string; since?: string } = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.t(slug), 'audit', 'pages', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/t/{slug}/audit', {
            params: { path: { slug }, query: { ...q, cursor: pageParam || undefined, limit: 50 } },
          })
        ),
      initialPageParam: '',
      getNextPageParam: (last) => last.meta.next_cursor ?? undefined,
    }),
  suggestName: (name?: string) =>
    queryOptions({
      queryKey: [...keys.me(), 'suggest-name', name],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/tenants/suggest-name', {
            params: { query: name ? { name } : {} },
          } as never)
        ),
    }),
}

export const tenantMutations = {
  create: (body: { name: string; slug?: string; description?: string }) =>
    unwrap(api.POST('/api/v1/tenants', { body })),
  patch: (
    slug: string,
    body: { name?: string; description?: string; public_name?: string | null }
  ) => unwrap(api.PATCH('/api/v1/tenants/{slug}', { params: { path: { slug } }, body })),
  remove: (slug: string) =>
    unwrap(api.DELETE('/api/v1/tenants/{slug}', { params: { path: { slug } } })),
  setRole: (slug: string, userId: string, role: TenantRole) =>
    unwrap(
      api.PATCH('/api/v1/tenants/{slug}/members/{userId}', {
        params: { path: { slug, userId } },
        body: { role } as never,
      })
    ),
  removeMember: (slug: string, userId: string) =>
    unwrap(
      api.DELETE('/api/v1/tenants/{slug}/members/{userId}', { params: { path: { slug, userId } } })
    ),
  transfer: (slug: string, userId: string) =>
    unwrap(
      api.POST('/api/v1/tenants/{slug}:transfer', {
        params: { path: { slug } },
        body: { user_id: userId } as never,
      })
    ),
  invite: (slug: string, body: { email: string; role: TenantRole; message?: string }) =>
    unwrap(
      api.POST('/api/v1/tenants/{slug}/invites', {
        params: { path: { slug } },
        body: body as never,
      })
    ),
  revokeInvite: (slug: string, id: string) =>
    unwrap(api.DELETE('/api/v1/tenants/{slug}/invites/{id}', { params: { path: { slug, id } } })),
  createToken: (
    slug: string,
    body: { name: string; role: 'member' | 'viewer'; expires_at?: string | null }
  ) =>
    unwrap(
      api.POST('/api/v1/tenants/{slug}/tokens', { params: { path: { slug } }, body: body as never })
    ),
  revokeToken: (slug: string, id: string) =>
    unwrap(api.DELETE('/api/v1/tenants/{slug}/tokens/{id}', { params: { path: { slug, id } } })),
}
