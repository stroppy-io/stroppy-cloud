import type { AdminStatus, Run, Schemas, SystemSettings } from '@api/types'
import { noContent, notFound, problem, route } from '../router'
import { sortRuns } from '../run-sort'
import { getSimulation } from '../simulation'
import type { MockStore, TenantData } from '../store'
import { daysAgo, hoursAgo, iso, minutesAgo, multi, paginate, parseListQuery } from '../util'

type AdminTenant = Schemas['AdminTenant']
type Namespace = AdminStatus['pipelines']['namespaces'][number]
type SettingsPatch = Schemas['SystemSettingsPatch']

const STARTED_AT = hoursAgo(31)
const EXPECTED_REVISION = 'a91c3f0'

// Mutable per-namespace push state (not part of TenantData).
const namespaces: Record<string, Namespace> = {
  'st-main': {
    namespace: 'st-main',
    tenant_slug: 'main',
    status: 'synced',
    revision: EXPECTED_REVISION,
    pushed_at: hoursAgo(5),
  },
  'st-sandbox': {
    namespace: 'st-sandbox',
    tenant_slug: 'sandbox',
    status: 'behind',
    revision: '7be2d11',
    pushed_at: daysAgo(3),
  },
  'st-acme': {
    namespace: 'st-acme',
    tenant_slug: 'acme',
    status: 'failed',
    revision: '7be2d11',
    pushed_at: daysAgo(1),
    error: 'graphene: namespace quota exceeded (pipelines: 12/12)',
  },
}
const suspendedReason: Record<string, string> = {}

const isLive = (r: Run) =>
  r.status === 'running' || r.status === 'pending' || r.status === 'cancelling'

function adminTenant(t: TenantData): AdminTenant {
  const runs = t.runs
  const last = [...runs].sort((a, b) => (a.created_at < b.created_at ? 1 : -1))[0]
  return {
    ...t.tenant,
    suspended_reason: suspendedReason[t.tenant.slug],
    counters: {
      members: t.members.length,
      runs_total: runs.length,
      runs_running: runs.filter(isLive).length,
      kept_stands: runs.filter((r) => r.stand_kept).length,
      providers: t.providers.length,
    },
    last_activity_at: last?.created_at ?? t.audit[0]?.at ?? null,
  }
}

function ensureNamespace(t: TenantData): Namespace {
  const ns = `st-${t.tenant.slug}`
  if (!namespaces[ns])
    namespaces[ns] = {
      namespace: ns,
      tenant_slug: t.tenant.slug,
      status: 'pending',
      revision: undefined,
      pushed_at: null,
    }
  return namespaces[ns]
}

function status(store: MockStore): AdminStatus {
  const tenants = Object.values(store.tenants)
  for (const t of tenants) ensureNamespace(t)
  const runs = tenants.flatMap((t) => t.runs)
  return {
    version: store.version,
    commit: EXPECTED_REVISION,
    started_at: STARTED_AT,
    components: {
      graphene: { status: 'ok', version: '1.8.2', detail: 'door: 2 namespaces behind' },
      kratos: { status: 'ok', detail: 'mock mode (static bearer)' },
      victoria: { status: 'degraded', detail: 'vmstorage disk 84% full', version: '1.113.0' },
      postgres: { status: 'ok', version: '17.4' },
    },
    pipelines: {
      expected_revision: EXPECTED_REVISION,
      namespaces: Object.values(namespaces),
    },
    runs: {
      running: runs.filter((r) => r.status === 'running' || r.status === 'cancelling').length,
      pending: runs.filter((r) => r.status === 'pending').length,
      kept_stands: runs.filter((r) => r.stand_kept).length,
    },
    tenants: {
      active: tenants.filter((t) => t.tenant.status === 'active').length,
      suspended: tenants.filter((t) => t.tenant.status === 'suspended').length,
      orphaned: tenants.filter((t) => t.tenant.status === 'orphaned').length,
    },
  }
}

route('GET', '/api/v1/admin/status', ({ store }) => ({ json: status(store) }))

route('POST', '/api/v1/admin/pipelines:resync', ({ store, body }) => {
  const slug = (body as { tenant_slug?: string } | null)?.tenant_slug
  const targets = Object.values(namespaces).filter((n) => !slug || n.tenant_slug === slug)
  if (slug && !targets.length) return notFound('namespace')
  for (const n of targets) {
    n.status = 'synced'
    n.revision = EXPECTED_REVISION
    n.pushed_at = iso()
    n.error = undefined
  }
  store.audit(
    undefined,
    'admin.pipelines.resync',
    { kind: 'namespace', name: slug ?? 'all' },
    {
      namespaces: targets.map((n) => n.namespace),
    }
  )
  return { status: 202, json: { namespaces: targets.map((n) => n.namespace) } }
})

// ---------- tenants ----------

