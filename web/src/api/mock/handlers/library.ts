import type {
  CatalogDatabase,
  DatabaseSpec,
  LaunchOverrides,
  Run,
  Schemas,
  TestValidation,
  ValidationError,
  WorkloadSpec,
} from '@api/types'
import { type HandlerResult, listRoutes, noContent, notFound, problem, route } from '../router'
import { launchFromTest } from '../run-factory'
import { sortRuns } from '../run-sort'
import { SIZES } from '../seed'
import type { Database, MockStore, TenantData, Test, Workload } from '../store'
import { iso, matchesSearch, multi, paginate, parseListQuery, sortBy, uuid } from '../util'
import { findSchema } from './catalog'

type Requirements = Schemas['Requirements']
type TopologyPreview = Schemas['TopologyPreview']
type Fit = Schemas['Fit']
type Issue = NonNullable<Fit['issues']>[number]
type ExportDocument = Schemas['ExportDocument']
type Diff = Schemas['Diff']
type Size = Schemas['Size']
type Usage = Schemas['Usage']

// ---------- small helpers ----------
const num = (v: unknown, d = 0): number => {
  const n = typeof v === 'string' ? Number.parseFloat(v) : typeof v === 'number' ? v : Number.NaN
  return Number.isFinite(n) ? n : d
}
const str = (v: unknown): string | undefined =>
  typeof v === 'string' || typeof v === 'number' ? String(v) : undefined

function author(store: MockStore) {
  return { id: store.me.id, display_name: store.me.display_name }
}

function validationProblem(errors: ValidationError[], detail = 'validation failed') {
  return problem(400, 'validation_failed', detail, { validation: { errors } })
}

function err(path: string, code: string, message: string, severity: 'ERROR' | 'WARNING' = 'ERROR') {
  return { path, code, severity, message }
}

// Go-duration ("2h", "30m", "1h30m", "0s") → seconds; NaN when malformed.
function durationSec(s: string | undefined | null): number {
  if (s === undefined || s === null || s === '') return 0
  const parts = s.match(/(\d+(?:\.\d+)?)(h|m|s)/g)
  if (!parts || parts.join('') !== s) return Number.NaN
  let sec = 0
  for (const p of parts) {
    const [, v, u] = p.match(/(\d+(?:\.\d+)?)(h|m|s)/) ?? []
    sec += Number(v) * (u === 'h' ? 3600 : u === 'm' ? 60 : 1)
  }
  return sec
}

// ---------- topology / requirements derived from params (mirrors the seed fixtures) ----------
type Node = TopologyPreview['nodes'][number]
type Flow = NonNullable<TopologyPreview['flows']>[number]

export function deriveDatabase(
  catalog: CatalogDatabase | undefined,
  kind: string,
  params: Record<string, unknown>
): { topology_preview: TopologyPreview; requirements: Requirements } {
  const nodes: Node[] = []
  const flows: Flow[] = []
  const req: Requirements = {}
  let label = 'single'
  const engine = (role: string) => catalog?.roles.find((r) => r.role === role)?.engine ?? kind
  const proxyRole = (role: string, count: number, to: string, protocol: string, port: number) => {
    if (count <= 0) return
    nodes.push({ role, engine: engine(role), count })
    flows.push({ from: role, to, protocol, port })
    req.proxy = { cpu: 2, memory_gb: 4, reason: `${role} ×${count}` }
  }

  switch (kind) {
    case 'postgres':
    case 'orioledb': {
      const replicas = num(params.replicas)
      const ha = str(params.ha) ?? 'none'
      const haproxy = num(params.haproxy)
      const etcd = ha === 'patroni' ? num(params.etcd_nodes, 3) : 0
      nodes.push({ role: 'master', engine: engine('master'), count: 1 })
      if (replicas > 0) {
        nodes.push({ role: 'replica', engine: engine('replica'), count: replicas })
        flows.push({ from: 'master', to: 'replica', protocol: 'pg', port: 5432 })
      }
      if (etcd > 0) {
        nodes.push({ role: 'etcd', engine: 'etcd', count: etcd })
        flows.push({ from: 'master', to: 'etcd', protocol: 'http', port: 2379 })
        req.etcd = { cpu: 2, memory_gb: 4, reason: `etcd quorum ×${etcd}` }
      }
      proxyRole('haproxy', haproxy, 'master', 'pg', 5432)
      if (params.pgbouncer === true)
        nodes.push({
          role: 'pgbouncer',
          engine: 'pgbouncer',
          count: 1 + replicas,
          colocated_with: 'master',
        })
      label = ha === 'patroni' ? 'patroni-ha' : replicas > 0 ? 'primary-replica' : 'single'
      const sbMb = num(params.shared_buffers_mb, 0)
      const memFromSb = sbMb ? Math.max(4, Math.ceil((sbMb / 1024) * 4)) : 0
      req.db =
        ha === 'patroni'
          ? {
              cpu: 8,
              memory_gb: Math.max(32, memFromSb),
              disk_gb: 200,
              reason: 'shared_buffers 8GB',
            }
          : replicas > 0
            ? {
                cpu: 4,
                memory_gb: Math.max(16, memFromSb),
                disk_gb: 80,
                reason: 'shared_buffers 4GB',
              }
            : {
                cpu: 2,
                memory_gb: Math.max(4, memFromSb),
                disk_gb: 40,
                reason: 'shared_buffers 1GB',
              }
      break
    }
    case 'mysql':
    case 'mariadb': {
      const replication = str(params.replication) ?? 'async'
      const galera = params.galera === true || replication === 'galera'
      const galeraNodes = num(params.galera_nodes, 3)
      const replicas = galera ? galeraNodes - 1 : num(params.replicas)
      nodes.push({ role: 'master', engine: engine('master'), count: 1 })
      if (replicas > 0) {
        nodes.push({ role: 'replica', engine: engine('replica'), count: replicas })
        flows.push({ from: 'master', to: 'replica', protocol: 'mysql', port: 3306 })
      }
      proxyRole('proxysql', num(params.proxysql), 'master', 'mysql', 6033)
      if (params.maxscale === true) proxyRole('maxscale', 1, 'master', 'mysql', 4006)
      label = galera
        ? 'galera'
        : replication === 'group'
          ? 'group-replication'
          : replicas > 0
            ? 'source-replica'
            : 'single'
      req.db =
        replicas > 0
          ? { cpu: 4, memory_gb: 16, disk_gb: 80, reason: 'innodb_buffer_pool 8GB' }
          : { cpu: 2, memory_gb: 4, disk_gb: 40, reason: 'innodb_buffer_pool 1GB' }
      break
    }
    case 'picodata': {
      const tiers = Array.isArray(params.tiers) ? (params.tiers as Record<string, unknown>[]) : []
      const fromTiers = tiers.reduce(
        (acc, t) => acc + num(t.replicasets, 1) * num(t.replication_factor, 1),
        0
      )
      const instances = num(params.instances, fromTiers || 1)
      nodes.push({ role: 'instance', engine: 'picodata', count: instances })
      proxyRole('haproxy', num(params.haproxy), 'instance', 'pg', 4327)
      label = instances > 1 ? `cluster-${instances}` : 'single'
      const memtx = num(params.memtx_memory_mb, 2048)
      req.db = {
        cpu: instances > 1 ? 4 : 2,
        memory_gb: Math.max(instances > 1 ? 16 : 4, Math.ceil((memtx * 2) / 1024)),
        disk_gb: 60,
        reason: `memtx ${Math.round(memtx / 1024)}GB`,
      }
      break
    }
    case 'ydb': {
      const storage = num(params.storage_nodes, num(params.nodes, 1))
      const database = num(params.database_nodes, Math.min(3, storage))
      const ft = str(params.erasure) ?? str(params.fault_tolerance) ?? 'none'
      const pdisks = num(params.pdisks_per_node, 1)
      nodes.push({ role: 'storage', engine: 'ydb', count: storage })
      nodes.push({ role: 'database', engine: 'ydb', count: database, colocated_with: 'storage' })
      proxyRole('haproxy', num(params.haproxy), 'database', 'grpc', 2135)
      label = ft !== 'none' ? ft : storage > 1 ? `cluster-${storage}` : 'single'
      req.db =
        ft === 'mirror-3-dc'
          ? {
              cpu: 8,
              memory_gb: 32,
              disk_gb: 93 * Math.max(3, pdisks),
              reason: `${pdisks} pdisks × 93GB`,
            }
          : storage > 1
            ? { cpu: 4, memory_gb: 16, disk_gb: 100, reason: `${pdisks} pdisks` }
            : { cpu: 2, memory_gb: 8, disk_gb: 60 }
      break
    }
    case 'cockroach': {
      const n = num(params.nodes, 3)
      nodes.push({ role: 'node', engine: 'cockroach', count: n })
      proxyRole('haproxy', num(params.haproxy), 'node', 'pg', 26257)
      label = n > 1 ? `cluster-${n}` : 'single'
      req.db = { cpu: 4, memory_gb: n > 1 ? 24 : 16, disk_gb: 100, reason: 'cockroach cache 25%' }
      break
    }
    case 'ydb_managed': {
      const n = str(params.type) === 'serverless' ? 0 : num(params.node_count, 3)
      nodes.push({ role: 'managed', engine: 'ydb', count: n })
      label = str(params.type) ?? 'managed'
      break
    }
    default: {
      const role = catalog?.roles[0]?.role ?? kind
      nodes.push({ role, engine: engine(role), count: 1 })
      label = catalog?.topologies[0]?.id ?? kind
    }
  }
  const node_count = nodes.filter((n) => !n.colocated_with).reduce((a, n) => a + n.count, 0)
  return {
    topology_preview: { label, node_count, nodes, flows: flows.length ? flows : undefined },
    requirements: req,
  }
}

