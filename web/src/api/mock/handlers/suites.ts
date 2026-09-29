import type {
  LaunchOverrides,
  Run,
  RunStatus,
  Schedule,
  Schemas,
  SuiteRun,
  SuiteRunSummary,
} from '@api/types'
import { nextFireTime } from '@helpers/cron'
import { listRoutes, noContent, notFound, problem, route } from '../router'
import { launchFromTest } from '../run-factory'
import { getSimulation } from '../simulation'
import type { MockStore, Suite, TenantData, Test } from '../store'
import {
  durationStr,
  iso,
  matchesSearch,
  multi,
  paginate,
  parseListQuery,
  sortBy,
  uuid,
} from '../util'
import { createShare } from './runs'

type SuiteCell = Schemas['SuiteCell']
type SuiteWrite = Schemas['SuiteWrite']
type SuitePreview = Schemas['SuitePreview']
type RoleSizes = Schemas['RoleSizes']

const TERMINAL: RunStatus[] = ['completed', 'failed', 'cancelled']
const SIZE_SPECS: Record<string, { cpu: number; memory_gb: number; disk_gb: number }> = {
  XS: { cpu: 2, memory_gb: 4, disk_gb: 40 },
  S: { cpu: 4, memory_gb: 16, disk_gb: 80 },
  M: { cpu: 8, memory_gb: 32, disk_gb: 160 },
  L: { cpu: 16, memory_gb: 64, disk_gb: 320 },
  XL: { cpu: 32, memory_gb: 128, disk_gb: 640 },
}

// ---------- helpers ----------
function tenantOf(store: MockStore, slug: string): TenantData | undefined {
  return store.tenant(slug)
}

function testOf(t: TenantData, id: string | undefined): Test | undefined {
  return id ? t.tests.find((x) => x.id === id) : undefined
}

function testRef(t: TenantData, ref: Schemas['Ref']): Schemas['Ref'] {
  return { id: ref.id, name: testOf(t, ref.id)?.name ?? ref.name }
}

function shortLabel(test: Test | undefined): string {
  if (!test) return 'test'
  const kind = test.summary?.db_kind
  return kind
    ? `${kind}${test.summary?.db_version ? ` ${test.summary.db_version}` : ''}`
    : test.name
}

// Machines of a test (from the database topology preview + one runner).
function machinesOf(t: TenantData, test: Test | undefined, sizes: RoleSizes | undefined) {
  const dbId = test?.database && 'ref' in test.database ? test.database.ref.id : undefined
  const db = t.databases.find((d) => d.id === dbId)
  const s = sizes ?? test?.sizes ?? {}
  let machines = 0
  let cpu = 0
  let memory_gb = 0
  let disk_gb = 0
  const add = (size: string | undefined, disk?: number) => {
    const spec = SIZE_SPECS[size ?? 'M'] ?? SIZE_SPECS.M
    machines += 1
    cpu += spec.cpu
    memory_gb += spec.memory_gb
    disk_gb += disk ?? spec.disk_gb
  }
  const nodes = db?.topology_preview?.nodes ?? [{ role: 'db', count: 1 }]
  for (const n of nodes) {
    if (n.colocated_with) continue
    const roleKey =
      n.role === 'master' ||
      n.role === 'replica' ||
      n.role === 'storage' ||
      n.role === 'node' ||
      n.role === 'instance'
        ? 'db'
        : n.role === 'haproxy'
          ? 'proxy'
          : n.role
    const rs = s[roleKey] ?? s.db
    for (let i = 0; i < (n.count ?? 1); i++) add(rs?.size, rs?.disk?.gb)
  }
  add(s.runner?.size ?? 'S')
  return { machines, cpu, memory_gb, disk_gb }
}

// Deterministic cell id from test + axis values.
function cellId(testId: string, axis: NonNullable<SuiteCell['axis']>): string {
  const parts = [
    testId,
    axis.provider_profile_id ?? '',
    axis.sizes
      ? Object.entries(axis.sizes)
          .map(([r, v]) => `${r}${v.size}`)
          .join('')
      : '',
    axis.database_version ?? '',
    axis.workload_variant ?? '',
  ]
  return `c-${parts
    .join('-')
    .replace(/[^a-zA-Z0-9-]/g, '')
    .replace(/-+/g, '-')
    .replace(/-$/, '')}`
}

// tests × axes product. Existing cells (by id) keep `enabled`/`overrides`; manual cells survive.
export function computeCells(t: TenantData, spec: Schemas['SuiteSpec']): SuiteCell[] {
  const axes = spec.axes ?? {}
  const providers = axes.provider_profiles?.length ? axes.provider_profiles : [undefined]
  const sizes = axes.sizes?.length ? axes.sizes : [undefined]
  const versions = axes.database_versions?.length ? axes.database_versions : [undefined]
  const variants = axes.workload_variants?.length ? axes.workload_variants : [undefined]
  // previous cells are matched by id or by their axis signature (seeded ids are hand-written)
  const prev = new Map<string, SuiteCell>()
  for (const c of spec.cells ?? []) {
    prev.set(c.id, c)
    prev.set(cellId(c.test.id, c.axis ?? {}), c)
  }
  const out: SuiteCell[] = []
  const tests = (spec.tests ?? []).flatMap((x) => ('ref' in x ? [x.ref] : []))
  const manyProviders = providers.length > 1
  const manySizes = sizes.length > 1
  const manyVersions = versions.length > 1
  const manyVariants = variants.length > 1
  const manyTests = tests.length > 1
  for (const ref of tests) {
    const test = testOf(t, ref.id)
    for (const provider of providers)
      for (const sz of sizes)
        for (const ver of versions)
          for (const variant of variants) {
            const axis: NonNullable<SuiteCell['axis']> = {}
            if (provider) axis.provider_profile_id = provider
            if (sz) axis.sizes = sz
            if (ver) axis.database_version = ver
            if (variant?.name) axis.workload_variant = variant.name
            const sig = cellId(ref.id, axis)
            const id = prev.get(sig)?.id ?? sig
            const nameBits: string[] = [
              manyTests || (!manyProviders && !manySizes && !manyVersions && !manyVariants)
                ? (test?.name ?? ref.name ?? ref.id)
                : shortLabel(test),
            ]
            if (manySizes && sz) nameBits.push(sz.db?.size ?? Object.values(sz)[0]?.size ?? '')
            if (manyVersions && ver) nameBits.push(ver)
            if (manyVariants && variant?.name) nameBits.push(variant.name)
            if (manyProviders && provider)
              nameBits.push(t.providers.find((p) => p.id === provider)?.name ?? provider)
            const old = prev.get(sig)
            out.push({
              id,
              name: old?.name ?? nameBits.filter(Boolean).join(' / '),
              test: testRef(t, ref),
              enabled: old?.enabled ?? true,
              axis,
              overrides: old?.overrides,
              generated: true,
            })
          }
  }
  // manually added cells (not generated) are kept as-is
  for (const c of spec.cells ?? [])
    if (c.generated === false && !out.find((x) => x.id === c.id)) out.push(c)
  return out
}