route('GET', '/api/v1/admin/tenants', ({ store, query }) => {
  const lq = parseListQuery(query)
  const st = query.get('status')
  let items = Object.values(store.tenants).map(adminTenant)
  if (st) items = items.filter((t) => t.status === st)
  if (lq.search) {
    const s = lq.search.toLowerCase()
    items = items.filter((t) =>
      `${t.name} ${t.slug} ${t.description ?? ''} ${t.owner.display_name ?? ''}`
        .toLowerCase()
        .includes(s)
    )
  }
  items.sort((a, b) => (a.created_at < b.created_at ? 1 : -1))
  return { json: paginate(items, lq) }
})

route('GET', '/api/v1/admin/tenants/:slug', ({ store, params }) => {
  const t = store.tenant(params.slug)
  return t ? { json: adminTenant(t) } : notFound('tenant')
})

route('DELETE', '/api/v1/admin/tenants/:slug', ({ store, params }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  if (t.runs.some(isLive))
    return problem(
      409,
      'tenant_has_live_runs',
      'Cancel or wait for running runs before deleting the tenant.'
    )
  delete store.tenants[params.slug]
  delete namespaces[`st-${params.slug}`]
  store.me.tenants = store.me.tenants.filter((m) => m.tenant.slug !== params.slug)
  if (store.me.owned_tenant_id === t.tenant.id) store.me.owned_tenant_id = null
  for (const u of store.users) if (u.owned_tenant?.id === t.tenant.id) u.owned_tenant = undefined
  store.audit(undefined, 'admin.tenant.delete', {
    kind: 'tenant',
    id: t.tenant.id,
    name: t.tenant.name,
  })
  return noContent()
})

route('POST', '/api/v1/admin/tenants/:slug:suspend', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  if (t.tenant.status === 'suspended') return problem(409, 'already_suspended', 'Already suspended')
  t.tenant.status = 'suspended'
  const reason = (body as { reason?: string } | null)?.reason?.trim()
  if (reason) suspendedReason[params.slug] = reason
  else delete suspendedReason[params.slug]
  // Suspension cancels the queue: live runs are cancelled.
  for (const r of t.runs) if (isLive(r)) getSimulation(store).cancelRun(params.slug, r)
  store.audit(
    params.slug,
    'admin.tenant.suspend',
    { kind: 'tenant', id: t.tenant.id, name: t.tenant.name },
    reason ? { reason } : undefined
  )
  return { json: adminTenant(t) }
})

route('POST', '/api/v1/admin/tenants/:slug:resume', ({ store, params }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  if (t.tenant.status !== 'suspended')
    return problem(409, 'not_suspended', 'Tenant is not suspended')
  t.tenant.status = t.members.some((m) => m.role === 'owner') ? 'active' : 'orphaned'
  delete suspendedReason[params.slug]
  store.audit(params.slug, 'admin.tenant.resume', {
    kind: 'tenant',
    id: t.tenant.id,
    name: t.tenant.name,
  })
  return { json: adminTenant(t) }
})

route('POST', '/api/v1/admin/tenants/:slug:assign-owner', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const userId = (body as { user_id?: string } | null)?.user_id
  const user = store.users.find((u) => u.id === userId)
  if (!user)
    return problem(400, 'validation_failed', 'unknown user', {
      validation: {
        errors: [
          {
            path: 'user_id',
            code: 'NOT_FOUND',
            severity: 'ERROR',
            message: 'Pick an existing user',
          },
        ],
      },
    })
  if (user.owned_tenant && user.owned_tenant.id !== t.tenant.id)
    return problem(
      409,
      'tenant_already_owned',
      `${user.display_name} already owns “${user.owned_tenant.name}”. One owned tenant per account.`,
      {
        validation: {
          errors: [
            {
              path: 'user_id',
              code: 'ALREADY_OWNER',
              severity: 'ERROR',
              message: `Already owns “${user.owned_tenant.name}” — one owned tenant per account`,
            },
          ],
        },
      }
    )
  const prev = t.members.find((m) => m.role === 'owner')
  if (prev) {
    prev.role = 'admin'
    const pu = store.users.find((u) => u.id === prev.user.id)
    if (pu?.owned_tenant?.id === t.tenant.id) pu.owned_tenant = undefined
    if (prev.user.id === store.me.id) store.me.owned_tenant_id = null
  }
  const member = t.members.find((m) => m.user.id === user.id)
  if (member) member.role = 'owner'
  else
    t.members.push({
      user: { id: user.id, display_name: user.display_name, email: user.email },
      role: 'owner',
      joined_at: iso(),
    })
  t.tenant.member_count = t.members.length
  t.tenant.owner = { id: user.id, display_name: user.display_name }
  if (t.tenant.status === 'orphaned') t.tenant.status = 'active'
  user.owned_tenant = { id: t.tenant.id, name: t.tenant.name }
  const mine = store.me.tenants.find((m) => m.tenant.slug === params.slug)
  if (user.id === store.me.id) {
    store.me.owned_tenant_id = t.tenant.id
    if (mine) mine.role = 'owner'
    else store.me.tenants.push({ tenant: t.tenant, role: 'owner', joined_at: iso() })
  } else if (mine && prev?.user.id === store.me.id) mine.role = 'admin'
  store.audit(params.slug, 'admin.tenant.assign_owner', {
    kind: 'user',
    id: user.id,
    name: user.display_name,
  })
  return { json: adminTenant(t) }
})