// Minimal schemapb default resolver for effective configs (mock only; the UI uses schema/engine.ts).
function schemaDefaults(fields: Record<string, unknown>[] | undefined): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const f of fields ?? []) {
    const name = f.name as string
    const pick = (k: string) => (f[k] as { default?: unknown } | undefined)?.default
    if (pick('string') !== undefined) out[name] = pick('string')
    else if (pick('int64') !== undefined) out[name] = Number(pick('int64'))
    else if (pick('uint64') !== undefined) out[name] = Number(pick('uint64'))
    else if (pick('double') !== undefined) out[name] = pick('double')
    else if (pick('bool') !== undefined) out[name] = pick('bool')
    else if (pick('duration') !== undefined) out[name] = pick('duration')
    else if (f.choice && (f.choice as { default?: Record<string, unknown> }).default) {
      const d = (f.choice as { default: Record<string, unknown> }).default
      out[name] = d.stringValue ?? (d.int64Value !== undefined ? Number(d.int64Value) : d.boolValue)
    }
  }
  return out
}

function configSchemaId(schemaBase: string, version: string): string {
  const major = version.split('.')[0]
  return `${schemaBase}@${major}`
}

function effectiveConfigs(
  catalog: CatalogDatabase | undefined,
  spec: DatabaseSpec
): Record<string, Record<string, Record<string, unknown>>> {
  const out: Record<string, Record<string, Record<string, unknown>>> = {}
  const topo = deriveDatabase(catalog, spec.kind, spec.params).topology_preview
  for (const n of topo.nodes) {
    const role = catalog?.roles.find((r) => r.role === n.role)
    for (const base of role?.config_schemas ?? []) {
      // Engine configs are versioned by the DB major; side-car software (etcd, haproxy, …) by its own
      // major — the catalog resolves the real schema id, which is the key the UI addresses.
      const requested = configSchemaId(base, spec.version)
      const schema = findSchema(requested)
      const id = schema?.id ?? requested
      const defaults = schemaDefaults(schema?.body.fields as Record<string, unknown>[] | undefined)
      const diff =
        spec.configs?.[n.role]?.[id] ??
        spec.configs?.[n.role]?.[requested] ??
        spec.configs?.[n.role]?.[base] ??
        {}
      out[n.role] = { ...out[n.role], [id]: { ...defaults, ...diff } }
    }
  }
  return out
}

// Basic params validation against the kind schema (required + int bounds).
function validateParams(schemaRef: string, value: Record<string, unknown>): ValidationError[] {
  const s = findSchema(schemaRef)
  const errors: ValidationError[] = []
  for (const f of (s?.body.fields as Record<string, unknown>[] | undefined) ?? []) {
    const name = f.name as string
    const v = value[name]
    if (f.required && !f.computed && (v === undefined || v === null || v === ''))
      errors.push({
        path: `params.${name}`,
        code: 'REQUIRED',
        severity: 'ERROR',
        message: `${(f.title as string) ?? name} is required`,
      })
    const i64 = f.int64 as { gte?: string; lte?: string } | undefined
    if (i64 && v !== undefined && v !== null && v !== '') {
      const n = Number(v)
      if (i64.gte !== undefined && n < Number(i64.gte))
        errors.push({
          path: `params.${name}`,
          code: 'GTE_VIOLATED',
          severity: 'ERROR',
          message: `Must be ≥ ${i64.gte}`,
        })
      if (i64.lte !== undefined && n > Number(i64.lte))
        errors.push({
          path: `params.${name}`,
          code: 'LTE_VIOLATED',
          severity: 'ERROR',
          message: `Must be ≤ ${i64.lte}`,
        })
    }
  }
  return errors
}

// ---------- entity filters shared by the three lists ----------
type Entity = Schemas['Entity']

function matchesTags(tags: Record<string, string> | undefined, filter: string | null): boolean {
  if (!filter) return true
  for (const pair of filter.split(',')) {
    const [k, v] = pair.split('=')
    if (!k) continue
    if (!tags || !(k in tags)) return false
    if (v !== undefined && v !== '' && tags[k] !== v) return false
  }
  return true
}

function filterEntities<T extends Entity>(
  items: T[],
  q: URLSearchParams,
  favorites: Set<string>,
  favKind: string,
  extraText: (e: T) => string = () => ''
): T[] {
  const search = q.get('search') ?? q.get('q')
  const author = q.get('author')
  const favOnly = q.get('favorites') === 'true'
  const tags = q.get('tags')
  return items.filter((e) => {
    if (favOnly && !favorites.has(`${favKind}:${e.id}`)) return false
    if (author && e.author.id !== author && e.author.display_name !== author) return false
    if (!matchesTags(e.tags, tags)) return false
    return matchesSearch(
      `${e.name} ${e.description ?? ''} ${Object.entries(e.tags ?? {})
        .map(([k, v]) => `${k}=${v}`)
        .join(' ')} ${extraText(e)}`,
      search ?? undefined
    )
  })
}

function sortEntities<T extends Entity>(
  items: T[],
  sort: string | undefined,
  order: 'asc' | 'desc',
  extra: (e: T, key: string) => string | number | undefined
): T[] {
  switch (sort) {
    case 'name':
      return sortBy(items, (e) => e.name.toLowerCase(), order)
    case 'created_at':
      return sortBy(items, (e) => e.created_at, order)
    case 'updated_at':
    case undefined:
      return sortBy(items, (e) => e.updated_at, order)
    default:
      return sortBy(items, (e) => extra(e, sort) ?? '', order)
  }
}