function validateCell(t: TenantData, cell: SuiteCell): Schemas['Fit'] {
  const issues: NonNullable<Schemas['Fit']['issues']> = []
  const test = testOf(t, cell.test.id)
  if (!test) {
    issues.push({ path: 'test', code: 'NOT_FOUND', severity: 'ERROR', message: 'Test not found' })
    return { fits: false, issues }
  }
  for (const i of test.validation?.issues ?? [])
    if (i.severity === 'ERROR') issues.push({ ...i, path: `test.${i.path}` })
  const providerId =
    cell.overrides?.provider_profile_id ??
    cell.axis?.provider_profile_id ??
    test.provider_profile_id
  const provider = t.providers.find((p) => p.id === providerId)
  if (!providerId)
    issues.push({
      path: 'provider_profile_id',
      code: 'REQUIRED',
      severity: 'ERROR',
      message: 'No provider profile: set one on the suite axes, the cell or the test',
    })
  else if (!provider)
    issues.push({
      path: 'provider_profile_id',
      code: 'NOT_FOUND',
      severity: 'ERROR',
      message: 'Provider profile not found',
    })
  else if (provider.status !== 'ready')
    issues.push({
      path: 'provider_profile_id',
      code: 'PROVIDER_NOT_READY',
      severity: 'WARNING',
      message: `Provider ${provider.name} is ${provider.status}`,
    })
  if (cell.axis?.database_version) {
    const dbId = test.database && 'ref' in test.database ? test.database.ref.id : undefined
    const db = t.databases.find((d) => d.id === dbId)
    if (db && db.version === cell.axis.database_version)
      issues.push({
        path: 'axis.database_version',
        code: 'SAME_VERSION',
        severity: 'WARNING',
        message: `Test already runs ${db.version}; this cell duplicates the base test`,
      })
  }
  const sizes = cell.overrides?.sizes ?? cell.axis?.sizes ?? test.sizes
  if (sizes?.db?.size === 'XS' && test.summary?.db_kind && test.summary.db_kind !== 'postgres')
    issues.push({
      path: 'axis.sizes.db.size',
      code: 'SIZE_TOO_SMALL',
      severity: 'ERROR',
      message: `${test.summary.db_kind} needs at least S for the db role`,
      suggested: 'S',
    })
  return {
    fits: !issues.some((i) => i.severity === 'ERROR'),
    issues: issues.length ? issues : undefined,
  }
}

export function buildPreview(t: TenantData, spec: Schemas['SuiteSpec']): SuitePreview {
  const cells = computeCells(t, spec).map((c) => ({ ...c, validation: validateCell(t, c) }))
  const totals = { machines: 0, cpu: 0, memory_gb: 0, disk_gb: 0 }
  const byProvider = new Map<string, typeof totals>()
  let seconds = 0
  for (const c of cells) {
    if (!c.enabled) continue
    const test = testOf(t, c.test.id)
    const m = machinesOf(t, test, c.overrides?.sizes ?? c.axis?.sizes)
    totals.machines += m.machines
    totals.cpu += m.cpu
    totals.memory_gb += m.memory_gb
    totals.disk_gb += m.disk_gb
    seconds += 25 + 20 + 90 + 12 + 15
    const pid =
      c.overrides?.provider_profile_id ?? c.axis?.provider_profile_id ?? test?.provider_profile_id
    if (pid) {
      const agg = byProvider.get(pid) ?? { machines: 0, cpu: 0, memory_gb: 0, disk_gb: 0 }
      agg.machines += m.machines
      agg.cpu += m.cpu
      agg.memory_gb += m.memory_gb
      agg.disk_gb += m.disk_gb
      byProvider.set(pid, agg)
    }
  }
  const concurrency = Math.max(1, spec.concurrency ?? 1)
  const enabled = cells.filter((c) => c.enabled).length
  const waves = Math.ceil(enabled / concurrency)
  const quota_check = [...byProvider].map(([provider_profile_id, agg]) => {
    const report = t.quotas[provider_profile_id]
    const issues: string[] = []
    // peak usage = one wave of `concurrency` cells
    const share = enabled ? Math.min(1, concurrency / enabled) : 1
    for (const q of report?.quotas ?? []) {
      const need =
        q.name === 'compute.cores'
          ? agg.cpu * share
          : q.name === 'compute.memory'
            ? agg.memory_gb * share
            : q.name === 'compute.instances'
              ? agg.machines * share
              : q.name === 'compute.ssd'
                ? agg.disk_gb * share
                : 0
      if (need && q.used + need > q.limit)
        issues.push(
          `${q.title ?? q.name}: need ${Math.ceil(need)} ${q.unit ?? ''}, free ${Math.max(0, q.limit - q.used)}`
        )
    }
    if (report?.stale) issues.push('Quota data is stale; re-verify the provider')
    return { provider_profile_id, fits: !issues.some((i) => !i.startsWith('Quota data')), issues }
  })
  const errors = cells.flatMap((c) =>
    (c.validation?.issues ?? [])
      .filter((i) => i.severity === 'ERROR')
      .map((i) => ({ ...i, path: `cells.${c.id}.${i.path}` }))
  )
  return {
    cells,
    validation: { fits: errors.length === 0, issues: errors.length ? errors : undefined },
    totals: { ...totals, estimated_duration: durationStr(waves * seconds * 1000) },
    quota_check,
  }
}

function decorateSuite(t: TenantData, s: Suite): Suite {
  const runs = t.suiteRuns.filter((r) => r.suite.id === s.id)
  const last = sortBy(runs, (r) => r.created_at, 'desc')[0]
  const cells = s.cells ?? []
  return {
    ...s,
    tests: (s.tests ?? []).map((x) => ('ref' in x ? { ref: testRef(t, x.ref) } : x)),
    cells: cells.map((c) => ({ ...c, test: testRef(t, c.test), validation: validateCell(t, c) })),
    is_favorite: t.favorites.has(`suite:${s.id}`),
    summary: {
      cell_count: cells.length,
      enabled_cell_count: cells.filter((c) => c.enabled).length,
      run_count: runs.length,
      last_run: last
        ? { id: last.id, name: last.name, status: last.status, started_at: last.started_at }
        : undefined,
      schedules: t.schedules
        .filter((sc) => sc.target.kind === 'suite' && sc.target.id === s.id)
        .map((sc) => ({ id: sc.id, name: sc.name })),
    },
  }
}

