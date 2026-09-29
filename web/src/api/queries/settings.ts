import { api, unwrap } from '@api/client'
import type { ProviderKind, Schemas, Webhook } from '@api/types'
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query'
import { keys } from './keys'

export type TenantSettingsPatch = Schemas['TenantSettingsPatch']
export type ProviderProfileCreate = Schemas['ProviderProfileCreate']
export type ProviderProfilePatch = Schemas['ProviderProfilePatch']
export type WebhookCreate = Schemas['WebhookCreate']
export type WebhookPatch = Schemas['WebhookPatch']
export type WebhookCreated = Schemas['WebhookCreated']

const terminalProvider = (s: string | undefined) =>
  s === 'ready' || s === 'failed' || s === 'delete_failed' || s === undefined

export const settingsQueries = {
  settings: (slug: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'settings'],
      queryFn: () => unwrap(api.GET('/api/v1/t/{slug}/settings', { params: { path: { slug } } })),
    }),
  limits: (slug: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'limits'],
      queryFn: () => unwrap(api.GET('/api/v1/t/{slug}/limits', { params: { path: { slug } } })),
    }),
  providers: (slug: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'providers', 'list'],
      queryFn: () => unwrap(api.GET('/api/v1/t/{slug}/providers', { params: { path: { slug } } })),
      // Poll while any profile is still verifying / deleting.
      refetchInterval: (q) =>
        q.state.data?.data.some((p) => !terminalProvider(p.status)) ? 2000 : false,
    }),
  provider: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'providers', 'detail', id],
      queryFn: () =>
        unwrap(api.GET('/api/v1/t/{slug}/providers/{id}', { params: { path: { slug, id } } })),
      refetchInterval: (q) => (terminalProvider(q.state.data?.status) ? false : 2000),
    }),
  quotas: (slug: string, id: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'providers', 'quotas', id],
      queryFn: () =>
        unwrap(
          api.GET('/api/v1/t/{slug}/providers/{id}/quotas', { params: { path: { slug, id } } })
        ),
    }),
  webhooks: (slug: string) =>
    queryOptions({
      queryKey: [...keys.t(slug), 'webhooks', 'list'],
      queryFn: () => unwrap(api.GET('/api/v1/t/{slug}/webhooks', { params: { path: { slug } } })),
    }),
  deliveries: (slug: string, id: string) =>
    infiniteQueryOptions({
      queryKey: [...keys.t(slug), 'webhooks', 'deliveries', id],
      queryFn: ({ pageParam }) =>
        unwrap(
          api.GET('/api/v1/t/{slug}/webhooks/{id}/deliveries', {
            params: { path: { slug, id }, query: { cursor: pageParam || undefined, limit: 25 } },
          })
        ),
      initialPageParam: '',
      getNextPageParam: (last) => last.meta.next_cursor ?? undefined,
    }),
}

// Tenant provider profiles as read by the library (test wizard, launch drawer) — same keys
// as `settingsQueries.providers/provider`, so both areas share one cache entry.
export const providerQueries = {
  list: settingsQueries.providers,
  detail: settingsQueries.provider,
}

export const settingsMutations = {
  patchSettings: (slug: string, body: TenantSettingsPatch) =>
    unwrap(api.PATCH('/api/v1/t/{slug}/settings', { params: { path: { slug } }, body })),
  createProvider: (slug: string, body: ProviderProfileCreate) =>
    unwrap(api.POST('/api/v1/t/{slug}/providers', { params: { path: { slug } }, body })),
  patchProvider: (slug: string, id: string, body: ProviderProfilePatch) =>
    unwrap(api.PATCH('/api/v1/t/{slug}/providers/{id}', { params: { path: { slug, id } }, body })),
  deleteProvider: (slug: string, id: string) =>
    unwrap(api.DELETE('/api/v1/t/{slug}/providers/{id}', { params: { path: { slug, id } } })),
  verifyProvider: (slug: string, id: string) =>
    unwrap(api.POST('/api/v1/t/{slug}/providers/{id}:verify', { params: { path: { slug, id } } })),
  refreshQuotas: (slug: string, id: string) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/providers/{id}/quotas:refresh', {
        params: { path: { slug, id } },
      })
    ),
  createWebhook: (slug: string, body: WebhookCreate) =>
    unwrap(api.POST('/api/v1/t/{slug}/webhooks', { params: { path: { slug } }, body })),
  patchWebhook: (slug: string, id: string, body: WebhookPatch) =>
    unwrap(api.PATCH('/api/v1/t/{slug}/webhooks/{id}', { params: { path: { slug, id } }, body })),
  deleteWebhook: (slug: string, id: string) =>
    unwrap(api.DELETE('/api/v1/t/{slug}/webhooks/{id}', { params: { path: { slug, id } } })),
  rotateWebhookSecret: (slug: string, id: string) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/webhooks/{id}:rotate-secret', {
        params: { path: { slug, id } },
      })
    ),
  replayDelivery: (slug: string, id: string, deliveryId: string) =>
    unwrap(
      api.POST('/api/v1/t/{slug}/webhooks/{id}/deliveries/{deliveryId}:replay', {
        params: { path: { slug, id, deliveryId } },
      })
    ),
}

// All webhook events from the OpenAPI enum `WebhookEvent`.
export const WEBHOOK_EVENTS: Webhook['events'] = [
  'run.started',
  'run.stage',
  'run.finished',
  'run.failed',
  'run.cancelled',
  'suite.started',
  'suite.cell_finished',
  'suite.finished',
]

export const PROVIDER_KINDS: ProviderKind[] = ['yandex', 'aws']