// ---------- usages ----------
function usagesOfTest(t: TenantData, id: string): Usage[] {
  const out: Usage[] = []
  for (const s of t.suites)
    if (s.tests?.some((x) => 'ref' in x && x.ref.id === id))
      out.push({ kind: 'suite', id: s.id, name: s.name })
  for (const s of t.schedules)
    if (s.target?.kind === 'test' && s.target.id === id)
      out.push({ kind: 'schedule', id: s.id, name: s.name })
  return out
}

function usagesOfDatabase(t: TenantData, id: string): Usage[] {
  const out: Usage[] = []
  for (const test of t.tests)
    if (test.database && 'ref' in test.database && test.database.ref.id === id) {
      out.push({ kind: 'test', id: test.id, name: test.name })
      for (const u of usagesOfTest(t, test.id))
        if (!out.some((x) => x.kind === u.kind && x.id === u.id)) out.push(u)
    }
  return out
}

function usagesOfWorkload(t: TenantData, id: string): Usage[] {
  const out: Usage[] = []
  for (const test of t.tests)
    if (test.workload && 'ref' in test.workload && test.workload.ref.id === id) {
      out.push({ kind: 'test', id: test.id, name: test.name })
      for (const u of usagesOfTest(t, test.id))
        if (!out.some((x) => x.kind === u.kind && x.id === u.id)) out.push(u)
    }
  return out
}

function inUse(usages: Usage[], what: string) {
  const names = usages.map((u) => `${u.kind} "${u.name}"`).join(', ')
  return problem(
    409,
    'entity_in_use',
    `This ${what} is referenced by ${usages.length} record${usages.length === 1 ? '' : 's'}: ${names}. Remove the references or delete with inline_usages=true to copy it into the dependents.`
  )
}

// ---------- decorators (server-computed read fields) ----------
function catalogFor(store: MockStore, kind: string) {
  return store.catalog.databases.find((c) => c.kind === kind)
}

function decorateDatabase(store: MockStore, t: TenantData, db: Database): Database {
  const cat = catalogFor(store, db.kind)
  return {
    ...db,
    is_favorite: t.favorites.has(`database:${db.id}`),
    effective_configs: effectiveConfigs(cat, db),
    usages: usagesOfDatabase(t, db.id),
  }
}

function workloadRequirements(spec: WorkloadSpec): Requirements {
  const vus = spec.segments.reduce((m, s) => {
    const run = (s as { run?: { vus?: unknown } }).run
    return Math.max(m, num(run?.vus, 1))
  }, 1)
  const cpu = vus >= 128 ? 16 : vus >= 64 ? 8 : vus >= 32 ? 4 : 2
  const memory_gb = vus >= 128 ? 32 : vus >= 64 ? 16 : vus >= 32 ? 8 : 4
  return { runner: { cpu, memory_gb, reason: `${vus} VUs` } }
}

function decorateWorkload(t: TenantData, wl: Workload): Workload {
  return {
    ...wl,
    requirements: wl.requirements ?? workloadRequirements(wl),
    is_favorite: t.favorites.has(`workload:${wl.id}`),
    usages: usagesOfWorkload(t, wl.id),
  }
}

function resolveDatabase(
  store: MockStore,
  t: TenantData,
  ref: Test['database']
): Database | undefined {
  if (!ref) return undefined
  if ('ref' in ref) {
    const db = t.databases.find((d) => d.id === ref.ref.id)
    return db ? decorateDatabase(store, t, db) : undefined
  }
  const spec = ref.inline
  const derived = deriveDatabase(catalogFor(store, spec.kind), spec.kind, spec.params ?? {})
  return {
    id: 'inline',
    name: `${spec.kind} ${spec.version} (inline)`,
    author: author(store),
    created_at: iso(),
    updated_at: iso(),
    ...spec,
    ...derived,
  }
}

function resolveWorkload(store: MockStore, t: TenantData, ref: Test['workload']) {
  if (!ref) return undefined
  if ('ref' in ref) {
    const wl = t.workloads.find((w) => w.id === ref.ref.id)
    return wl ? decorateWorkload(t, wl) : undefined
  }
  const spec = ref.inline
  return {
    id: 'inline',
    name: `${spec.protocol} workload (inline)`,
    author: author(store),
    created_at: iso(),
    updated_at: iso(),
    ...spec,
    requirements: workloadRequirements(spec),
  } as Workload
}

function smallestFitting(role: string, req: Requirements[string]): Size | undefined {
  const table = role === 'etcd' ? SIZES.slice(0, 2) : role === 'proxy' ? SIZES.slice(0, 3) : SIZES
  return table.find((s) => s.cpu >= (req.cpu ?? 0) && s.memory_gb >= (req.memory_gb ?? 0))?.size
}