function validateSuiteWrite(body: Partial<SuiteWrite>, requireName: boolean) {
  const errors: { path: string; code: string; severity: 'ERROR'; message: string }[] = []
  if (requireName && !body.name?.trim())
    errors.push({ path: 'name', code: 'REQUIRED', severity: 'ERROR', message: 'Name is required' })
  if (body.name !== undefined && body.name.trim().length > 120)
    errors.push({
      path: 'name',
      code: 'MAX',
      severity: 'ERROR',
      message: 'Name is longer than 120 characters',
    })
  if (body.concurrency !== undefined && (body.concurrency < 1 || body.concurrency > 16))
    errors.push({
      path: 'concurrency',
      code: 'RANGE',
      severity: 'ERROR',
      message: 'Concurrency must be between 1 and 16',
    })
  if (body.defaults?.keep && !/^\d+(h|m|s)$/.test(body.defaults.keep))
    errors.push({
      path: 'defaults.keep',
      code: 'FORMAT',
      severity: 'ERROR',
      message: 'Use a Go duration like 2h or 30m',
    })
  return errors.length
    ? problem(400, 'validation_failed', 'Fix the highlighted fields', { validation: { errors } })
    : undefined
}

// ---------- suite run execution ----------
function refreshProgress(sr: SuiteRun): void {
  const cells = sr.cells
  const done = cells.filter((c) => TERMINAL.includes(c.status)).length
  const failed = cells.filter((c) => c.status === 'failed').length
  const cancelled = cells.filter((c) => c.status === 'cancelled').length
  const running = cells.filter((c) => c.status === 'running' || c.status === 'cancelling').length
  sr.progress = {
    total: cells.length,
    done,
    failed,
    running,
    cancelled,
    pending: cells.length - done - running,
    pct: cells.length ? Math.round((done / cells.length) * 100) : 0,
  }
}

function overridesForCell(
  sr: SuiteRun,
  suite: Suite | undefined,
  cell: SuiteCell | undefined,
  test: Test
): LaunchOverrides {
  const axis = cell?.axis
  const o: LaunchOverrides = {
    name: `${test.name} · ${sr.name}`,
    provider_profile_id:
      cell?.overrides?.provider_profile_id ?? axis?.provider_profile_id ?? undefined,
    sizes: cell?.overrides?.sizes ?? axis?.sizes,
    keep: cell?.overrides?.keep ?? suite?.defaults?.keep,
    rating: cell?.overrides?.rating ?? suite?.defaults?.rating,
    labels: {
      suite: suite?.name ?? sr.suite.name ?? sr.suite.id,
      suite_run: sr.name,
      ...(axis?.database_version ? { db_version: axis.database_version } : {}),
      ...(axis?.workload_variant ? { variant: axis.workload_variant } : {}),
      ...(cell?.overrides?.labels ?? {}),
      ...(sr.labels ?? {}),
    },
  }
  return o
}

// Starts pending cells of a running suite run up to its concurrency.
function fillSlots(store: MockStore, slug: string, t: TenantData, sr: SuiteRun): boolean {
  if (sr.status !== 'running') return false
  const suite = t.suites.find((s) => s.id === sr.suite.id)
  const limit = Math.max(1, sr.concurrency ?? suite?.concurrency ?? 1)
  let active = sr.cells.filter((c) => c.run && !TERMINAL.includes(c.status)).length
  let changed = false
  for (const c of sr.cells) {
    if (active >= limit) break
    if (c.run || c.status !== 'pending') continue
    const cell = suite?.cells?.find((x) => x.id === c.cell_id)
    const test = testOf(t, cell?.test.id) ?? t.tests.find((x) => x.name === c.name)
    if (!test) {
      c.status = 'failed'
      c.summary = undefined
      changed = true
      continue
    }
    const run = launchFromTest(
      store,
      slug,
      t,
      test,
      overridesForCell(sr, suite, cell, test),
      'suite',
      {
        suite_run_id: sr.id,
        cell_id: c.cell_id,
        ...(sr.trigger_ref?.schedule_id ? { schedule_id: sr.trigger_ref.schedule_id } : {}),
      }
    )
    c.run = { id: run.id, name: run.name, status: run.status, started_at: run.started_at }
    c.status = run.status
    active += 1
    changed = true
  }
  return changed
}

// Reconciles every suite run each second: cell status ← run status, slot filling, cancellation.
// The base simulation already publishes progress for `running` suite runs; this tick adds
// scheduling and terminal handling for `cancelling`.
let schedulerTimer: number | undefined
function ensureScheduler(store: MockStore): void {
  if (schedulerTimer !== undefined || typeof window === 'undefined') return
  schedulerTimer = window.setInterval(() => {
    const sim = getSimulation(store)
    for (const [slug, t] of Object.entries(store.tenants)) {
      for (const sr of t.suiteRuns) {
        if (TERMINAL.includes(sr.status)) continue
        let changed = false
        for (const c of sr.cells) {
          if (!c.run) continue
          const r = t.runs.find((x) => x.id === c.run?.id)
          if (r && (r.status !== c.status || r.status !== c.run.status)) {
            c.status = r.status
            c.run = { id: r.id, name: r.name, status: r.status, started_at: r.started_at }
            c.summary = r.summary
            changed = true
          }
        }
        if (sr.status === 'running' && fillSlots(store, slug, t, sr)) changed = true
        if (sr.status === 'cancelling') {
          const allDone = sr.cells.every((c) => TERMINAL.includes(c.status))
          if (allDone) {
            sr.status = 'cancelled'
            sr.finished_at = iso()
            sr.duration = durationStr(
              Date.now() - new Date(sr.started_at ?? sr.created_at).getTime()
            )
            changed = true
          }
        }
        if (changed) {
          refreshProgress(sr)
          sim.emit(`suite_run/${sr.id}`, sr)
        }
      }
    }
  }, 1000)
}

