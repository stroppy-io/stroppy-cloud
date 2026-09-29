import type { ApiToken, Invite, Tenant, TenantRole } from '@api/types'
import { noContent, notFound, problem, route } from '../router'
import type { TenantData } from '../store'
import { daysAgo, iso, paginate, parseListQuery, uuid } from '../util'

const RANK: Record<TenantRole, number> = { viewer: 0, member: 1, admin: 2, owner: 3 }

function slugify(name: string): string {
  return (
    name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '')
      .slice(0, 40) || 'tenant'
  )
}

route('GET', '/api/v1/tenants', ({ store }) => ({
  json: { data: store.me.tenants, meta: { next_cursor: null, has_more: false } },
}))

route('GET', '/api/v1/tenants/suggest-name', ({ store, query }) => {
  const base =
    query.get('name') ?? `${store.me.display_name.toLowerCase().replace(/\s+/g, '-')}-lab`
  let slug = slugify(base)
  let n = 1
  while (store.tenants[slug]) slug = `${slugify(base)}-${++n}`
  return { json: { name: base, slug, available: true } }
})

route('POST', '/api/v1/tenants', ({ store, body }) => {
  const b = body as { name: string; slug?: string; description?: string }
  if (!b?.name?.trim())
    return problem(400, 'validation_failed', 'name required', {
      validation: {
        errors: [
          { path: 'name', code: 'REQUIRED', severity: 'ERROR', message: 'Name is required' },
        ],
      },
    })
  if (store.me.owned_tenant_id)
    return problem(
      409,
      'tenant_already_owned',
      'You already own a tenant. Only one owned tenant per account.'
    )
  const slug = b.slug?.trim() ? slugify(b.slug) : slugify(b.name)
  if (store.tenants[slug])
    return problem(409, 'slug_taken', `slug ${slug} is taken`, {
      validation: {
        errors: [
          {
            path: 'slug',
            code: 'TAKEN',
            severity: 'ERROR',
            message: `“${slug}” is already used — pick another name`,
          },
        ],
      },
    })
  const tenant: Tenant = {
    id: uuid(),
    slug,
    name: b.name.trim(),
    description: b.description,
    status: 'active',
    owner: { id: store.me.id, display_name: store.me.display_name },
    member_count: 1,
    created_at: iso(),
  }
  const data: TenantData = {
    tenant,
    members: [
      {
        user: { id: store.me.id, display_name: store.me.display_name, email: store.me.email },
        role: 'owner',
        joined_at: iso(),
      },
    ],
    invites: [],
    tokens: [],
    providers: [],
    quotas: {},
    webhooks: [],
    deliveries: {},
    databases: [],
    workloads: [],
    tests: [],
    runs: [],
    runLive: {},
    suites: [],
    suiteRuns: [],
    schedules: [],
    shares: [],
    audit: [],
    favorites: new Set(),
    settings: {
      run_retention_days: 30,
      default_rating: { tenant: true, global: false },
      default_keep: '0s',
    },
    limits: { ...store.system.default_limits, source: 'platform_default' },
  }
  store.tenants[slug] = data
  store.me.tenants.push({ tenant, role: 'owner', joined_at: iso() })
  store.me.owned_tenant_id = tenant.id
  store.audit(slug, 'tenant.create', { kind: 'tenant', id: tenant.id, name: tenant.name })
  return { status: 201, json: tenant }
})

route('GET', '/api/v1/tenants/:slug', ({ store, params }) => {
  const t = store.tenant(params.slug)
  return t ? { json: t.tenant } : notFound('tenant')
})

route('PATCH', '/api/v1/tenants/:slug', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const b = body as Partial<Tenant>
  if (b.name !== undefined) t.tenant.name = b.name
  if (b.description !== undefined) t.tenant.description = b.description
  if (b.public_name !== undefined) t.tenant.public_name = b.public_name
  store.audit(
    params.slug,
    'tenant.update',
    { kind: 'tenant', id: t.tenant.id, name: t.tenant.name },
    b as Record<string, unknown>
  )
  return { json: t.tenant }
})