// Domain validation of a test: fits/suggested per role, protocol vs kind, provider, keep.
export function validateTest(
  store: MockStore,
  t: TenantData,
  spec: Schemas['TestSpec'],
  previous?: Fit
): TestValidation {
  const issues: Issue[] = []
  const db = resolveDatabase(store, t, spec.database)
  const wl = resolveWorkload(store, t, spec.workload)
  if (!spec.database)
    issues.push(err('database', 'REQUIRED', 'Pick a database definition or describe one inline'))
  else if (!db)
    issues.push(err('database', 'MISSING_REF', 'The referenced database no longer exists'))
  if (!spec.workload)
    issues.push(err('workload', 'REQUIRED', 'Pick a workload or describe one inline'))
  else if (!wl)
    issues.push(err('workload', 'MISSING_REF', 'The referenced workload no longer exists'))

  const requirements: Requirements = {}
  if (db && wl) {
    const cat = catalogFor(store, db.kind)
    const protocols = cat?.protocols ?? []
    if (protocols.length && !protocols.includes(wl.protocol))
      issues.push(
        err(
          'workload',
          'PROTOCOL_MISMATCH',
          `Workload protocol ${wl.protocol} does not match ${db.kind} (accepts ${protocols.join(', ')})`
        )
      )
    Object.assign(requirements, db.requirements ?? {}, wl.requirements ?? {})
    for (const [role, req] of Object.entries(requirements)) {
      const chosen = spec.sizes?.[role]
      const suggested = smallestFitting(role, req)
      if (!chosen) {
        issues.push({
          ...err(`sizes.${role}.size`, 'REQUIRED', `Pick a size for role "${role}"`),
          suggested,
        })
        continue
      }
      const s = SIZES.find((x) => x.size === chosen.size)
      if (!s) {
        issues.push(err(`sizes.${role}.size`, 'UNKNOWN_SIZE', `Unknown size ${chosen.size}`))
        continue
      }
      if (s.cpu < (req.cpu ?? 0) || s.memory_gb < (req.memory_gb ?? 0))
        issues.push({
          ...err(
            `sizes.${role}.size`,
            'SIZE_TOO_SMALL',
            `${role} needs ${req.cpu ?? 0} CPU / ${req.memory_gb ?? 0}GB${req.reason ? ` (${req.reason})` : ''}; ${chosen.size} has ${s.cpu} CPU / ${s.memory_gb}GB`
          ),
          suggested,
        })
      if (req.disk_gb && chosen.disk?.gb !== undefined && chosen.disk.gb < req.disk_gb)
        issues.push({
          ...err(
            `sizes.${role}.disk.gb`,
            'DISK_TOO_SMALL',
            `${role} data needs ≥ ${req.disk_gb}GB; ${chosen.disk.gb}GB configured`,
            'WARNING'
          ),
          suggested: req.disk_gb,
        })
    }
  }
  if (!spec.provider_profile_id)
    issues.push(
      err(
        'provider_profile_id',
        'PROVIDER_UNSET',
        'No default provider — you will pick one at launch',
        'WARNING'
      )
    )
  else {
    const p = t.providers.find((x) => x.id === spec.provider_profile_id)
    if (!p) issues.push(err('provider_profile_id', 'MISSING_REF', 'Provider profile not found'))
    else if (p.status !== 'ready')
      issues.push(
        err(
          'provider_profile_id',
          'PROVIDER_NOT_READY',
          `Provider "${p.name}" is ${p.status}${p.status_reason ? `: ${p.status_reason}` : ''}`
        )
      )
  }
  const keepSec = durationSec(spec.keep)
  if (Number.isNaN(keepSec))
    issues.push(err('keep', 'FORMAT', 'Use a Go duration such as 30m or 2h'))
  else if (keepSec > durationSec(t.limits.max_keep))
    issues.push(
      err('keep', 'KEEP_TOO_LONG', `Tenant limit for keeping a stand is ${t.limits.max_keep}`)
    )

  const fits = !issues.some((i) => i.severity === 'ERROR')
  const status: Schemas['TestStatus'] =
    !db || !wl
      ? 'draft'
      : fits && !previous?.stale?.database && !previous?.stale?.workload
        ? 'ready'
        : 'needs_attention'
  const machines: NonNullable<TestValidation['estimated']>['machines'] = []
  if (db) {
    for (const n of db.topology_preview?.nodes ?? []) {
      if (n.colocated_with) continue
      const roleKey = ['master', 'replica', 'storage', 'node', 'instance', 'managed'].includes(
        n.role
      )
        ? 'db'
        : ['haproxy', 'proxysql', 'maxscale', 'pgbouncer'].includes(n.role)
          ? 'proxy'
          : n.role
      const size = spec.sizes?.[roleKey]?.size
      const s = SIZES.find((x) => x.size === size)
      machines.push({
        role: n.role,
        count: n.count,
        size,
        cpu: s?.cpu,
        memory_gb: s?.memory_gb,
        disk_gb: spec.sizes?.[roleKey]?.disk?.gb ?? s?.default_disk_gb,
      })
    }
  }
  if (wl) {
    const s = SIZES.find((x) => x.size === spec.sizes?.runner?.size)
    machines.push({
      role: 'runner',
      count: 1,
      size: spec.sizes?.runner?.size,
      cpu: s?.cpu,
      memory_gb: s?.memory_gb,
      disk_gb: s?.default_disk_gb,
    })
  }
  return {
    status,
    validation: { fits, issues, stale: previous?.stale },
    requirements,
    resolved: { database: db, workload: wl },
    estimated: { machines },
  }
}

function decorateTest(store: MockStore, t: TenantData, test: Test): Test {
  const db = resolveDatabase(store, t, test.database)
  const wl = resolveWorkload(store, t, test.workload)
  const runs = t.runs
    .filter((r) => r.test_ref.id === test.id)
    .sort((a, b) => (b.created_at < a.created_at ? -1 : 1))
  const last = runs[0]
  return {
    ...test,
    is_favorite: t.favorites.has(`test:${test.id}`),
    resolved: { database: db, workload: wl },
    requirements: { ...(db?.requirements ?? {}), ...(wl?.requirements ?? {}) },
    summary: {
      db_kind: db?.kind,
      db_version: db?.version,
      topology_label: db?.topology_preview?.label,
      node_count: db?.topology_preview?.node_count,
      protocol: wl?.protocol,
      stroppy_version: wl?.stroppy_version,
      last_run: last
        ? { id: last.id, name: last.name, status: last.status, started_at: last.started_at }
        : undefined,
      run_count: runs.length,
    },
    usages: usagesOfTest(t, test.id),
  }
}

// Recompute validation + status of a saved test after it or a dependency changed.
function refreshTest(store: MockStore, t: TenantData, test: Test, promote = false): void {
  const v = validateTest(store, t, test, test.validation)
  test.validation = v.validation
  if (promote) test.status = v.status
  else if (test.status !== 'draft')
    test.status = v.status === 'draft' ? 'needs_attention' : v.status
}

function markStale(t: TenantData, field: 'database' | 'workload', id: string): void {
  for (const test of t.tests) {
    const ref = test[field]
    if (ref && 'ref' in ref && ref.ref.id === id) {
      test.validation = {
        fits: test.validation?.fits ?? true,
        issues: [
          ...(test.validation?.issues ?? []).filter((i) => i.code !== 'STALE_REF'),
          {
            path: field,
            code: 'STALE_REF',
            severity: 'WARNING',
            message: `${field === 'database' ? 'Database' : 'Workload'} changed after this test was validated — re-validate.`,
          },
        ],
        stale: { ...test.validation?.stale, [field]: true },
      }
      if (test.status === 'ready') test.status = 'needs_attention'
    }
  }
}

// ---------- diff / export ----------
function deepDiff(a: unknown, b: unknown, path = ''): Diff['changes'] {
  const changes: Diff['changes'] = []
  const isObj = (v: unknown) => v !== null && typeof v === 'object' && !Array.isArray(v)
  if (isObj(a) && isObj(b)) {
    const ao = a as Record<string, unknown>
    const bo = b as Record<string, unknown>
    for (const k of new Set([...Object.keys(ao), ...Object.keys(bo)])) {
      const p = path ? `${path}.${k}` : k
      if (!(k in ao)) changes.push({ path: p, op: 'add', b: bo[k] })
      else if (!(k in bo)) changes.push({ path: p, op: 'remove', a: ao[k] })
      else changes.push(...deepDiff(ao[k], bo[k], p))
    }
    return changes
  }
  if (JSON.stringify(a) !== JSON.stringify(b))
    changes.push({ path: path || '.', op: 'replace', a, b })
  return changes
}

const dbSpec = (d: DatabaseSpec): DatabaseSpec => ({
  kind: d.kind,
  version: d.version,
  image: d.image,
  params: d.params,
  configs: d.configs,
  runtime: d.runtime,
  external: d.external,
  schema: d.schema,
})
const wlSpec = (w: WorkloadSpec): WorkloadSpec => ({
  stroppy_version: w.stroppy_version,
  protocol: w.protocol,
  segments: w.segments,
  options: w.options,
  schema: w.schema,
})
const testSpec = (x: Schemas['TestSpec']): Schemas['TestSpec'] => ({
  database: x.database,
  workload: x.workload,
  sizes: x.sizes,
  provider_profile_id: x.provider_profile_id,
  keep: x.keep,
  rating: x.rating,
  execution: x.execution,
})

function exportDoc(
  kind: ExportDocument['kind'],
  e: Entity,
  spec: Record<string, unknown>
): ExportDocument {
  return {
    api_version: 'stroppy.io/v1',
    kind,
    metadata: { name: e.name, description: e.description, tags: e.tags },
    spec,
  }
}

function checkImport(
  body: unknown,
  kind: ExportDocument['kind']
): { problem: HandlerResult; doc?: never } | { doc: ExportDocument; problem?: never } {
  const doc = body as Partial<ExportDocument> | undefined
  if (!doc || typeof doc !== 'object')
    return {
      problem: validationProblem([err('', 'BAD_DOCUMENT', 'Expected a YAML/JSON document')]),
    }
  if (doc.kind !== kind)
    return {
      problem: validationProblem([
        err('kind', 'KIND_MISMATCH', `Document kind must be "${kind}", got "${doc.kind ?? ''}"`),
      ]),
    }
  if (!doc.spec || typeof doc.spec !== 'object')
    return { problem: validationProblem([err('spec', 'REQUIRED', '`spec` is required')]) }
  if (!doc.metadata?.name?.trim())
    return {
      problem: validationProblem([err('metadata.name', 'REQUIRED', '`metadata.name` is required')]),
    }
  return { doc: doc as ExportDocument }
}