function createSuiteRun(
  store: MockStore,
  slug: string,
  t: TenantData,
  suite: Suite,
  cells: SuiteCell[],
  opts: {
    name?: string
    concurrency?: number
    trigger: SuiteRun['trigger']
    trigger_ref?: SuiteRun['trigger_ref']
    labels?: Record<string, string>
  }
): SuiteRun {
  const count = t.suiteRuns.filter((r) => r.suite.id === suite.id).length + 1
  const sr: SuiteRun = {
    id: uuid(),
    name: opts.name?.trim() || `${suite.name} #${count}`,
    suite: { id: suite.id, name: suite.name },
    status: 'running',
    trigger: opts.trigger,
    trigger_ref: opts.trigger_ref,
    concurrency: opts.concurrency ?? suite.concurrency ?? 1,
    progress: {
      total: cells.length,
      done: 0,
      failed: 0,
      running: 0,
      pending: cells.length,
      pct: 0,
    },
    cells: cells.map((c) => ({ cell_id: c.id, name: c.name, status: 'pending' as RunStatus })),
    author: { id: store.me.id, display_name: store.me.display_name },
    labels: opts.labels,
    created_at: iso(),
    started_at: iso(),
    finished_at: null,
    duration: null,
    graphene: { run_ref: `suite-run/${count}` },
  }
  t.suiteRuns.unshift(sr)
  fillSlots(store, slug, t, sr)
  refreshProgress(sr)
  store.audit(slug, 'suite_run.launch', { kind: 'suite_run', id: sr.id, name: sr.name })
  ensureScheduler(store)
  return sr
}

// ---------- suites ----------
route('GET', '/api/v1/t/:slug/suites', ({ store, params, query }) => {
  const t = tenantOf(store, params.slug)
  if (!t) return notFound('tenant')
  const lq = parseListQuery(query, 'updated_at')
  const tags = multi(query, 'tags')
  const author = query.get('author')
  const favOnly = query.get('favorites') === 'true'
  let items = t.suites.map((s) => decorateSuite(t, s))
  items = items.filter((s) => {
    if (!matchesSearch(`${s.name} ${s.description ?? ''}`, lq.search)) return false
    if (favOnly && !s.is_favorite) return false
    if (author && s.author.id !== author && s.author.display_name !== author) return false
    for (const tag of tags) {
      const [k, v] = tag.split('=')
      if (!s.tags || !(k in s.tags) || (v !== undefined && s.tags[k] !== v)) return false
    }
    return true
  })
  const sorted =
    lq.sort === 'name'
      ? sortBy(items, (s) => s.name.toLowerCase(), lq.order)
      : lq.sort === 'created_at'
        ? sortBy(items, (s) => s.created_at, lq.order)
        : lq.sort === 'last_run_at'
          ? sortBy(items, (s) => s.summary?.last_run?.started_at ?? '', lq.order)
          : lq.sort === 'cell_count'
            ? sortBy(items, (s) => s.summary?.cell_count ?? 0, lq.order)
            : sortBy(items, (s) => s.updated_at, lq.order)
  return { json: paginate(sorted, lq) }
})

route('POST', '/api/v1/t/:slug/suites', ({ store, params, body }) => {
  const t = tenantOf(store, params.slug)
  if (!t) return notFound('tenant')
  const b = (body ?? {}) as SuiteWrite
  const invalid = validateSuiteWrite(b, true)
  if (invalid) return invalid
  const spec: Schemas['SuiteSpec'] = {
    tests: b.tests ?? [],
    axes: b.axes ?? {},
    concurrency: b.concurrency ?? 1,
    defaults: b.defaults,
    cells: b.cells,
  }
  const suite: Suite = {
    id: uuid(),
    name: b.name.trim(),
    description: b.description?.trim() || undefined,
    tags: b.tags,
    author: { id: store.me.id, display_name: store.me.display_name },
    created_at: iso(),
    updated_at: iso(),
    ...spec,
    cells: computeCells(t, spec),
  }
  t.suites.unshift(suite)
  store.audit(params.slug, 'suite.create', { kind: 'suite', id: suite.id, name: suite.name })
  return { status: 201, json: decorateSuite(t, suite) }
})

route('POST', '/api/v1/t/:slug/suites:preview', ({ store, params, body }) => {
  const t = tenantOf(store, params.slug)
  if (!t) return notFound('tenant')
  const b = (body ?? {}) as SuiteWrite
  return { json: buildPreview(t, { ...b, concurrency: b.concurrency ?? 1 }) }
})

route('POST', '/api/v1/t/:slug/suites:import', ({ store, params, body }) => {
  const t = tenantOf(store, params.slug)
  if (!t) return notFound('tenant')
  const doc = body as Schemas['ExportDocument'] | undefined
  if (doc?.kind !== 'Suite' || !doc.spec)
    return problem(400, 'validation_failed', 'Expected an ExportDocument of kind Suite', {
      validation: {
        errors: [
          { path: 'kind', code: 'KIND', severity: 'ERROR', message: 'kind must be "Suite"' },
        ],
      },
    })
  const spec = doc.spec as Partial<Schemas['SuiteSpec']>
  const base = doc.metadata?.name?.trim() || 'Imported suite'
  const taken = new Set(t.suites.map((s) => s.name))
  let name = base
  for (let i = 2; taken.has(name); i++) name = `${base} (${i})`
  const full: Schemas['SuiteSpec'] = {
    tests: spec.tests ?? [],
    axes: spec.axes ?? {},
    cells: spec.cells,
    concurrency: spec.concurrency ?? 1,
    defaults: spec.defaults,
  }
  const suite: Suite = {
    id: uuid(),
    name,
    description: doc.metadata?.description,
    tags: doc.metadata?.tags,
    author: { id: store.me.id, display_name: store.me.display_name },
    created_at: iso(),
    updated_at: iso(),
    ...full,
    cells: computeCells(t, full),
  }
  t.suites.unshift(suite)
  store.audit(params.slug, 'suite.import', { kind: 'suite', id: suite.id, name: suite.name })
  return { json: decorateSuite(t, suite) }
})

route('GET', '/api/v1/t/:slug/suites/:id', ({ store, params }) => {
  const t = tenantOf(store, params.slug)
  const s = t?.suites.find((x) => x.id === params.id)
  if (!t || !s) return notFound('suite')
  return { json: decorateSuite(t, s) }
})

route('PATCH', '/api/v1/t/:slug/suites/:id', ({ store, params, body }) => {
  const t = tenantOf(store, params.slug)
  const s = t?.suites.find((x) => x.id === params.id)
  if (!t || !s) return notFound('suite')
  const b = (body ?? {}) as Schemas['SuitePatch']
  const invalid = validateSuiteWrite(b as Partial<SuiteWrite>, false)
  if (invalid) return invalid
  if (b.name !== undefined) s.name = b.name.trim()
  if (b.description !== undefined) s.description = b.description.trim() || undefined
  if (b.tags !== undefined) s.tags = b.tags
  if (b.concurrency !== undefined) s.concurrency = b.concurrency
  if (b.defaults !== undefined) s.defaults = b.defaults
  const axesChanged = b.tests !== undefined || b.axes !== undefined
  if (b.tests !== undefined) s.tests = b.tests
  if (b.axes !== undefined) s.axes = b.axes
  if (b.cells !== undefined) s.cells = b.cells.map((c) => ({ ...c, validation: undefined }))
  // axes/tests change regenerates the matrix, preserving manual edits by cell id
  if (axesChanged) s.cells = computeCells(t, { ...s, cells: b.cells ?? s.cells })
  s.updated_at = iso()
  store.audit(params.slug, 'suite.update', { kind: 'suite', id: s.id, name: s.name })
  return { json: decorateSuite(t, s) }
})

