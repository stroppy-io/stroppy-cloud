import { api, unwrap } from '@api/client'
import type { ApiToken, MePatch } from '@api/types'
import { queryOptions } from '@tanstack/react-query'
import { keys } from './keys'

export const meQueries = {
  me: () =>
    queryOptions({
      queryKey: [...keys.me()],
      queryFn: () => unwrap(api.GET('/api/v1/me')),
      staleTime: 60_000,
    }),
  tokens: () =>
    queryOptions({
      queryKey: [...keys.me(), 'tokens'],
      queryFn: () => unwrap(api.GET('/api/v1/me/tokens')),
    }),
  invites: () =>
    queryOptions({
      queryKey: [...keys.me(), 'invites'],
      queryFn: () => unwrap(api.GET('/api/v1/me/invites')),
    }),
  publicConfig: () =>
    queryOptions({
      queryKey: [...keys.public(), 'config'],
      queryFn: () => unwrap(api.GET('/api/v1/public/config')),
      staleTime: Number.POSITIVE_INFINITY,
    }),
}

export const meMutations = {
  patch: (body: MePatch) => unwrap(api.PATCH('/api/v1/me', { body })),
  createToken: (body: {
    name: string
    tenant_id: string
    role: ApiToken['role']
    expires_at?: string | null
  }) => unwrap(api.POST('/api/v1/me/tokens', { body: body as never })),
  revokeToken: (id: string) =>
    unwrap(api.DELETE('/api/v1/me/tokens/{id}', { params: { path: { id } } })),
  acceptInvite: (id: string) =>
    unwrap(api.POST('/api/v1/me/invites/{id}:accept', { params: { path: { id } } })),
  declineInvite: (id: string) =>
    unwrap(api.POST('/api/v1/me/invites/{id}:decline', { params: { path: { id } } })),
}