function tenant(store: MockStore, slug: string) {
  return store.tenant(slug)
}

function entityBase(
  store: MockStore,
  name: string,
  description?: string,
  tags?: Record<string, string>
) {
  const now = iso()
  return {
    id: uuid(),
    name: name.trim(),
    description: description?.trim() || undefined,
    tags,
    author: author(store),
    created_at: now,
    updated_at: now,
  }
}

function uniqueName(existing: { name: string }[], base: string): string {
  let name = `${base} (copy)`
  let i = 2
  while (existing.some((e) => e.name === name)) name = `${base} (copy ${i++})`
  return name
}

// =====================================================================
// Databases
// =====================================================================
function buildDatabase(store: MockStore, body: Schemas['DatabaseWrite']) {
  const errors: ValidationError[] = []
  if (!body?.name?.trim()) errors.push(err('name', 'REQUIRED', 'Give the database a name'))
  const cat = catalogFor(store, body?.kind)
  if (!body?.kind) errors.push(err('kind', 'REQUIRED', 'Pick a database kind'))
  else if (!cat) errors.push(err('kind', 'UNKNOWN_KIND', `Unknown kind ${body.kind}`))
  const params = { ...(body?.params ?? {}) }
  const version =
    body?.version || str(params.version) || cat?.versions.find((v) => v.default)?.version
  if (!version) errors.push(err('version', 'REQUIRED', 'Pick a version'))
  else if (cat && !cat.versions.some((v) => v.version === version) && cat.deployable)
    errors.push(
      err('version', 'UNKNOWN_VERSION', `Version ${version} is not in the catalog`, 'WARNING')
    )
  if (cat && version && params.version === undefined && cat.kind !== 'orioledb')
    params.version = version
  if (cat) errors.push(...validateParams(cat.params_schema, params))
  return { errors, cat, params, version: version ?? '' }
}

route('GET', '/api/v1/t/:slug/databases', ({ store, params, query }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const lq = parseListQuery(query, 'updated_at')
  const kinds = multi(query, 'kind')
  let items = filterEntities(
    t.databases,
    query,
    t.favorites,
    'database',
    (d) => `${d.kind} ${d.version}`
  )
  if (kinds.length) items = items.filter((d) => kinds.includes(d.kind))
  items = sortEntities(items, lq.sort, lq.order, (d, k) =>
    k === 'kind' ? `${d.kind} ${d.version}` : undefined
  )
  return {
    json: paginate(
      items.map((d) => decorateDatabase(store, t, d)),
      lq
    ),
  }
})

route('POST', '/api/v1/t/:slug/databases', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const b = (body ?? {}) as Schemas['DatabaseWrite']
  const { errors, cat, params: p, version } = buildDatabase(store, b)
  if (errors.some((e) => e.severity === 'ERROR')) return validationProblem(errors)
  const derived = deriveDatabase(cat, b.kind, p)
  const db: Database = {
    ...entityBase(store, b.name, b.description, b.tags),
    kind: b.kind,
    version,
    image: b.image,
    params: p,
    configs: b.configs ?? {},
    runtime: b.runtime,
    external: b.external,
    schema: { id: cat?.params_schema ?? `db.${b.kind}.params`, version: '1' },
    ...derived,
    validation: { errors: errors.filter((e) => e.severity === 'WARNING') },
  }
  t.databases.unshift(db)
  store.audit(params.slug, 'database.create', { kind: 'database', id: db.id, name: db.name })
  return { status: 201, json: decorateDatabase(store, t, db) }
})

route('POST', '/api/v1/t/:slug/databases:preview', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const b = (body ?? {}) as Schemas['DatabaseWrite']
  const {
    errors,
    cat,
    params: p,
    version,
  } = buildDatabase(store, { ...b, name: b.name || 'preview' })
  if (!cat) return validationProblem(errors)
  const derived = deriveDatabase(cat, b.kind, p)
  return {
    json: {
      validation: { errors },
      ...derived,
      effective_configs: effectiveConfigs(cat, { ...b, version, params: p }),
    } satisfies Schemas['DatabasePreview'],
  }
})

route('GET', '/api/v1/t/:slug/databases:diff', ({ store, params, query }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const a = t.databases.find((d) => d.id === query.get('a'))
  const b = t.databases.find((d) => d.id === query.get('b'))
  if (!a || !b) return notFound('database')
  return { json: { changes: deepDiff(dbSpec(a), dbSpec(b)) } }
})

route('POST', '/api/v1/t/:slug/databases:import', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const r = checkImport(body, 'Database')
  if (r.problem) return r.problem
  const spec = r.doc.spec as DatabaseSpec
  const write: Schemas['DatabaseWrite'] = {
    ...spec,
    ...r.doc.metadata,
    name: r.doc.metadata?.name ?? '',
  }
  const { errors, cat, params: p, version } = buildDatabase(store, write)
  if (errors.some((e) => e.severity === 'ERROR')) return validationProblem(errors)
  const db: Database = {
    ...entityBase(store, write.name, write.description, write.tags),
    ...dbSpec({ ...spec, version, params: p }),
    schema: { id: cat?.params_schema ?? `db.${spec.kind}.params`, version: '1' },
    ...deriveDatabase(cat, spec.kind, p),
  }
  t.databases.unshift(db)
  store.audit(params.slug, 'database.import', { kind: 'database', id: db.id, name: db.name })
  return { json: decorateDatabase(store, t, db) }
})

route('GET', '/api/v1/t/:slug/databases/:id', ({ store, params }) => {
  const t = tenant(store, params.slug)
  const db = t?.databases.find((d) => d.id === params.id)
  if (!t || !db) return notFound('database')
  return { json: decorateDatabase(store, t, db) }
})

route('PATCH', '/api/v1/t/:slug/databases/:id', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  const db = t?.databases.find((d) => d.id === params.id)
  if (!t || !db) return notFound('database')
  const b = (body ?? {}) as Schemas['DatabasePatch']
  if (b.name !== undefined && !b.name.trim())
    return validationProblem([err('name', 'REQUIRED', 'Name cannot be empty')])
  const next: Database = {
    ...db,
    name: b.name?.trim() ?? db.name,
    description: b.description !== undefined ? b.description.trim() || undefined : db.description,
    tags: b.tags ?? db.tags,
    version: b.version ?? db.version,
    image: b.image ?? db.image,
    params: b.params ?? db.params,
    configs: b.configs ?? db.configs,
    runtime: b.runtime ?? db.runtime,
  }
  const specChanged =
    b.params !== undefined ||
    b.configs !== undefined ||
    b.version !== undefined ||
    b.image !== undefined
  if (specChanged) {
    const cat = catalogFor(store, next.kind)
    const errors = cat ? validateParams(cat.params_schema, next.params) : []
    if (errors.some((e) => e.severity === 'ERROR')) return validationProblem(errors)
    if (b.params && str(b.params.version) && b.version === undefined)
      next.version = str(b.params.version) ?? next.version
    Object.assign(next, deriveDatabase(cat, next.kind, next.params))
    next.validation = { errors }
    markStale(t, 'database', db.id)
  }
  next.updated_at = iso()
  t.databases = t.databases.map((d) => (d.id === db.id ? next : d))
  store.audit(params.slug, 'database.update', { kind: 'database', id: db.id, name: next.name })
  return { json: decorateDatabase(store, t, next) }
})