route('DELETE', '/api/v1/tenants/:slug', ({ store, params }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  if (
    t.runs.some(
      (r) => r.status === 'running' || r.status === 'pending' || r.status === 'cancelling'
    )
  )
    return problem(
      409,
      'tenant_has_live_runs',
      'Cancel or wait for running runs before deleting the tenant.'
    )
  delete store.tenants[params.slug]
  store.me.tenants = store.me.tenants.filter((m) => m.tenant.slug !== params.slug)
  if (store.me.owned_tenant_id === t.tenant.id) store.me.owned_tenant_id = null
  return noContent()
})

route('GET', '/api/v1/tenants/:slug/members', ({ store, params }) => {
  const t = store.tenant(params.slug)
  return t
    ? { json: { data: t.members, meta: { next_cursor: null, has_more: false } } }
    : notFound('tenant')
})

route('PATCH', '/api/v1/tenants/:slug/members/:userId', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const m = t.members.find((x) => x.user.id === params.userId)
  if (!m) return notFound('member')
  const role = (body as { role: TenantRole }).role
  if (m.role === 'owner')
    return problem(
      409,
      'owner_role_immutable',
      'Transfer ownership instead of changing the owner role.'
    )
  if (role === 'owner')
    return problem(400, 'validation_failed', 'use :transfer to change the owner')
  m.role = role
  store.audit(
    params.slug,
    'member.role',
    { kind: 'user', id: m.user.id, name: m.user.display_name },
    { role }
  )
  return { json: m }
})

route('DELETE', '/api/v1/tenants/:slug/members/:userId', ({ store, params }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const m = t.members.find((x) => x.user.id === params.userId)
  if (!m) return notFound('member')
  if (m.role === 'owner')
    return problem(
      409,
      'owner_cannot_leave',
      'The owner cannot be removed. Transfer ownership first.'
    )
  t.members = t.members.filter((x) => x !== m)
  t.tenant.member_count = t.members.length
  if (m.user.id === store.me.id)
    store.me.tenants = store.me.tenants.filter((x) => x.tenant.slug !== params.slug)
  store.audit(params.slug, 'member.remove', {
    kind: 'user',
    id: m.user.id,
    name: m.user.display_name,
  })
  return noContent()
})

route('POST', '/api/v1/tenants/:slug:transfer', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const to = (body as { user_id: string }).user_id
  const target = t.members.find((m) => m.user.id === to)
  if (!target)
    return problem(400, 'validation_failed', 'user is not a member', {
      validation: {
        errors: [
          {
            path: 'user_id',
            code: 'NOT_MEMBER',
            severity: 'ERROR',
            message: 'Pick an existing member',
          },
        ],
      },
    })
  const owner = t.members.find((m) => m.role === 'owner')
  if (owner) owner.role = 'admin'
  target.role = 'owner'
  t.tenant.owner = { id: target.user.id, display_name: target.user.display_name }
  const mine = store.me.tenants.find((m) => m.tenant.slug === params.slug)
  if (mine) mine.role = target.user.id === store.me.id ? 'owner' : 'admin'
  store.audit(params.slug, 'tenant.transfer', {
    kind: 'user',
    id: target.user.id,
    name: target.user.display_name,
  })
  return { json: t.tenant }
})

route('GET', '/api/v1/tenants/:slug/invites', ({ store, params }) => {
  const t = store.tenant(params.slug)
  return t
    ? { json: { data: t.invites, meta: { next_cursor: null, has_more: false } } }
    : notFound('tenant')
})