// ---------- users ----------

route('GET', '/api/v1/admin/users', ({ store, query }) => {
  const lq = parseListQuery(query)
  const pa = query.get('platform_admin')
  let items = store.users.map((u) => ({
    ...u,
    memberships: Object.values(store.tenants).filter((t) =>
      t.members.some((m) => m.user.id === u.id)
    ).length,
  }))
  if (pa === 'true') items = items.filter((u) => u.is_platform_admin)
  if (pa === 'false') items = items.filter((u) => !u.is_platform_admin)
  if (lq.search) {
    const s = lq.search.toLowerCase()
    items = items.filter((u) => `${u.display_name} ${u.email}`.toLowerCase().includes(s))
  }
  return { json: paginate(items, lq) }
})

route('PATCH', '/api/v1/admin/users/:userId', ({ store, params, body }) => {
  const u = store.users.find((x) => x.id === params.userId)
  if (!u) return notFound('user')
  const b = (body ?? {}) as { is_platform_admin?: boolean }
  if (b.is_platform_admin !== undefined) {
    if (u.admin_source === 'config')
      return problem(
        409,
        'admin_from_config',
        'This admin is granted by the server config (STROPPY_ADMIN_EMAILS) and cannot be changed here.'
      )
    u.is_platform_admin = b.is_platform_admin
    u.admin_source = b.is_platform_admin ? 'db' : undefined
    if (u.id === store.me.id) store.me.is_platform_admin = b.is_platform_admin
  }
  store.audit(
    undefined,
    'admin.user.platform_admin',
    { kind: 'user', id: u.id, name: u.display_name },
    b
  )
  return { json: u }
})

// ---------- system settings ----------

route('GET', '/api/v1/admin/settings', ({ store }) => ({ json: store.system }))

route('PATCH', '/api/v1/admin/settings', ({ store, body }) => {
  const b = (body ?? {}) as SettingsPatch
  const s: SystemSettings = { ...store.system }
  if (b.tenant_creation !== undefined) s.tenant_creation = b.tenant_creation
  if (b.public_rating_enabled !== undefined) s.public_rating_enabled = b.public_rating_enabled
  if (b.examples_enabled !== undefined) s.examples_enabled = b.examples_enabled
  if (b.stroppy_catalog !== undefined) s.stroppy_catalog = b.stroppy_catalog
  s.updated_at = iso()
  s.updated_by = { id: store.me.id, display_name: store.me.display_name }
  store.system = s
  store.audit(
    undefined,
    'admin.settings.update',
    { kind: 'system_settings' },
    b as Record<string, unknown>
  )
  return { json: s }
})

// ---------- global run queue ----------

route('GET', '/api/v1/admin/runs', ({ store, query }) => {
  const lq = parseListQuery(query, 'default')
  const st = multi(query, 'status')
  const tenant = query.get('tenant')
  let items = Object.values(store.tenants).flatMap((t) =>
    t.runs.map((r) => ({
      ...r,
      tenant: { id: t.tenant.id, name: t.tenant.name, slug: t.tenant.slug },
    }))
  )
  if (tenant) items = items.filter((r) => r.tenant.slug === tenant || r.tenant.id === tenant)
  if (st.length) items = items.filter((r) => st.includes(r.status))
  else items = items.filter(isLive)
  // `default` here: live first, then the newest (favorites are per tenant, not global).
  return { json: paginate(sortRuns(items, lq.sort, lq.order), lq) }
})

route('POST', '/api/v1/admin/runs/:id:cancel', ({ store, params }) => {
  for (const t of Object.values(store.tenants)) {
    const run = t.runs.find((r) => r.id === params.id)
    if (!run) continue
    if (!isLive(run)) return problem(409, 'run_not_live', `Run is already ${run.status}`)
    getSimulation(store).cancelRun(t.tenant.slug, run)
    store.audit(t.tenant.slug, 'admin.run.cancel', { kind: 'run', id: run.id, name: run.name })
    return { json: run }
  }
  return notFound('run')
})

// ---------- audit ----------

route('GET', '/api/v1/admin/audit', ({ store, query }) => {
  const lq = parseListQuery(query)
  const tenant = query.get('tenant')
  const action = query.get('action')
  const actor = query.get('actor')
  const since = query.get('since')
  let items = store.adminAudit
  if (tenant) items = items.filter((a) => a.tenant?.id === tenant || a.tenant?.name === tenant)
  if (action) items = items.filter((a) => a.action.startsWith(action))
  if (actor)
    items = items.filter(
      (a) =>
        a.actor.id === actor || a.actor.display_name?.toLowerCase().includes(actor.toLowerCase())
    )
  if (since) items = items.filter((a) => a.at >= since)
  return { json: paginate(items, lq) }
})

// Keep the seed's freshness marker referenced so fixtures share the same clock helpers.
void minutesAgo