route('DELETE', '/api/v1/t/:slug/databases/:id', ({ store, params, query }) => {
  const t = tenant(store, params.slug)
  const db = t?.databases.find((d) => d.id === params.id)
  if (!t || !db) return notFound('database')
  const usages = usagesOfDatabase(t, db.id)
  if (usages.length) {
    if (query.get('inline_usages') !== 'true') return inUse(usages, 'database')
    for (const test of t.tests)
      if (test.database && 'ref' in test.database && test.database.ref.id === db.id)
        test.database = { inline: dbSpec(db) }
  }
  t.databases = t.databases.filter((d) => d.id !== db.id)
  t.favorites.delete(`database:${db.id}`)
  store.audit(params.slug, 'database.delete', { kind: 'database', id: db.id, name: db.name })
  return noContent()
})

route('POST', '/api/v1/t/:slug/databases/:id:clone', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  const db = t?.databases.find((d) => d.id === params.id)
  if (!t || !db) return notFound('database')
  const name =
    (body as { name?: string } | undefined)?.name?.trim() || uniqueName(t.databases, db.name)
  const copy: Database = { ...db, ...entityBase(store, name, db.description, db.tags) }
  t.databases.unshift(copy)
  store.audit(params.slug, 'database.clone', { kind: 'database', id: copy.id, name: copy.name })
  return { status: 201, json: decorateDatabase(store, t, copy) }
})

route('GET', '/api/v1/t/:slug/databases/:id:export', ({ store, params }) => {
  const t = tenant(store, params.slug)
  const db = t?.databases.find((d) => d.id === params.id)
  if (!t || !db) return notFound('database')
  return { json: exportDoc('Database', db, dbSpec(db) as Record<string, unknown>) }
})

// =====================================================================
// Workloads
// =====================================================================
function validateWorkloadWrite(
  store: MockStore,
  b: Partial<Schemas['WorkloadWrite']>
): ValidationError[] {
  const errors: ValidationError[] = []
  if (!b.name?.trim()) errors.push(err('name', 'REQUIRED', 'Give the workload a name'))
  const versions = store.catalog.stroppy.versions
  if (!b.stroppy_version) errors.push(err('stroppy_version', 'REQUIRED', 'Pick a stroppy version'))
  else if (!versions.some((v) => v.version === b.stroppy_version))
    errors.push(
      err(
        'stroppy_version',
        'UNKNOWN_VERSION',
        `stroppy ${b.stroppy_version} is not in the catalog`,
        'WARNING'
      )
    )
  if (!b.protocol) errors.push(err('protocol', 'REQUIRED', 'Pick a protocol'))
  if (!b.segments?.length) errors.push(err('segments', 'REQUIRED', 'Add at least one segment'))
  b.segments?.forEach((s, i) => {
    const seg = s as { name?: unknown; workload?: { script?: unknown }; run?: { vus?: unknown } }
    if (!seg.name) errors.push(err(`segments[${i}].name`, 'REQUIRED', 'Segment name is required'))
    if (!seg.workload?.script)
      errors.push(err(`segments[${i}].workload.script`, 'REQUIRED', 'Pick a script'))
    else if (b.stroppy_version) {
      const ver = versions.find((v) => v.version === b.stroppy_version)
      const script = ver?.scripts?.find((x) => x.id === seg.workload?.script)
      if (script && b.protocol && script.protocols && !script.protocols.includes(b.protocol))
        errors.push(
          err(
            `segments[${i}].workload.script`,
            'PROTOCOL_MISMATCH',
            `${script.id} does not support protocol ${b.protocol}`,
            'WARNING'
          )
        )
    }
    if (seg.run?.vus !== undefined && num(seg.run.vus, 1) < 1)
      errors.push(err(`segments[${i}].run.vus`, 'GTE_VIOLATED', 'VUs must be ≥ 1'))
  })
  return errors
}

const segmentSummary = (segments: Record<string, unknown>[]) =>
  segments.map((s) => {
    const w = s.workload as { script?: string } | undefined
    const r = s.run as { vus?: number; duration?: string; iterations?: number } | undefined
    return {
      name: str(s.name),
      script: w?.script,
      vus: r?.vus,
      limit: r?.duration ?? (r?.iterations !== undefined ? `${r.iterations} it` : undefined),
      steps: Array.isArray(s.steps) ? (s.steps as string[]) : undefined,
    }
  })

route('GET', '/api/v1/t/:slug/workloads', ({ store, params, query }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const lq = parseListQuery(query, 'updated_at')
  const protocols = multi(query, 'protocol')
  const versions = multi(query, 'stroppy_version')
  const script = query.get('script')
  let items = filterEntities(
    t.workloads,
    query,
    t.favorites,
    'workload',
    (w) =>
      `${w.protocol} ${w.stroppy_version} ${segmentSummary(w.segments)
        .map((s) => s.script)
        .join(' ')}`
  )
  if (protocols.length) items = items.filter((w) => protocols.includes(w.protocol))
  if (versions.length) items = items.filter((w) => versions.includes(w.stroppy_version))
  if (script)
    items = items.filter((w) => segmentSummary(w.segments).some((s) => s.script === script))
  items = sortEntities(items, lq.sort, lq.order, (w, k) =>
    k === 'protocol' ? w.protocol : k === 'stroppy_version' ? w.stroppy_version : undefined
  )
  return {
    json: paginate(
      items.map((w) => decorateWorkload(t, w)),
      lq
    ),
  }
})

route('POST', '/api/v1/t/:slug/workloads', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const b = (body ?? {}) as Schemas['WorkloadWrite']
  const errors = validateWorkloadWrite(store, b)
  if (errors.some((e) => e.severity === 'ERROR')) return validationProblem(errors)
  const wl: Workload = {
    ...entityBase(store, b.name, b.description, b.tags),
    ...wlSpec(b),
    options: b.options ?? { connection: { kind: b.protocol === 'ydb_grpcs' ? 'ydb' : b.protocol } },
    schema: { id: 'workload.stroppy', version: '1' },
    requirements: workloadRequirements(b),
    validation: { errors: errors.filter((e) => e.severity === 'WARNING') },
  }
  t.workloads.unshift(wl)
  store.audit(params.slug, 'workload.create', { kind: 'workload', id: wl.id, name: wl.name })
  return { status: 201, json: decorateWorkload(t, wl) }
})

route('POST', '/api/v1/t/:slug/workloads:preview', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const b = (body ?? {}) as Schemas['WorkloadWrite']
  const errors = validateWorkloadWrite(store, { ...b, name: b.name || 'preview' })
  return {
    json: {
      validation: { errors },
      requirements: workloadRequirements({ ...b, segments: b.segments ?? [] }),
      segments_summary: segmentSummary(b.segments ?? []),
    } satisfies Schemas['WorkloadPreview'],
  }
})

route('GET', '/api/v1/t/:slug/workloads:diff', ({ store, params, query }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const a = t.workloads.find((d) => d.id === query.get('a'))
  const b = t.workloads.find((d) => d.id === query.get('b'))
  if (!a || !b) return notFound('workload')
  return { json: { changes: deepDiff(wlSpec(a), wlSpec(b)) } }
})

