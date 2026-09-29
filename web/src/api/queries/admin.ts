import { api, unwrap } from '@api/client'
import type { RunStatus, Schemas } from '@api/types'
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query'
import { keys } from './keys'

export type AdminTenant = Schemas['AdminTenant']
export type TenantLimitsWrite = Schemas['TenantLimitsWrite']
export type SystemSettingsPatch = Schemas['SystemSettingsPatch']
export type AdminRun = Schemas['Run'] & { tenant?: Schemas['Ref'] }

export interface AdminTenantsQuery {
  search?: string
  status?: 'active' | 'orphaned' | 'suspended'
}
export interface AdminUsersQuery {
  search?: string
  platform_admin?: boolean
}
export interface AdminRunsQuery {
  status?: RunStatus[]
  tenant?: string
}
export interface AdminAuditQuery {
  tenant?: string
  action?: string
  actor?: string
  since?: string
}

export const adminQueries = {
  status: () =>
    queryOptions({
      queryKey: [...keys.admin(), 'status'],
      queryFn: () => unwrap(api.GET('/api/v1/admin/status')),
      refetchInterval: 15_000,
    }),
  tenants: (q: AdminTenantsQuery = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.admin(), 'tenants', 'list', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/admin/tenants', {
            params: { query: { ...q, cursor: pageParam || undefined, limit: 50 } },
          })
        ),
      initialPageParam: '',
      getNextPageParam: (last) => last.meta.next_cursor ?? undefined,
    }),
  tenant: (slug: string) =>
    queryOptions({
      queryKey: [...keys.admin(), 'tenants', 'detail', slug],
      queryFn: () =>
        unwrap(api.GET('/api/v1/admin/tenants/{slug}', { params: { path: { slug } } })),
    }),
  users: (q: AdminUsersQuery = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.admin(), 'users', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/admin/users', {
            params: { query: { ...q, cursor: pageParam || undefined, limit: 50 } },
          })
        ),
      initialPageParam: '',
      getNextPageParam: (last) => last.meta.next_cursor ?? undefined,
    }),
  settings: () =>
    queryOptions({
      queryKey: [...keys.admin(), 'settings'],
      queryFn: () => unwrap(api.GET('/api/v1/admin/settings')),
    }),
  runs: (q: AdminRunsQuery = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.admin(), 'runs', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/admin/runs', {
            params: { query: { ...q, cursor: pageParam || undefined, limit: 50 } },
          })
        ),
      initialPageParam: '',
      getNextPageParam: (last) => last.meta.next_cursor ?? undefined,
      refetchInterval: 5000,
    }),
  audit: (q: AdminAuditQuery = {}) =>
    infiniteQueryOptions({
      queryKey: [...keys.admin(), 'audit', q],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/admin/audit', {
            params: { query: { ...q, cursor: pageParam || undefined, limit: 50 } },
          })
        ),
      initialPageParam: '',
      getNextPageParam: (last) => last.meta.next_cursor ?? undefined,
    }),
}

export const adminMutations = {
  suspendTenant: (slug: string, reason?: string) =>
    unwrap(
      api.POST('/api/v1/admin/tenants/{slug}:suspend', {
        params: { path: { slug } },
        body: { reason },
      })
    ),
  resumeTenant: (slug: string) =>
    unwrap(api.POST('/api/v1/admin/tenants/{slug}:resume', { params: { path: { slug } } })),
  setLimits: (slug: string, body: TenantLimitsWrite) =>
    unwrap(api.PUT('/api/v1/admin/tenants/{slug}/limits', { params: { path: { slug } }, body })),
  assignOwner: (slug: string, userId: string) =>
    unwrap(
      api.POST('/api/v1/admin/tenants/{slug}:assign-owner', {
        params: { path: { slug } },
        body: { user_id: userId },
      })
    ),
  deleteTenant: (slug: string) =>
    unwrap(api.DELETE('/api/v1/admin/tenants/{slug}', { params: { path: { slug } } })),
  patchUser: (userId: string, body: { is_platform_admin?: boolean }) =>
    unwrap(api.PATCH('/api/v1/admin/users/{userId}', { params: { path: { userId } }, body })),
  patchSettings: (body: SystemSettingsPatch) =>
    unwrap(api.PATCH('/api/v1/admin/settings', { body })),
  resync: (tenantSlug?: string) =>
    unwrap(
      api.POST('/api/v1/admin/pipelines:resync', {
        body: tenantSlug ? { tenant_slug: tenantSlug } : {},
      })
    ),
  cancelRun: (id: string) =>
    unwrap(api.POST('/api/v1/admin/runs/{id}:cancel', { params: { path: { id } } })),
}