route('POST', '/api/v1/tenants/:slug/invites', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const b = body as { email: string; role: TenantRole; message?: string }
  if (!b?.email || !/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(b.email))
    return problem(400, 'validation_failed', 'invalid email', {
      validation: {
        errors: [
          {
            path: 'email',
            code: 'FORMAT',
            severity: 'ERROR',
            message: 'Enter a valid e-mail, e.g. name@company.com',
          },
        ],
      },
    })
  if (t.members.some((m) => m.user.email === b.email))
    return problem(409, 'already_member', 'This person is already a member', {
      validation: {
        errors: [
          {
            path: 'email',
            code: 'ALREADY_MEMBER',
            severity: 'ERROR',
            message: 'Already a member of this tenant',
          },
        ],
      },
    })
  if (RANK[b.role] >= RANK.owner) return problem(400, 'validation_failed', 'cannot invite as owner')
  const inv: Invite = {
    id: uuid(),
    tenant: t.tenant,
    email: b.email,
    role: b.role,
    status: 'pending',
    invited_by: { id: store.me.id, display_name: store.me.display_name },
    message: b.message,
    created_at: iso(),
    expires_at: iso(Date.now() + 7 * 86400_000),
  }
  t.invites.unshift(inv)
  store.audit(
    params.slug,
    'member.invite',
    { kind: 'invite', id: inv.id, name: inv.email },
    { role: inv.role }
  )
  return { status: 201, json: inv }
})

route('DELETE', '/api/v1/tenants/:slug/invites/:id', ({ store, params }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const inv = t.invites.find((i) => i.id === params.id)
  if (!inv) return notFound('invite')
  inv.status = 'revoked'
  return noContent()
})

route('GET', '/api/v1/tenants/:slug/tokens', ({ store, params }) => {
  const t = store.tenant(params.slug)
  return t
    ? { json: { data: t.tokens, meta: { next_cursor: null, has_more: false } } }
    : notFound('tenant')
})

route('POST', '/api/v1/tenants/:slug/tokens', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const b = body as { name: string; role: 'member' | 'viewer'; expires_at?: string | null }
  if (!b?.name?.trim())
    return problem(400, 'validation_failed', 'name required', {
      validation: {
        errors: [
          { path: 'name', code: 'REQUIRED', severity: 'ERROR', message: 'Name is required' },
        ],
      },
    })
  const prefix = `stc_${Math.random().toString(36).slice(2, 6)}`
  const tok: ApiToken = {
    id: uuid(),
    name: b.name,
    prefix,
    kind: 'service',
    tenant: { id: t.tenant.id, name: t.tenant.name },
    role: b.role ?? 'member',
    expires_at: b.expires_at ?? null,
    last_used_at: null,
    created_at: iso(),
  }
  t.tokens.unshift(tok)
  store.audit(params.slug, 'token.create', { kind: 'token', id: tok.id, name: tok.name })
  return { status: 201, json: { ...tok, secret: `${prefix}_${uuid().replace(/-/g, '')}` } }
})

route('DELETE', '/api/v1/tenants/:slug/tokens/:id', ({ store, params }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const i = t.tokens.findIndex((x) => x.id === params.id)
  if (i < 0) return notFound('token')
  store.audit(params.slug, 'token.revoke', { kind: 'token', id: params.id, name: t.tokens[i].name })
  t.tokens.splice(i, 1)
  return noContent()
})

route('GET', '/api/v1/t/:slug/audit', ({ store, params, query }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const lq = parseListQuery(query)
  const action = query.get('action')
  const actor = query.get('actor')
  const since = query.get('since')
  let items = t.audit
  if (action) items = items.filter((a) => a.action.startsWith(action))
  if (actor)
    items = items.filter(
      (a) =>
        a.actor.id === actor || a.actor.display_name?.toLowerCase().includes(actor.toLowerCase())
    )
  if (since) items = items.filter((a) => a.at >= since)
  return { json: paginate(items, lq) }
})

// used by tenant creation flow demo: seed some entries for brand-new tenants
export function seedNewTenantAudit(t: TenantData): void {
  t.audit.push({
    id: uuid(),
    at: daysAgo(0),
    actor: { kind: 'system' },
    action: 'tenant.bootstrap',
    target: { kind: 'tenant', id: t.tenant.id },
  })
}