route('POST', '/api/v1/t/:slug/workloads:import', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const r = checkImport(body, 'Workload')
  if (r.problem) return r.problem
  const spec = r.doc.spec as WorkloadSpec
  const write: Schemas['WorkloadWrite'] = {
    ...spec,
    ...r.doc.metadata,
    name: r.doc.metadata?.name ?? '',
  }
  const errors = validateWorkloadWrite(store, write)
  if (errors.some((e) => e.severity === 'ERROR')) return validationProblem(errors)
  const wl: Workload = {
    ...entityBase(store, write.name, write.description, write.tags),
    ...wlSpec(spec),
    schema: { id: 'workload.stroppy', version: '1' },
    requirements: workloadRequirements(spec),
  }
  t.workloads.unshift(wl)
  store.audit(params.slug, 'workload.import', { kind: 'workload', id: wl.id, name: wl.name })
  return { json: decorateWorkload(t, wl) }
})

route('GET', '/api/v1/t/:slug/workloads/:id', ({ store, params }) => {
  const t = tenant(store, params.slug)
  const wl = t?.workloads.find((d) => d.id === params.id)
  if (!t || !wl) return notFound('workload')
  return { json: decorateWorkload(t, wl) }
})

route('PATCH', '/api/v1/t/:slug/workloads/:id', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  const wl = t?.workloads.find((d) => d.id === params.id)
  if (!t || !wl) return notFound('workload')
  const b = (body ?? {}) as Schemas['WorkloadPatch']
  const next: Workload = {
    ...wl,
    name: b.name?.trim() ?? wl.name,
    description: b.description !== undefined ? b.description.trim() || undefined : wl.description,
    tags: b.tags ?? wl.tags,
    stroppy_version: b.stroppy_version ?? wl.stroppy_version,
    protocol: b.protocol ?? wl.protocol,
    segments: b.segments ?? wl.segments,
    options: b.options ?? wl.options,
  }
  const errors = validateWorkloadWrite(store, next)
  if (errors.some((e) => e.severity === 'ERROR')) return validationProblem(errors)
  const specChanged =
    b.segments !== undefined ||
    b.options !== undefined ||
    b.protocol !== undefined ||
    b.stroppy_version !== undefined
  if (specChanged) {
    next.requirements = workloadRequirements(next)
    next.validation = { errors }
    markStale(t, 'workload', wl.id)
  }
  next.updated_at = iso()
  t.workloads = t.workloads.map((w) => (w.id === wl.id ? next : w))
  store.audit(params.slug, 'workload.update', { kind: 'workload', id: wl.id, name: next.name })
  return { json: decorateWorkload(t, next) }
})

route('DELETE', '/api/v1/t/:slug/workloads/:id', ({ store, params, query }) => {
  const t = tenant(store, params.slug)
  const wl = t?.workloads.find((d) => d.id === params.id)
  if (!t || !wl) return notFound('workload')
  const usages = usagesOfWorkload(t, wl.id)
  if (usages.length) {
    if (query.get('inline_usages') !== 'true') return inUse(usages, 'workload')
    for (const test of t.tests)
      if (test.workload && 'ref' in test.workload && test.workload.ref.id === wl.id)
        test.workload = { inline: wlSpec(wl) }
  }
  t.workloads = t.workloads.filter((w) => w.id !== wl.id)
  t.favorites.delete(`workload:${wl.id}`)
  store.audit(params.slug, 'workload.delete', { kind: 'workload', id: wl.id, name: wl.name })
  return noContent()
})

route('POST', '/api/v1/t/:slug/workloads/:id:clone', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  const wl = t?.workloads.find((d) => d.id === params.id)
  if (!t || !wl) return notFound('workload')
  const name =
    (body as { name?: string } | undefined)?.name?.trim() || uniqueName(t.workloads, wl.name)
  const copy: Workload = { ...wl, ...entityBase(store, name, wl.description, wl.tags) }
  t.workloads.unshift(copy)
  store.audit(params.slug, 'workload.clone', { kind: 'workload', id: copy.id, name: copy.name })
  return { status: 201, json: decorateWorkload(t, copy) }
})

route('GET', '/api/v1/t/:slug/workloads/:id:export', ({ store, params }) => {
  const t = tenant(store, params.slug)
  const wl = t?.workloads.find((d) => d.id === params.id)
  if (!t || !wl) return notFound('workload')
  return { json: exportDoc('Workload', wl, wlSpec(wl) as Record<string, unknown>) }
})

// =====================================================================
// Tests
// =====================================================================
route('GET', '/api/v1/t/:slug/tests', ({ store, params, query }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const lq = parseListQuery(query, 'updated_at')
  const statuses = multi(query, 'status')
  const kinds = multi(query, 'kind')
  const decorated = t.tests.map((x) => decorateTest(store, t, x))
  let items = filterEntities(
    decorated,
    query,
    t.favorites,
    'test',
    (x) =>
      `${x.summary?.db_kind ?? ''} ${x.resolved?.database?.name ?? ''} ${x.resolved?.workload?.name ?? ''}`
  )
  if (statuses.length) items = items.filter((x) => statuses.includes(x.status))
  if (kinds.length) items = items.filter((x) => kinds.includes(x.summary?.db_kind ?? ''))
  items = sortEntities(items, lq.sort, lq.order, (x, k) =>
    k === 'kind'
      ? `${x.summary?.db_kind ?? ''} ${x.summary?.db_version ?? ''}`
      : k === 'last_run_at'
        ? (x.summary?.last_run?.started_at ?? '')
        : k === 'status'
          ? x.status
          : undefined
  )
  return { json: paginate(items, lq) }
})

route('POST', '/api/v1/t/:slug/tests', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const b = (body ?? {}) as Schemas['TestWrite']
  if (!b.name?.trim()) return validationProblem([err('name', 'REQUIRED', 'Give the test a name')])
  const spec = testSpec(b)
  const v = validateTest(store, t, spec)
  const test: Test = {
    ...entityBase(store, b.name, b.description, b.tags),
    ...spec,
    keep: spec.keep ?? t.settings.default_keep ?? '0s',
    rating: spec.rating ?? t.settings.default_rating ?? { tenant: true, global: false },
    status: 'draft',
    validation: v.validation,
  }
  t.tests.unshift(test)
  store.audit(params.slug, 'test.create', { kind: 'test', id: test.id, name: test.name })
  return { status: 201, json: decorateTest(store, t, test) }
})

route('POST', '/api/v1/t/:slug/tests:validate', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const b = (body ?? {}) as Schemas['TestWrite']
  return { json: validateTest(store, t, testSpec(b)) }
})

route('GET', '/api/v1/t/:slug/tests:diff', ({ store, params, query }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const a = t.tests.find((d) => d.id === query.get('a'))
  const b = t.tests.find((d) => d.id === query.get('b'))
  if (!a || !b) return notFound('test')
  return { json: { changes: deepDiff(testSpec(a), testSpec(b)) } }
})

route('POST', '/api/v1/t/:slug/tests:import', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  if (!t) return notFound('tenant')
  const r = checkImport(body, 'Test')
  if (r.problem) return r.problem
  const spec = testSpec(r.doc.spec as Schemas['TestSpec'])
  const v = validateTest(store, t, spec)
  const test: Test = {
    ...entityBase(
      store,
      r.doc.metadata?.name ?? '',
      r.doc.metadata?.description,
      r.doc.metadata?.tags
    ),
    ...spec,
    status: v.status,
    validation: v.validation,
  }
  t.tests.unshift(test)
  store.audit(params.slug, 'test.import', { kind: 'test', id: test.id, name: test.name })
  return { json: decorateTest(store, t, test) }
})

route('GET', '/api/v1/t/:slug/tests/:id', ({ store, params }) => {
  const t = tenant(store, params.slug)
  const test = t?.tests.find((d) => d.id === params.id)
  if (!t || !test) return notFound('test')
  return { json: decorateTest(store, t, test) }
})