route('DELETE', '/api/v1/t/:slug/suites/:id', ({ store, params }) => {
  const t = tenantOf(store, params.slug)
  const i = t?.suites.findIndex((x) => x.id === params.id) ?? -1
  if (!t || i < 0) return notFound('suite')
  const [s] = t.suites.splice(i, 1)
  for (const sc of t.schedules)
    if (sc.target.kind === 'suite' && sc.target.id === s.id) {
      sc.enabled = false
      sc.next_run_at = null
    }
  t.favorites.delete(`suite:${s.id}`)
  store.audit(params.slug, 'suite.delete', { kind: 'suite', id: s.id, name: s.name })
  return noContent()
})

route('POST', '/api/v1/t/:slug/suites/:id:launch', ({ store, params, body }) => {
  const t = tenantOf(store, params.slug)
  const s = t?.suites.find((x) => x.id === params.id)
  if (!t || !s) return notFound('suite')
  const b = (body ?? {}) as Schemas['SuiteLaunch']
  let cells = (s.cells ?? []).filter((c) => c.enabled)
  if (b.cell_ids?.length) cells = cells.filter((c) => b.cell_ids?.includes(c.id))
  if (!cells.length)
    return problem(400, 'validation_failed', 'No enabled cells to launch', {
      validation: {
        errors: [
          { path: 'cells', code: 'EMPTY', severity: 'ERROR', message: 'Enable at least one cell' },
        ],
      },
    })
  const blocked = cells.filter((c) => !validateCell(t, c).fits)
  if (blocked.length)
    return problem(400, 'validation_failed', `${blocked.length} cell(s) fail validation`, {
      validation: {
        errors: blocked.map((c) => ({
          path: `cells.${c.id}`,
          code: 'CELL_INVALID',
          severity: 'ERROR' as const,
          message: `${c.name ?? c.id}: ${validateCell(t, c).issues?.[0]?.message ?? 'invalid'}`,
        })),
      },
    })
  if (b.concurrency !== undefined && (b.concurrency < 1 || b.concurrency > 16))
    return problem(400, 'validation_failed', 'Concurrency must be between 1 and 16', {
      validation: {
        errors: [
          { path: 'concurrency', code: 'RANGE', severity: 'ERROR', message: 'Between 1 and 16' },
        ],
      },
    })
  const suiteForRun: Suite = {
    ...s,
    defaults: { rating: b.rating ?? s.defaults?.rating, keep: b.keep ?? s.defaults?.keep },
  }
  const sr = createSuiteRun(store, params.slug, t, suiteForRun, cells, {
    name: b.name,
    concurrency: b.concurrency,
    trigger: 'manual',
    labels: b.labels,
  })
  return { status: 201, json: sr }
})

route('POST', '/api/v1/t/:slug/suites/:id:clone', ({ store, params, body }) => {
  const t = tenantOf(store, params.slug)
  const s = t?.suites.find((x) => x.id === params.id)
  if (!t || !s) return notFound('suite')
  const b = (body ?? {}) as Schemas['CloneRequest']
  const clone: Suite = {
    ...structuredClone(s),
    id: uuid(),
    name: b.name?.trim() || `${s.name} (copy)`,
    author: { id: store.me.id, display_name: store.me.display_name },
    created_at: iso(),
    updated_at: iso(),
    is_favorite: false,
    summary: undefined,
  }
  t.suites.unshift(clone)
  store.audit(params.slug, 'suite.clone', { kind: 'suite', id: clone.id, name: clone.name })
  return { status: 201, json: decorateSuite(t, clone) }
})

route('GET', '/api/v1/t/:slug/suites/:id:export', ({ store, params }) => {
  const t = tenantOf(store, params.slug)
  const s = t?.suites.find((x) => x.id === params.id)
  if (!t || !s) return notFound('suite')
  const doc: Schemas['ExportDocument'] = {
    api_version: 'stroppy.io/v1',
    kind: 'Suite',
    metadata: { name: s.name, description: s.description, tags: s.tags },
    spec: {
      tests: s.tests,
      axes: s.axes,
      cells: (s.cells ?? []).map((c) => ({
        id: c.id,
        name: c.name,
        test: c.test,
        enabled: c.enabled,
        axis: c.axis,
        overrides: c.overrides,
      })),
      concurrency: s.concurrency,
      defaults: s.defaults,
    },
  }
  return { json: doc }
})

route('GET', '/api/v1/t/:slug/suites/:id/runs', ({ store, params, query }) => {
  const t = tenantOf(store, params.slug)
  const s = t?.suites.find((x) => x.id === params.id)
  if (!t || !s) return notFound('suite')
  ensureScheduler(store)
  const lq = parseListQuery(query, 'created_at')
  const items = sortBy(
    t.suiteRuns.filter((r) => r.suite.id === s.id),
    (r) => r.created_at,
    'desc'
  )
  return { json: paginate(items, lq) }
})

// ---------- suite runs ----------
route('GET', '/api/v1/t/:slug/suite-runs', ({ store, params, query }) => {
  const t = tenantOf(store, params.slug)
  if (!t) return notFound('tenant')
  ensureScheduler(store)
  const lq = parseListQuery(query, 'started_at')
  const status = multi(query, 'status')
  const trigger = multi(query, 'trigger')
  const suiteId = query.get('suite_id')
  const after = query.get('started_after')
  const before = query.get('started_before')
  const items = t.suiteRuns.filter((r) => {
    if (status.length && !status.includes(r.status)) return false
    if (trigger.length && !trigger.includes(r.trigger)) return false
    if (suiteId && r.suite.id !== suiteId) return false
    if (after && (r.started_at ?? '') < after) return false
    if (before && (r.started_at ?? '') > before) return false
    return true
  })
  const sorted =
    lq.sort === 'finished_at'
      ? sortBy(items, (r) => r.finished_at ?? '', lq.order)
      : lq.sort === 'status'
        ? sortBy(items, (r) => r.status, lq.order)
        : lq.sort === 'progress'
          ? sortBy(items, (r) => r.progress.pct ?? 0, lq.order)
          : lq.sort === 'duration'
            ? sortBy(
                items,
                (r) =>
                  r.finished_at
                    ? new Date(r.finished_at).getTime() -
                      new Date(r.started_at ?? r.created_at).getTime()
                    : 0,
                lq.order
              )
            : sortBy(items, (r) => r.started_at ?? r.created_at, lq.order)
  return { json: paginate(sorted, lq) }
})

