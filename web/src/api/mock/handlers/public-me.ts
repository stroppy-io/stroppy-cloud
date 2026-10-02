import type { ApiToken, Me, MePatch } from '@api/types'
import { noContent, notFound, problem, route } from '../router'
import { iso, uuid } from '../util'

route('GET', '/api/v1/public/config', ({ store }) => ({
  json: {
    auth: { mode: 'dev', kratos: { public_url: '' } },
    tenant_creation: store.system.tenant_creation,
    public_rating_enabled: store.system.public_rating_enabled,
    examples_enabled: store.system.examples_enabled,
    version: store.version,
    commit: 'mock',
  },
}))

route('GET', '/api/v1/public/catalog', ({ store }) => ({
  json: {
    databases: store.catalog.databases.map((d) => ({
      kind: d.kind,
      title: d.title,
      versions: d.versions.map((v) => v.version),
      topologies: d.topologies.map((t) => t.id),
    })),
    providers: store.catalog.providers.map((p) => p.kind),
  },
}))

route('GET', '/api/v1/public/health', ({ store }) => ({
  json: {
    status: 'ok',
    version: store.version,
    components: {
      graphene: { status: 'ok' },
      kratos: { status: 'ok', detail: 'mock mode' },
      victoria: { status: 'ok' },
      postgres: { status: 'ok' },
    },
  },
}))

route('GET', '/api/v1/me', ({ store }) => ({ json: store.me }))

route('PATCH', '/api/v1/me', ({ store, body }) => {
  const patch = body as MePatch
  const me: Me = { ...store.me }
  if (patch.display_name !== undefined) me.display_name = patch.display_name
  if (patch.avatar !== undefined) me.avatar = patch.avatar
  if (patch.preferences) me.preferences = { ...me.preferences, ...patch.preferences }
  if (patch.notifications) me.notifications = { ...me.notifications, ...patch.notifications }
  store.me = me
  return { json: me }
})

route('GET', '/api/v1/me/tokens', ({ store }) => ({
  json: { data: store.personalTokens, meta: { next_cursor: null, has_more: false } },
}))

route('POST', '/api/v1/me/tokens', ({ store, body }) => {
  const b = body as {
    name: string
    tenant_id: string
    role: ApiToken['role']
    expires_at?: string | null
  }
  if (!b?.name)
    return problem(400, 'validation_failed', 'name is required', {
      validation: {
        errors: [
          { path: 'name', code: 'REQUIRED', severity: 'ERROR', message: 'Name is required' },
        ],
      },
    })
  const tenant = Object.values(store.tenants).find((t) => t.tenant.id === b.tenant_id)
  if (!tenant)
    return problem(400, 'validation_failed', 'unknown tenant', {
      validation: {
        errors: [
          { path: 'tenant_id', code: 'NOT_FOUND', severity: 'ERROR', message: 'Tenant not found' },
        ],
      },
    })
  const prefix = `stc_${Math.random().toString(36).slice(2, 6)}`
  const token: ApiToken = {
    id: uuid(),
    name: b.name,
    prefix,
    kind: 'personal',
    tenant: { id: tenant.tenant.id, name: tenant.tenant.name },
    role: b.role ?? 'member',
    owner: { id: store.me.id, display_name: store.me.display_name },
    expires_at: b.expires_at ?? null,
    last_used_at: null,
    created_at: iso(),
  }
  store.personalTokens.unshift(token)
  return { status: 201, json: { ...token, secret: `${prefix}_${uuid().replace(/-/g, '')}` } }
})

route('DELETE', '/api/v1/me/tokens/:id', ({ store, params }) => {
  const i = store.personalTokens.findIndex((t) => t.id === params.id)
  if (i < 0) return notFound('token')
  store.personalTokens.splice(i, 1)
  return noContent()
})

route('GET', '/api/v1/me/invites', ({ store }) => ({
  json: { data: store.myInvites, meta: { next_cursor: null, has_more: false } },
}))

route('POST', '/api/v1/me/invites/:id:accept', ({ store, params }) => {
  const inv = store.myInvites.find((i) => i.id === params.id)
  if (!inv) return notFound('invite')
  inv.status = 'accepted'
  if (!store.me.tenants.some((m) => m.tenant.id === inv.tenant.id))
    store.me.tenants.push({ tenant: inv.tenant, role: inv.role, joined_at: iso() })
  return { json: { tenant: inv.tenant, role: inv.role, joined_at: iso() } }
})

route('POST', '/api/v1/me/invites/:id:decline', ({ store, params }) => {
  const inv = store.myInvites.find((i) => i.id === params.id)
  if (!inv) return notFound('invite')
  inv.status = 'declined'
  return noContent()
})