route('PATCH', '/api/v1/t/:slug/tests/:id', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  const test = t?.tests.find((d) => d.id === params.id)
  if (!t || !test) return notFound('test')
  const b = (body ?? {}) as Schemas['TestPatch'] & { finalize?: boolean; revalidate?: boolean }
  if (b.name !== undefined && !b.name.trim())
    return validationProblem([err('name', 'REQUIRED', 'Name cannot be empty')])
  const next: Test = {
    ...test,
    name: b.name?.trim() ?? test.name,
    description: b.description !== undefined ? b.description.trim() || undefined : test.description,
    tags: b.tags ?? test.tags,
    database: b.database ?? test.database,
    workload: b.workload ?? test.workload,
    sizes: b.sizes ?? test.sizes,
    provider_profile_id:
      b.provider_profile_id !== undefined ? b.provider_profile_id : test.provider_profile_id,
    keep: b.keep ?? test.keep,
    rating: b.rating ?? test.rating,
    execution: b.execution ?? test.execution,
    updated_at: iso(),
  }
  // Changing a reference clears its stale flag.
  if (b.database !== undefined || b.workload !== undefined || b.revalidate) {
    next.validation = {
      fits: next.validation?.fits ?? true,
      issues: (next.validation?.issues ?? []).filter((i) => i.code !== 'STALE_REF'),
      stale: {
        database:
          b.database !== undefined || b.revalidate ? false : next.validation?.stale?.database,
        workload:
          b.workload !== undefined || b.revalidate ? false : next.validation?.stale?.workload,
      },
    }
  }
  if (b.finalize) {
    const v = validateTest(store, t, next, next.validation)
    if (!v.validation.fits)
      return problem(400, 'test_not_ready', 'The test does not fit yet — fix the issues first.', {
        validation: {
          errors: (v.validation.issues ?? []).map((i) => ({
            path: i.path,
            code: i.code,
            severity: i.severity,
            message: i.message,
          })),
        },
      })
    next.validation = v.validation
    next.status = 'ready'
  } else refreshTest(store, t, next, !!b.revalidate)
  t.tests = t.tests.map((x) => (x.id === test.id ? next : x))
  store.audit(params.slug, 'test.update', { kind: 'test', id: test.id, name: next.name })
  return { json: decorateTest(store, t, next) }
})

route('DELETE', '/api/v1/t/:slug/tests/:id', ({ store, params }) => {
  const t = tenant(store, params.slug)
  const test = t?.tests.find((d) => d.id === params.id)
  if (!t || !test) return notFound('test')
  const usages = usagesOfTest(t, test.id)
  if (usages.length) return inUse(usages, 'test')
  t.tests = t.tests.filter((x) => x.id !== test.id)
  t.favorites.delete(`test:${test.id}`)
  store.audit(params.slug, 'test.delete', { kind: 'test', id: test.id, name: test.name })
  return noContent()
})

route('POST', '/api/v1/t/:slug/tests/:id:clone', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  const test = t?.tests.find((d) => d.id === params.id)
  if (!t || !test) return notFound('test')
  const name =
    (body as { name?: string } | undefined)?.name?.trim() || uniqueName(t.tests, test.name)
  const copy: Test = { ...test, ...entityBase(store, name, test.description, test.tags) }
  t.tests.unshift(copy)
  store.audit(params.slug, 'test.clone', { kind: 'test', id: copy.id, name: copy.name })
  return { status: 201, json: decorateTest(store, t, copy) }
})

route('GET', '/api/v1/t/:slug/tests/:id:export', ({ store, params, query }) => {
  const t = tenant(store, params.slug)
  const test = t?.tests.find((d) => d.id === params.id)
  if (!t || !test) return notFound('test')
  const spec = testSpec(test)
  if (query.get('inline_refs') === 'true') {
    const db = resolveDatabase(store, t, test.database)
    const wl = resolveWorkload(store, t, test.workload)
    if (db) spec.database = { inline: dbSpec(db) }
    if (wl) spec.workload = { inline: wlSpec(wl) }
  }
  return { json: exportDoc('Test', test, spec as Record<string, unknown>) }
})

route('GET', '/api/v1/t/:slug/tests/:id/runs', ({ store, params, query }) => {
  const t = tenant(store, params.slug)
  const test = t?.tests.find((d) => d.id === params.id)
  if (!t || !test) return notFound('test')
  const lq = parseListQuery(query, 'default')
  const runs = sortRuns(
    t.runs.filter((r) => r.test_ref.id === test.id),
    lq.sort,
    lq.order,
    (r) => t.favorites.has(`run:${r.id}`)
  ).map((r) => ({ ...r, is_favorite: t.favorites.has(`run:${r.id}`) }))
  const completed = sortBy(
    runs.filter((r) => r.status === 'completed' && r.summary?.headline?.tps !== undefined),
    (r) => r.started_at ?? r.created_at,
    'asc'
  )
  const page = paginate(runs, lq)
  return {
    json: {
      ...page,
      trend: {
        metric: 'tps',
        points: completed.map((r) => ({
          run_id: r.id,
          at: r.started_at ?? r.created_at,
          value: r.summary?.headline?.tps ?? 0,
        })),
        baseline_run_id: completed[0]?.id ?? null,
      },
    } satisfies Schemas['TestRunHistory'],
  }
})

route('POST', '/api/v1/t/:slug/tests/:id:launch', ({ store, params, body }) => {
  const t = tenant(store, params.slug)
  const test = t?.tests.find((d) => d.id === params.id)
  if (!t || !test) return notFound('test')
  const overrides = (body ?? {}) as LaunchOverrides
  const effective: Schemas['TestSpec'] = {
    ...test,
    sizes: overrides.sizes ?? test.sizes,
    provider_profile_id: overrides.provider_profile_id ?? test.provider_profile_id,
    keep: overrides.keep ?? test.keep,
  }
  const v = validateTest(store, t, effective)
  const errors = (v.validation.issues ?? []).filter(
    (i) => i.severity === 'ERROR' || i.code === 'PROVIDER_UNSET'
  )
  if (errors.length)
    return problem(
      409,
      'test_not_ready',
      'Fix the issues (or override them at launch) before launching.',
      {
        validation: {
          errors: errors.map((i) => ({
            path: i.path,
            code: i.code,
            severity: 'ERROR',
            message: i.message,
          })),
        },
      }
    )
  const running = t.runs.filter((r) => r.status === 'running' || r.status === 'pending').length
  if (running >= t.limits.max_concurrent_runs)
    return problem(
      429,
      'quota_exceeded',
      `Tenant limit of ${t.limits.max_concurrent_runs} concurrent runs reached.`
    )
  const run: Run = launchFromTest(
    store,
    params.slug,
    t,
    { ...test, sizes: effective.sizes, provider_profile_id: effective.provider_profile_id },
    overrides,
    'manual'
  )
  return { status: 201, json: run }
})

// =====================================================================
// Providers (read-only) — only when the settings area has not registered them.
// =====================================================================
if (!listRoutes().includes('GET /api/v1/t/:slug/providers')) {
  route('GET', '/api/v1/t/:slug/providers', ({ store, params }) => {
    const t = tenant(store, params.slug)
    if (!t) return notFound('tenant')
    return { json: { data: t.providers } }
  })
  route('GET', '/api/v1/t/:slug/providers/:id', ({ store, params }) => {
    const t = tenant(store, params.slug)
    const p = t?.providers.find((x) => x.id === params.id)
    return p ? { json: p } : notFound('provider')
  })
}