function getSuiteRun(store: MockStore, slug: string, id: string) {
  const t = tenantOf(store, slug)
  const sr = t?.suiteRuns.find((x) => x.id === id)
  return { t, sr }
}

route('GET', '/api/v1/t/:slug/suite-runs/:id', ({ store, params }) => {
  const { t, sr } = getSuiteRun(store, params.slug, params.id)
  if (!t || !sr) return notFound('suite run')
  ensureScheduler(store)
  return { json: sr }
})

route('DELETE', '/api/v1/t/:slug/suite-runs/:id', ({ store, params }) => {
  const t = tenantOf(store, params.slug)
  const i = t?.suiteRuns.findIndex((x) => x.id === params.id) ?? -1
  if (!t || i < 0) return notFound('suite run')
  if (!TERMINAL.includes(t.suiteRuns[i].status))
    return problem(409, 'conflict', 'Cancel the suite run before deleting it')
  const [sr] = t.suiteRuns.splice(i, 1)
  store.audit(params.slug, 'suite_run.delete', { kind: 'suite_run', id: sr.id, name: sr.name })
  return noContent()
})

route('POST', '/api/v1/t/:slug/suite-runs/:id:cancel', ({ store, params }) => {
  const { t, sr } = getSuiteRun(store, params.slug, params.id)
  if (!t || !sr) return notFound('suite run')
  if (TERMINAL.includes(sr.status)) return problem(409, 'conflict', 'Suite run already finished')
  const sim = getSimulation(store)
  sr.status = 'cancelling'
  for (const c of sr.cells) {
    if (TERMINAL.includes(c.status)) continue
    const run = c.run ? t.runs.find((r) => r.id === c.run?.id) : undefined
    if (run && (run.status === 'running' || run.status === 'cancelling')) {
      if (run.status === 'running') sim.cancelRun(params.slug, run)
      c.status = 'cancelling'
    } else if (run && run.status === 'pending') {
      run.status = 'cancelled'
      run.status_reason = 'cancelled with suite run'
      run.finished_at = iso()
      c.status = 'cancelled'
      c.run = { ...c.run, status: 'cancelled' } as NonNullable<typeof c.run>
    } else {
      c.status = 'cancelled'
    }
  }
  refreshProgress(sr)
  sim.emit(`suite_run/${sr.id}`, sr)
  ensureScheduler(store)
  store.audit(params.slug, 'suite_run.cancel', { kind: 'suite_run', id: sr.id, name: sr.name })
  return { json: sr }
})

route('POST', '/api/v1/t/:slug/suite-runs/:id:retry-failed', ({ store, params }) => {
  const { t, sr } = getSuiteRun(store, params.slug, params.id)
  if (!t || !sr) return notFound('suite run')
  const suite = t.suites.find((s) => s.id === sr.suite.id)
  if (!suite) return problem(409, 'conflict', 'The suite of this run no longer exists')
  const failedIds = sr.cells
    .filter((c) => c.status === 'failed' || c.status === 'cancelled')
    .map((c) => c.cell_id)
  const cells = (suite.cells ?? []).filter((c) => failedIds.includes(c.id))
  if (!cells.length) return problem(409, 'conflict', 'No failed cells to retry')
  const retry = createSuiteRun(store, params.slug, t, suite, cells, {
    name: `${sr.name} · retry`,
    concurrency: sr.concurrency,
    trigger: 'manual',
    trigger_ref: { retry_of: sr.id },
    labels: sr.labels,
  })
  return { status: 201, json: retry }
})

route('POST', '/api/v1/t/:slug/suite-runs/:id:share', ({ store, params, body }) => {
  const { t, sr } = getSuiteRun(store, params.slug, params.id)
  if (!t || !sr) return notFound('suite run')
  const share = createShare(
    t,
    store,
    { kind: 'suite_run', id: sr.id, name: sr.name },
    (body ?? {}) as Schemas['ShareCreate']
  )
  store.audit(params.slug, 'share.create', { kind: 'share', id: share.id, name: sr.name })
  return { status: 201, json: share }
})

const SUMMARY_METRICS: {
  key: string
  higher_is_better: boolean
  from: (r: Run) => number | undefined
}[] = [
  {
    key: 'tps',
    higher_is_better: true,
    from: (r) => r.result?.metrics?.tps?.value ?? r.summary?.headline?.tps,
  },
  {
    key: 'latency_p99_ms',
    higher_is_better: false,
    from: (r) => r.result?.metrics?.latency_p99_ms?.value ?? r.summary?.headline?.latency_p99_ms,
  },
  {
    key: 'errors',
    higher_is_better: false,
    from: (r) => r.result?.metrics?.errors?.value ?? r.summary?.headline?.errors,
  },
]

route('GET', '/api/v1/t/:slug/suite-runs/:id/summary', ({ store, params, query }) => {
  const { t, sr } = getSuiteRun(store, params.slug, params.id)
  if (!t || !sr) return notFound('suite run')
  const compareTo = query.get('compare_to')
  const previous = compareTo
    ? t.suiteRuns.find((r) => r.id === compareTo)
    : sortBy(
        t.suiteRuns.filter(
          (r) =>
            r.suite.id === sr.suite.id &&
            r.id !== sr.id &&
            r.created_at < sr.created_at &&
            TERMINAL.includes(r.status)
        ),
        (r) => r.created_at,
        'desc'
      )[0]
  const deadband = 5
  const runOf = (ref: { id: string } | undefined) =>
    ref ? t.runs.find((r) => r.id === ref.id) : undefined
  const rows: SuiteRunSummary['rows'] = sr.cells.map((c) => {
    const run = runOf(c.run)
    const prevCell = previous?.cells.find((p) => p.cell_id === c.cell_id)
    const prevRun = runOf(prevCell?.run)
    const metrics: NonNullable<SuiteRunSummary['rows'][number]['metrics']> = {}
    for (const m of SUMMARY_METRICS) {
      const value = run ? m.from(run) : undefined
      const prev = prevRun ? m.from(prevRun) : undefined
      if (value === undefined && prev === undefined) continue
      let diff_pct: number | undefined
      let verdict: 'better' | 'worse' | 'same' | 'missing' = 'missing'
      if (value !== undefined && prev !== undefined) {
        diff_pct =
          prev === 0 ? (value === 0 ? 0 : 100) : Math.round(((value - prev) / prev) * 1000) / 10
        const better = m.higher_is_better ? diff_pct > deadband : diff_pct < -deadband
        const worse = m.higher_is_better ? diff_pct < -deadband : diff_pct > deadband
        verdict = better ? 'better' : worse ? 'worse' : 'same'
      }
      metrics[m.key] = { value, previous: prev, diff_pct, verdict }
    }
    return { cell_id: c.cell_id, name: c.name, run: c.run, status: c.status, metrics }
  })
  const out: SuiteRunSummary = {
    suite_run_id: sr.id,
    compared_to: previous?.id ?? null,
    metric_keys: SUMMARY_METRICS.map((m) => m.key),
    rows,
  }
  return { json: out }
})

// ---------- schedules ----------
function decorateSchedule(t: TenantData, s: Schedule): Schedule {
  const targetName =
    s.target.kind === 'test'
      ? testOf(t, s.target.id)?.name
      : t.suites.find((x) => x.id === s.target.id)?.name
  return { ...s, target: { ...s.target, name: targetName ?? s.target.name } }
}

function recomputeNext(s: Schedule): void {
  s.next_run_at = s.enabled ? (nextFireTime(s.cron, s.timezone)?.toISOString() ?? null) : null
}

function validateSchedule(
  t: TenantData,
  b: Partial<Schemas['ScheduleWrite']>,
  requireAll: boolean
) {
  const errors: { path: string; code: string; severity: 'ERROR'; message: string }[] = []
  if ((requireAll || b.name !== undefined) && !b.name?.trim())
    errors.push({ path: 'name', code: 'REQUIRED', severity: 'ERROR', message: 'Name is required' })
  if (requireAll || b.cron !== undefined) {
    if (!b.cron || nextFireTime(b.cron, b.timezone ?? 'UTC') === undefined)
      errors.push({
        path: 'cron',
        code: 'FORMAT',
        severity: 'ERROR',
        message: '5 fields: minute hour day month weekday — e.g. 0 2 * * *',
      })
  }
  if (b.timezone !== undefined) {
    try {
      new Intl.DateTimeFormat('en-US', { timeZone: b.timezone })
    } catch {
      errors.push({
        path: 'timezone',
        code: 'FORMAT',
        severity: 'ERROR',
        message: 'Unknown IANA timezone',
      })
    }
  }
  if (requireAll || b.target !== undefined) {
    const tg = b.target
    if (!tg?.id)
      errors.push({
        path: 'target.id',
        code: 'REQUIRED',
        severity: 'ERROR',
        message: 'Pick a test or a suite',
      })
    else if (tg.kind === 'test' && !testOf(t, tg.id))
      errors.push({
        path: 'target.id',
        code: 'NOT_FOUND',
        severity: 'ERROR',
        message: 'Test not found',
      })
    else if (tg.kind === 'suite' && !t.suites.find((s) => s.id === tg.id))
      errors.push({
        path: 'target.id',
        code: 'NOT_FOUND',
        severity: 'ERROR',
        message: 'Suite not found',
      })
  }
  if (b.overrides?.keep && !/^\d+(h|m|s)$/.test(b.overrides.keep))
    errors.push({
      path: 'overrides.keep',
      code: 'FORMAT',
      severity: 'ERROR',
      message: 'Use a Go duration like 2h or 30m',
    })
  return errors.length
    ? problem(400, 'validation_failed', 'Fix the highlighted fields', { validation: { errors } })
    : undefined
}

route('GET', '/api/v1/t/:slug/schedules', ({ store, params, query }) => {
  const t = tenantOf(store, params.slug)
  if (!t) return notFound('tenant')
  const lq = parseListQuery(query, 'name')
  const kind = query.get('target_kind')
  const enabled = query.get('enabled')
  const items = t.schedules
    .map((s) => decorateSchedule(t, s))
    .filter((s) => {
      if (kind && s.target.kind !== kind) return false
      if (enabled === 'true' && !s.enabled) return false
      if (enabled === 'false' && s.enabled) return false
      if (!matchesSearch(`${s.name} ${s.target.name ?? ''} ${s.cron}`, lq.search)) return false
      return true
    })
  const sorted =
    lq.sort === 'next_run_at'
      ? sortBy(items, (s) => s.next_run_at ?? '9999', lq.order)
      : lq.sort === 'created_at'
        ? sortBy(items, (s) => s.created_at, lq.order)
        : sortBy(items, (s) => s.name.toLowerCase(), lq.order === 'desc' ? 'desc' : 'asc')
  return { json: paginate(sorted, lq) }
})

route('POST', '/api/v1/t/:slug/schedules', ({ store, params, body }) => {
  const t = tenantOf(store, params.slug)
  if (!t) return notFound('tenant')
  const b = (body ?? {}) as Schemas['ScheduleWrite']
  const invalid = validateSchedule(t, b, true)
  if (invalid) return invalid
  const s: Schedule = {
    id: uuid(),
    name: b.name.trim(),
    target: { kind: b.target.kind, id: b.target.id },
    cron: b.cron.trim().replace(/\s+/g, ' '),
    timezone: b.timezone || 'UTC',
    enabled: b.enabled ?? true,
    overrides: b.overrides,
    author: { id: store.me.id, display_name: store.me.display_name },
    created_at: iso(),
    updated_at: iso(),
  }
  recomputeNext(s)
  t.schedules.unshift(s)
  store.audit(params.slug, 'schedule.create', { kind: 'schedule', id: s.id, name: s.name })
  return { status: 201, json: decorateSchedule(t, s) }
})

function getSchedule(store: MockStore, slug: string, id: string) {
  const t = tenantOf(store, slug)
  const s = t?.schedules.find((x) => x.id === id)
  return { t, s }
}

route('GET', '/api/v1/t/:slug/schedules/:id', ({ store, params }) => {
  const { t, s } = getSchedule(store, params.slug, params.id)
  if (!t || !s) return notFound('schedule')
  return { json: decorateSchedule(t, s) }
})

route('PATCH', '/api/v1/t/:slug/schedules/:id', ({ store, params, body }) => {
  const { t, s } = getSchedule(store, params.slug, params.id)
  if (!t || !s) return notFound('schedule')
  const b = (body ?? {}) as Schemas['SchedulePatch']
  const invalid = validateSchedule(t, { ...b, timezone: b.timezone ?? s.timezone }, false)
  if (invalid) return invalid
  if (b.name !== undefined) s.name = b.name.trim()
  if (b.target !== undefined) s.target = { kind: b.target.kind, id: b.target.id }
  if (b.cron !== undefined) s.cron = b.cron.trim().replace(/\s+/g, ' ')
  if (b.timezone !== undefined) s.timezone = b.timezone
  if (b.enabled !== undefined) s.enabled = b.enabled
  if (b.overrides !== undefined) s.overrides = b.overrides
  s.updated_at = iso()
  recomputeNext(s)
  store.audit(params.slug, 'schedule.update', { kind: 'schedule', id: s.id, name: s.name })
  return { json: decorateSchedule(t, s) }
})

route('DELETE', '/api/v1/t/:slug/schedules/:id', ({ store, params }) => {
  const t = tenantOf(store, params.slug)
  const i = t?.schedules.findIndex((x) => x.id === params.id) ?? -1
  if (!t || i < 0) return notFound('schedule')
  const [s] = t.schedules.splice(i, 1)
  t.favorites.delete(`schedule:${s.id}`)
  store.audit(params.slug, 'schedule.delete', { kind: 'schedule', id: s.id, name: s.name })
  return noContent()
})

route('POST', '/api/v1/t/:slug/schedules/:id:pause', ({ store, params }) => {
  const { t, s } = getSchedule(store, params.slug, params.id)
  if (!t || !s) return notFound('schedule')
  s.enabled = false
  s.updated_at = iso()
  recomputeNext(s)
  store.audit(params.slug, 'schedule.pause', { kind: 'schedule', id: s.id, name: s.name })
  return { json: decorateSchedule(t, s) }
})

route('POST', '/api/v1/t/:slug/schedules/:id:resume', ({ store, params }) => {
  const { t, s } = getSchedule(store, params.slug, params.id)
  if (!t || !s) return notFound('schedule')
  s.enabled = true
  s.updated_at = iso()
  recomputeNext(s)
  store.audit(params.slug, 'schedule.resume', { kind: 'schedule', id: s.id, name: s.name })
  return { json: decorateSchedule(t, s) }
})

route('POST', '/api/v1/t/:slug/schedules/:id:run-now', ({ store, params }) => {
  const { t, s } = getSchedule(store, params.slug, params.id)
  if (!t || !s) return notFound('schedule')
  let ref: Schemas['ScheduleRunRef']
  if (s.target.kind === 'test') {
    const test = testOf(t, s.target.id)
    if (!test) return problem(409, 'conflict', 'The target test no longer exists')
    const run = launchFromTest(
      store,
      params.slug,
      t,
      test,
      { ...s.overrides, name: `${test.name} · ${s.name}` },
      'schedule',
      {
        schedule_id: s.id,
      }
    )
    ref = { kind: 'run', id: run.id, name: run.name, status: run.status, at: iso() }
  } else {
    const suite = t.suites.find((x) => x.id === s.target.id)
    if (!suite) return problem(409, 'conflict', 'The target suite no longer exists')
    const cells = (suite.cells ?? []).filter((c) => c.enabled)
    if (!cells.length) return problem(409, 'conflict', 'The suite has no enabled cells')
    const suiteForRun: Suite = {
      ...suite,
      defaults: {
        rating: s.overrides?.rating ?? suite.defaults?.rating,
        keep: s.overrides?.keep ?? suite.defaults?.keep,
      },
      cells: cells.map((c) => ({
        ...c,
        overrides: {
          ...c.overrides,
          provider_profile_id: c.overrides?.provider_profile_id ?? s.overrides?.provider_profile_id,
          sizes: c.overrides?.sizes ?? s.overrides?.sizes,
        },
      })),
    }
    const sr = createSuiteRun(store, params.slug, t, suiteForRun, suiteForRun.cells ?? [], {
      trigger: 'schedule',
      trigger_ref: { schedule_id: s.id },
      labels: s.overrides?.labels,
    })
    ref = { kind: 'suite_run', id: sr.id, name: sr.name, status: sr.status, at: iso() }
  }
  s.last_run = ref
  store.audit(params.slug, 'schedule.run_now', { kind: 'schedule', id: s.id, name: s.name })
  return { status: 201, json: ref }
})

route('GET', '/api/v1/t/:slug/schedules/:id/history', ({ store, params, query }) => {
  const { t, s } = getSchedule(store, params.slug, params.id)
  if (!t || !s) return notFound('schedule')
  const lq = parseListQuery(query, 'at')
  const runs: Schemas['ScheduleRunRef'][] = t.runs
    .filter((r) => r.trigger_ref?.schedule_id === s.id && !r.trigger_ref?.suite_run_id)
    .map((r) => ({ kind: 'run', id: r.id, name: r.name, status: r.status, at: r.created_at }))
  const suiteRuns: Schemas['ScheduleRunRef'][] = t.suiteRuns
    .filter((r) => r.trigger_ref?.schedule_id === s.id)
    .map((r) => ({ kind: 'suite_run', id: r.id, name: r.name, status: r.status, at: r.created_at }))
  const items = sortBy([...runs, ...suiteRuns], (x) => x.at, 'desc')
  // keep the seeded last_run visible even when its record is not in the tenant (fixture drift)
  if (s.last_run && !items.find((x) => x.id === s.last_run?.id)) items.push(s.last_run)
  return {
    json: paginate(
      sortBy(items, (x) => x.at, 'desc'),
      lq
    ),
  }
})

// ---------- neighbouring reads (registered only if another area did not) ----------
const registered = new Set(listRoutes())
if (!registered.has('GET /api/v1/t/:slug/providers'))
  route('GET', '/api/v1/t/:slug/providers', ({ store, params }) => {
    const t = tenantOf(store, params.slug)
    if (!t) return notFound('tenant')
    return { json: { data: t.providers, meta: { next_cursor: null, has_more: false } } }
  })
if (!registered.has('GET /api/v1/t/:slug/tests'))
  route('GET', '/api/v1/t/:slug/tests', ({ store, params, query }) => {
    const t = tenantOf(store, params.slug)
    if (!t) return notFound('tenant')
    const lq = parseListQuery(query, 'name')
    const status = multi(query, 'status')
    const kind = multi(query, 'kind')
    const items = t.tests
      .filter((x) => {
        if (status.length && !status.includes(x.status)) return false
        if (kind.length && !kind.includes(x.summary?.db_kind ?? '')) return false
        return matchesSearch(
          `${x.name} ${x.description ?? ''} ${x.summary?.db_kind ?? ''}`,
          lq.search
        )
      })
      .map((x) => ({ ...x, is_favorite: t.favorites.has(`test:${x.id}`) }))
    const sorted =
      lq.sort === 'created_at'
        ? sortBy(items, (x) => x.created_at, lq.order)
        : lq.sort === 'updated_at'
          ? sortBy(items, (x) => x.updated_at, lq.order)
          : lq.sort === 'kind'
            ? sortBy(items, (x) => x.summary?.db_kind ?? '', lq.order)
            : sortBy(items, (x) => x.name.toLowerCase(), lq.order)
    return { json: paginate(sorted, lq) }
  })
