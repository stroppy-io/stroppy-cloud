import type {
  Comparison,
  RatingEntry,
  Run,
  RunEvent,
  RunMetrics,
  Schemas,
  Share,
  SharedRun,
  SuiteRun,
  TenantDashboard,
} from '@api/types'
import { type HandlerResult, listRoutes, noContent, notFound, problem, route } from '../router'
import type { MockStore, TenantData } from '../store'
import { iso, multi, paginate, parseListQuery } from '../util'
import { buildComparison } from './runs'

type ShareSnapshot = Schemas['ShareSnapshot']
type TopologyPreview = Schemas['TopologyPreview']

const DAY = 86_400_000
const SECRET_RE = /pass|secret|token|key|credential|dsn/i

// ---------------------------------------------------------------------------------------------
// Rating
// ---------------------------------------------------------------------------------------------

const PERIOD_DAYS: Record<string, number> = { '7d': 7, '30d': 30, '90d': 90, '1y': 365 }

function tenantsOf(store: MockStore): TenantData[] {
  return Object.values(store.tenants)
}

function ratingEntries(
  store: MockStore,
  query: URLSearchParams,
  scope: 'tenant' | 'global',
  slug?: string
): { metric: Schemas['MetricDef']; leagues: string[]; entries: RatingEntry[] } | undefined {
  const metricKey = query.get('metric') ?? 'tps'
  const metric = store.catalog.metrics.find((m) => m.key === metricKey)
  if (!metric) return undefined
  const kinds = multi(query, 'kind')
  const versions = multi(query, 'version')
  const providers = multi(query, 'provider')
  const stroppy = multi(query, 'stroppy_version')
  const league = query.get('league')
  const period = query.get('period') ?? 'all'
  const since = PERIOD_DAYS[period] ? Date.now() - PERIOD_DAYS[period] * DAY : 0

  const tenants = scope === 'tenant' ? [store.tenant(slug ?? '')].filter(Boolean) : tenantsOf(store)
  const raw: RatingEntry[] = []
  for (const t of tenants as TenantData[]) {
    for (const r of t.runs) {
      if (r.status !== 'completed') continue
      if (scope === 'tenant' ? !r.rating?.tenant : !r.rating?.global) continue
      if (r.summary?.db_kind === 'external') continue
      const mv = r.result?.metrics?.[metric.key]?.value
      if (mv === undefined) continue
      const at = r.finished_at ?? r.started_at ?? r.created_at
      if (since && new Date(at).getTime() < since) continue
      const s = r.summary ?? {}
      if (kinds.length && !kinds.includes(s.db_kind ?? '')) continue
      if (versions.length && !versions.includes(s.db_version ?? '')) continue
      if (providers.length && !providers.includes(s.provider_kind ?? '')) continue
      if (stroppy.length && !stroppy.includes(s.stroppy_version ?? '')) continue
      const lg = s.league ?? 'M/M'
      if (league && lg !== league) continue
      const share = t.shares.find(
        (sh) => sh.active && sh.target.kind === 'run' && sh.target.id === r.id
      )
      raw.push({
        rank: 0,
        value: mv,
        unit: metric.unit,
        db_kind: s.db_kind ?? 'postgres',
        db_version: s.db_version,
        topology_label: s.topology_label,
        node_count: s.node_count,
        sizes: s.sizes,
        league: lg,
        workload_name: s.workload_name,
        script: r.snapshot.workload.segments
          .map((seg) => (seg as { workload?: { script?: string } }).workload?.script)
          .filter(Boolean)
          .join(', '),
        stroppy_version: s.stroppy_version,
        provider_kind: s.provider_kind,
        run_at: at,
        run_id: scope === 'tenant' || t.tenant.slug === slug ? r.id : undefined,
        author: scope === 'tenant' ? r.author : undefined,
        tenant_name: scope === 'global' ? (t.tenant.public_name ?? t.tenant.name) : undefined,
        share_token: share?.token,
      })
    }
  }
  const dir = metric.higher_is_better ? -1 : 1
  raw.sort((a, b) => (a.value - b.value) * dir)
  const leagues = [...new Set(raw.map((e) => e.league))].sort()
  const perLeague = new Map<string, number>()
  for (const e of raw) {
    const n = (perLeague.get(e.league) ?? 0) + 1
    perLeague.set(e.league, n)
    e.rank = n
  }
  const entries = leagues.flatMap((lg) => raw.filter((e) => e.league === lg))
  return { metric, leagues, entries }
}

function ratingResponse(
  store: MockStore,
  query: URLSearchParams,
  scope: 'tenant' | 'global',
  slug?: string
) {
  const r = ratingEntries(store, query, scope, slug)
  if (!r)
    return problem(400, 'unknown_metric', `metric ${query.get('metric')} is not in the catalog`)
  const page = paginate(r.entries, parseListQuery(query))
  return { json: { metric: r.metric, leagues: r.leagues, data: page.data, meta: page.meta } }
}

route('GET', '/api/v1/t/:slug/rating', ({ store, params, query }) => {
  if (!store.tenant(params.slug)) return notFound('tenant')
  return ratingResponse(store, query, 'tenant', params.slug)
})

route('GET', '/api/v1/public/rating', ({ store, query }) => {
  if (!store.system.public_rating_enabled)
    return problem(404, 'rating_disabled', 'public rating is disabled on this installation')
  return ratingResponse(store, query, 'global')
})

// ---------------------------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------------------------

route('GET', '/api/v1/t/:slug/dashboard', ({ store, params }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const week = Date.now() - 7 * DAY
  const inWeek = (r: Run) => new Date(r.finished_at ?? r.created_at).getTime() >= week
  const runs = t.runs
  const completed7 = runs.filter((r) => r.status === 'completed' && inWeek(r)).length
  const failed7 = runs.filter((r) => r.status === 'failed' && inWeek(r)).length
  const active = runs
    .filter((r) => r.status === 'running' || r.status === 'pending' || r.status === 'cancelling')
    .sort((a, b) => (b.started_at ?? b.created_at).localeCompare(a.started_at ?? a.created_at))
  const recent = runs
    .filter((r) => r.status === 'completed' || r.status === 'failed' || r.status === 'cancelled')
    .sort((a, b) => (b.finished_at ?? '').localeCompare(a.finished_at ?? ''))
    .slice(0, 8)
  const top = ratingEntries(store, new URLSearchParams({ metric: 'tps' }), 'tenant', params.slug)
  const dashboard: TenantDashboard = {
    run_counts: {
      total: runs.length,
      pending: runs.filter((r) => r.status === 'pending').length,
      running: runs.filter((r) => r.status === 'running' || r.status === 'cancelling').length,
      completed: completed7,
      failed: failed7,
      cancelled: runs.filter((r) => r.status === 'cancelled' && inWeek(r)).length,
      kept_stands: runs.filter((r) => r.stand_kept).length,
    },
    success_rate:
      completed7 + failed7 > 0
        ? Math.round((completed7 / (completed7 + failed7)) * 1000) / 10
        : undefined,
    recent_runs: [...active, ...recent],
    recent_suite_runs: [...t.suiteRuns]
      .sort((a, b) => b.created_at.localeCompare(a.created_at))
      .slice(0, 5),
    upcoming: t.schedules
      .filter((s) => s.enabled && s.next_run_at)
      .sort((a, b) => (a.next_run_at ?? '').localeCompare(b.next_run_at ?? ''))
      .slice(0, 5),
    top_results: top?.entries.slice(0, 5) ?? [],
    providers: t.providers,
    limits: t.limits,
  }
  return { json: dashboard }
})

// ---------------------------------------------------------------------------------------------
// Providers (read-only, only when no other handler module registered them)
// ---------------------------------------------------------------------------------------------

if (!listRoutes().includes('GET /api/v1/t/:slug/providers')) {
  route('GET', '/api/v1/t/:slug/providers', ({ store, params }) => {
    const t = store.tenant(params.slug)
    if (!t) return notFound('tenant')
    return { json: { data: t.providers } }
  })
}
if (!listRoutes().includes('GET /api/v1/t/:slug/providers/:id/quotas')) {
  route('GET', '/api/v1/t/:slug/providers/:id/quotas', ({ store, params }) => {
    const t = store.tenant(params.slug)
    if (!t) return notFound('tenant')
    const p = t.providers.find((x) => x.id === params.id)
    if (!p) return notFound('provider')
    return {
      json: t.quotas[p.id] ?? {
        observed_at: null,
        stale: true,
        unavailable_reason: 'quotas were never fetched for this profile',
        quotas: [],
      },
    }
  })
}

// ---------------------------------------------------------------------------------------------
// Shares (tenant side)
// ---------------------------------------------------------------------------------------------

function ttlToMs(ttl: string | undefined): number {
  if (!ttl) return 0
  const m = ttl.match(/^(\d+)(d|h|m)$/)
  if (!m) return 0
  const n = Number(m[1])
  return n * (m[2] === 'd' ? DAY : m[2] === 'h' ? 3_600_000 : 60_000)
}

function isExpired(s: Share): boolean {
  return !!s.expires_at && new Date(s.expires_at).getTime() < Date.now()
}

route('GET', '/api/v1/t/:slug/shares', ({ store, params, query }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const kind = query.get('target_kind')
  const active = query.get('active')
  const targetId = query.get('target_id')
  let items = [...t.shares].sort((a, b) => b.created_at.localeCompare(a.created_at))
  if (kind) items = items.filter((s) => s.target.kind === kind)
  if (targetId) items = items.filter((s) => s.target.id === targetId)
  if (active === 'true') items = items.filter((s) => s.active && !isExpired(s))
  if (active === 'false') items = items.filter((s) => !s.active || isExpired(s))
  return { json: paginate(items, parseListQuery(query)) }
})

route('GET', '/api/v1/t/:slug/shares/:id', ({ store, params }) => {
  const s = store.tenant(params.slug)?.shares.find((x) => x.id === params.id)
  return s ? { json: s } : notFound('share')
})

route('PATCH', '/api/v1/t/:slug/shares/:id', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  const s = t?.shares.find((x) => x.id === params.id)
  if (!t || !s) return notFound('share')
  const b = (body ?? {}) as { ttl?: string | null; scope?: Share['scope']; title?: string | null }
  if (b.ttl !== undefined) {
    if (b.ttl === null || b.ttl === '' || b.ttl === 'never') s.expires_at = null
    else {
      const ms = ttlToMs(b.ttl)
      if (!ms)
        return problem(400, 'validation_failed', 'bad ttl', {
          validation: {
            errors: [
              {
                path: 'ttl',
                code: 'PATTERN',
                severity: 'ERROR',
                message: 'Use a duration like 7d, 12h or 30m',
              },
            ],
          },
        })
      s.expires_at = iso(Date.now() + ms)
    }
  }
  if (b.scope) s.scope = b.scope
  if (b.title !== undefined) s.title = b.title?.trim() || undefined
  store.audit(params.slug, 'share.update', { kind: 'share', id: s.id, name: s.title })
  return { json: s }
})

route('DELETE', '/api/v1/t/:slug/shares/:id', ({ store, params }) => {
  const t = store.tenant(params.slug)
  const s = t?.shares.find((x) => x.id === params.id)
  if (!t || !s) return notFound('share')
  s.active = false
  s.revoked_at = iso()
  store.audit(params.slug, 'share.revoke', { kind: 'share', id: s.id, name: s.title })
  return noContent()
})

route('POST', '/api/v1/t/:slug/shares/:id:rebuild', ({ store, params }) => {
  const t = store.tenant(params.slug)
  const s = t?.shares.find((x) => x.id === params.id)
  if (!t || !s) return notFound('share')
  if (!s.active) return problem(409, 'share_revoked', 'a revoked share cannot be rebuilt')
  s.captured_at = iso()
  store.audit(params.slug, 'share.rebuild', { kind: 'share', id: s.id, name: s.title })
  return { json: s }
})

// ---------------------------------------------------------------------------------------------
// Public share snapshot
// ---------------------------------------------------------------------------------------------

function findShare(store: MockStore, token: string): { t: TenantData; share: Share } | undefined {
  for (const t of tenantsOf(store)) {
    const share = t.shares.find((s) => s.token === token)
    if (share) return { t, share }
  }
  return undefined
}

function shareGone(): ReturnType<typeof problem> {
  return problem(404, 'share_gone', 'this shared report was revoked or has expired')
}

function maskSecrets(v: Record<string, unknown> | undefined): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const [k, val] of Object.entries(v ?? {})) {
    if (SECRET_RE.test(k) && typeof val === 'string') out[k] = '••••••'
    else if (val && typeof val === 'object' && !Array.isArray(val))
      out[k] = maskSecrets(val as Record<string, unknown>)
    else out[k] = val
  }
  return out
}

function renderConfig(values: Record<string, unknown>): string {
  return Object.entries(maskSecrets(values))
    .map(([k, v]) => `${k} = ${typeof v === 'string' ? v : JSON.stringify(v)}`)
    .join('\n')
}

function configsOf(run: Run): SharedRun['configs'] {
  const out: NonNullable<SharedRun['configs']> = {}
  const eff = run.snapshot.effective_configs
  if (eff) {
    for (const [role, files] of Object.entries(eff)) {
      out[role] = {}
      for (const [file, baked] of Object.entries(files))
        out[role][file] = renderConfig(baked.values)
    }
  }
  const cfg = run.snapshot.database.configs
  if (cfg) {
    for (const [role, files] of Object.entries(cfg)) {
      out[role] ??= {}
      for (const [file, values] of Object.entries(files))
        out[role][file] ??= renderConfig(values as Record<string, unknown>)
    }
  }
  return Object.keys(out).length ? out : undefined
}

function seriesFromLive(
  store: MockStore,
  live: TenantData['runLive'][string] | undefined,
  start?: string | null,
  end?: string | null
): RunMetrics | undefined {
  if (!live) return undefined
  const startMs = start ? new Date(start).getTime() : undefined
  const endMs = end ? new Date(end).getTime() : undefined
  const series = Object.entries(live.metrics).map(([key, pts]) => {
    const points = pts.filter(
      ([ts]) => (startMs === undefined || ts >= startMs) && (endMs === undefined || ts <= endMs)
    )
    const vals = points.map(([, v]) => v)
    const def = store.catalog.metrics.find((m) => m.key === key)
    const sorted = [...vals].sort((a, b) => a - b)
    return {
      key,
      title: def?.title,
      unit: def?.unit,
      role: key.startsWith('db_') ? 'db' : key.startsWith('runner_') ? 'runner' : undefined,
      points,
      aggregates: vals.length
        ? {
            avg: vals.reduce((a, b) => a + b, 0) / vals.length,
            min: sorted[0],
            max: sorted[sorted.length - 1],
            last: vals[vals.length - 1],
            p95: sorted[Math.floor(sorted.length * 0.95)],
          }
        : undefined,
    }
  })
  const all = series.flatMap((s) => s.points.map(([ts]) => ts))
  return {
    window: {
      start: iso(all.length ? Math.min(...all) : Date.now() - 60_000),
      end: iso(all.length ? Math.max(...all) : Date.now()),
      segment: live.overview.workload_segments?.[0]?.name,
    },
    series,
  }
}

function topologyOf(t: TenantData, run: Run): TopologyPreview | undefined {
  const db = t.databases.find((d) => d.name === run.snapshot.database_name)
  const preview = (db as { topology_preview?: TopologyPreview } | undefined)?.topology_preview
  if (preview) return preview
  const machines = run.snapshot.machines ?? []
  if (!machines.length) return undefined
  const byRole = new Map<string, number>()
  for (const m of machines) byRole.set(m.role, (byRole.get(m.role) ?? 0) + 1)
  return {
    label: run.summary?.topology_label ?? 'custom',
    node_count: machines.length,
    nodes: [...byRole].map(([role, count]) => ({ role, count })),
  }
}

function timelineOf(live: TenantData['runLive'][string] | undefined, run: Run): RunEvent[] {
  if (live?.events.length) return live.events
  const out: RunEvent[] = []
  if (run.started_at)
    out.push({ id: `${run.id}-s`, at: run.started_at, kind: 'run.started', title: 'Run started' })
  if (run.finished_at)
    out.push({
      id: `${run.id}-f`,
      at: run.finished_at,
      kind: `run.${run.status}`,
      title: `Run ${run.status}`,
      status: run.status,
    })
  return out
}

function sharedRun(store: MockStore, t: TenantData, run: Run, scope: Share['scope']): SharedRun {
  const live = t.runLive[run.id]
  const withMetrics = scope === 'metrics' || scope === 'configs'
  const withConfigs = scope === 'configs'
  return {
    name: run.name,
    status: run.status,
    started_at: run.started_at ?? undefined,
    finished_at: run.finished_at ?? undefined,
    duration: run.duration ?? undefined,
    summary: run.summary ?? {},
    result: run.result,
    timeline: timelineOf(live, run),
    topology: topologyOf(t, run),
    database: {
      kind: run.snapshot.database.kind,
      version: run.snapshot.database.version,
      image: run.snapshot.database.image,
      params: maskSecrets(run.snapshot.database.params),
    },
    workload: {
      stroppy_version: run.snapshot.workload.stroppy_version,
      protocol: run.snapshot.workload.protocol,
      segments: run.snapshot.workload.segments,
    },
    machines: run.snapshot.machines,
    configs: withConfigs ? configsOf(run) : undefined,
    metrics: withMetrics ? seriesFromLive(store, live) : undefined,
    workload_segments: (live?.overview.workload_segments ?? run.result?.segments ?? []).map(
      (s) => ({
        name: s.name,
        started_at: s.started_at ?? undefined,
        finished_at: s.finished_at ?? undefined,
      })
    ),
  }
}

type SnapshotResult = { snap: ShareSnapshot } | { err: HandlerResult }

function snapshotOf(store: MockStore, t: TenantData, share: Share): SnapshotResult {
  const base = {
    kind: share.target.kind,
    scope: share.scope,
    title: share.title,
    captured_at: share.captured_at ?? share.created_at,
    tenant_name: t.tenant.public_name ?? t.tenant.name,
  }
  if (share.target.kind === 'run') {
    const run = t.runs.find((r) => r.id === share.target.id)
    if (!run) return { err: notFound('shared run') }
    return { snap: { ...base, run: sharedRun(store, t, run, share.scope) } }
  }
  if (share.target.kind === 'suite_run') {
    const sr: SuiteRun | undefined = t.suiteRuns.find((s) => s.id === share.target.id)
    if (!sr) return { err: notFound('shared suite run') }
    const cells = sr.cells
      .map((c) => t.runs.find((r) => r.id === c.run?.id))
      .filter((r): r is Run => !!r)
      .map((r) => sharedRun(store, t, r, 'overview'))
    const keys = ['tps', 'latency_p99_ms', 'errors']
    const summary: Schemas['SuiteRunSummary'] = {
      suite_run_id: sr.id,
      metric_keys: keys,
      rows: sr.cells.map((c) => {
        const r = t.runs.find((x) => x.id === c.run?.id)
        const metrics: NonNullable<Schemas['SuiteRunSummary']['rows'][number]['metrics']> = {}
        for (const k of keys) {
          const v = r?.result?.metrics?.[k]?.value
          if (v !== undefined) metrics[k] = { value: v }
        }
        return { cell_id: c.cell_id, name: c.name, run: c.run, status: c.status, metrics }
      }),
    }
    return { snap: { ...base, suite_run: { name: sr.name, status: sr.status, cells, summary } } }
  }
  const cmp: Comparison | undefined = buildComparison(
    t,
    store,
    share.target.run_ids ?? [],
    share.target.run_ids?.[0]
  )
  if (!cmp) return { err: notFound('shared comparison') }
  return { snap: { ...base, comparison: cmp } }
}

function resolvePublic(store: MockStore, token: string) {
  const hit = findShare(store, token)
  if (!hit) return { gone: notFound('share') }
  if (!hit.share.active || isExpired(hit.share)) return { gone: shareGone() }
  return hit
}

route('GET', '/api/v1/public/share/:token', ({ store, params }) => {
  const hit = resolvePublic(store, params.token)
  if ('gone' in hit) return hit.gone
  hit.share.view_count = (hit.share.view_count ?? 0) + 1
  const res = snapshotOf(store, hit.t, hit.share)
  if ('err' in res) return res.err
  return { json: res.snap, headers: { 'x-robots-tag': 'noindex' } }
})

route('GET', '/api/v1/public/share/:token/metrics', ({ store, params, query }) => {
  const hit = resolvePublic(store, params.token)
  if ('gone' in hit) return hit.gone
  if (hit.share.scope === 'overview')
    return problem(403, 'scope_denied', 'this share does not include metrics')
  if (hit.share.target.kind !== 'run')
    return problem(400, 'not_a_run', 'metrics are only available for run shares')
  const live = hit.t.runLive[hit.share.target.id]
  const m = seriesFromLive(store, live, query.get('start'), query.get('end'))
  return m ? { json: m } : notFound('metrics')
})

route('POST', '/api/v1/public/share/:token/grafana-session', ({ store, params }) => {
  const hit = resolvePublic(store, params.token)
  if ('gone' in hit) return hit.gone
  if (hit.share.scope === 'overview')
    return problem(403, 'scope_denied', 'this share does not include metrics')
  const run =
    hit.share.target.kind === 'run'
      ? hit.t.runs.find((r) => r.id === hit.share.target.id)
      : undefined
  const from = run?.started_at ? new Date(run.started_at).getTime() : Date.now() - 3_600_000
  const to = run?.finished_at ? new Date(run.finished_at).getTime() : Date.now()
  const qs = `?orgId=1&from=${from}&to=${to}&var-share=${hit.share.token}&kiosk`
  return {
    json: {
      expires_at: iso(Date.now() + 3_600_000),
      dashboards: [
        { id: 'stroppy-run', title: 'Run overview', url: `/grafana/d/stroppy-run/run${qs}` },
        {
          id: 'db',
          title: `${run?.summary?.db_kind ?? 'db'} internals`,
          url: `/grafana/d/stroppy-${run?.summary?.db_kind ?? 'db'}/db${qs}`,
        },
        {
          id: 'node',
          title: 'Node exporter',
          url: `/grafana/d/stroppy-node/node${qs}`,
          per_machine: true,
        },
      ],
    },
  }
})

// ---------------------------------------------------------------------------------------------
// Export
// ---------------------------------------------------------------------------------------------

function fmt(v: number | undefined, unit?: string): string {
  if (v === undefined) return '—'
  return unit ? `${Math.round(v * 100) / 100} ${unit}` : String(Math.round(v * 100) / 100)
}

function runMarkdown(r: SharedRun, title: string, captured: string, tenant?: string): string {
  const h = r.result?.summary ?? {}
  const lines = [
    `# ${title}`,
    '',
    `_Shared report · captured ${captured}${tenant ? ` · ${tenant}` : ''}_`,
    '',
    '## Summary',
    '',
    '| | |',
    '|---|---|',
    `| Status | ${r.status} |`,
    `| Started | ${r.started_at ?? '—'} |`,
    `| Duration | ${r.duration ?? '—'} |`,
    `| TPS | ${fmt(h.tps)} |`,
    `| Latency p50 / p95 / p99 | ${fmt(h.latency_p50_ms, 'ms')} / ${fmt(h.latency_p95_ms, 'ms')} / ${fmt(h.latency_p99_ms, 'ms')} |`,
    `| Errors | ${fmt(h.errors)} |`,
    '',
    '## Setup',
    '',
    `- Database: ${r.database?.kind} ${r.database?.version ?? ''}`,
    `- Topology: ${r.topology?.label ?? r.summary.topology_label ?? '—'} (${r.topology?.node_count ?? r.summary.node_count ?? '?'} nodes)`,
    `- Workload: ${r.summary.workload_name ?? '—'} · stroppy ${r.workload?.stroppy_version ?? ''} · ${r.workload?.protocol ?? ''}`,
    `- Provider: ${r.summary.provider_kind ?? '—'} · league ${r.summary.league ?? '—'}`,
    '',
    '### Database parameters',
    '',
    '```json',
    JSON.stringify(r.database?.params ?? {}, null, 2),
    '```',
    '',
    '### Machines',
    '',
    '| Name | Role | Size | vCPU | RAM, GB | Disk, GB |',
    '|---|---|---|---|---|---|',
    ...(r.machines ?? []).map(
      (m) =>
        `| ${m.name ?? ''} | ${m.role ?? ''} | ${m.size ?? ''} | ${m.cpu ?? ''} | ${m.memory_gb ?? ''} | ${m.disk_gb ?? ''} |`
    ),
    '',
    '## Results',
    '',
    '| Segment | Status | TPS | Errors |',
    '|---|---|---|---|',
    ...(r.result?.segments ?? []).map(
      (s) =>
        `| ${s.name} | ${s.status} | ${fmt(s.metrics?.tps?.value)} | ${s.errors?.failed_queries ?? 0} |`
    ),
    '',
    '## Timeline',
    '',
    ...r.timeline.map((e) => `- ${e.at} — ${e.title}${e.subject ? ` (${e.subject})` : ''}`),
    '',
  ]
  if (r.configs) {
    lines.push('## Configs', '')
    for (const [role, files] of Object.entries(r.configs))
      for (const [file, text] of Object.entries(files))
        lines.push(`### ${role} · ${file}`, '', '```', text, '```', '')
  }
  return lines.join('\n')
}

function runCsv(r: SharedRun): string {
  const h = r.result?.summary ?? {}
  const rows = [
    ['metric', 'value', 'unit'],
    ['status', r.status, ''],
    ['duration', r.duration ?? '', ''],
    ['tps', String(h.tps ?? ''), 'tps'],
    ['latency_p50_ms', String(h.latency_p50_ms ?? ''), 'ms'],
    ['latency_p95_ms', String(h.latency_p95_ms ?? ''), 'ms'],
    ['latency_p99_ms', String(h.latency_p99_ms ?? ''), 'ms'],
    ['errors', String(h.errors ?? ''), 'count'],
    ...Object.entries(r.result?.metrics ?? {}).map(([k, v]) => [k, String(v.value), v.unit ?? '']),
  ]
  return rows
    .map((row) => row.map((c) => `"${String(c).replace(/"/g, '""')}"`).join(','))
    .join('\n')
}

function comparisonMarkdown(c: Comparison, title: string): string {
  const head = `| Metric | ${c.columns.map((col) => col.name ?? col.run_id).join(' | ')} |`
  const sep = `|---|${c.columns.map(() => '---').join('|')}|`
  const rows = c.metrics.map(
    (m) =>
      `| ${m.title ?? m.key} | ${m.cells
        .map((cell) =>
          cell.present
            ? `${fmt(cell.value, m.unit)}${cell.diff_pct !== undefined ? ` (${cell.diff_pct > 0 ? '+' : ''}${cell.diff_pct}%)` : ''}`
            : '—'
        )
        .join(' | ')} |`
  )
  return [`# ${title}`, '', head, sep, ...rows, ''].join('\n')
}

function comparisonCsv(c: Comparison): string {
  const rows = [
    ['metric', 'unit', ...c.columns.map((col) => col.name ?? col.run_id)],
    ...c.metrics.map((m) => [
      m.key,
      m.unit ?? '',
      ...m.cells.map((cell) => (cell.present ? String(cell.value ?? '') : '')),
    ]),
  ]
  return rows
    .map((row) => row.map((v) => `"${String(v).replace(/"/g, '""')}"`).join(','))
    .join('\n')
}

route('GET', '/api/v1/public/share/:token/export', ({ store, params, query }) => {
  const hit = resolvePublic(store, params.token)
  if ('gone' in hit) return hit.gone
  const res = snapshotOf(store, hit.t, hit.share)
  if ('err' in res) return res.err
  const snap = res.snap
  const format = query.get('format') ?? 'json'
  const title =
    snap.title ?? snap.run?.name ?? snap.suite_run?.name ?? `Comparison ${hit.share.token}`
  if (format === 'json') return { json: snap }
  let md = ''
  let csv = ''
  if (snap.run) {
    md = runMarkdown(snap.run, title, snap.captured_at, snap.tenant_name)
    csv = runCsv(snap.run)
  } else if (snap.comparison) {
    md = comparisonMarkdown(snap.comparison, title)
    csv = comparisonCsv(snap.comparison)
  } else if (snap.suite_run) {
    const rows = snap.suite_run.summary?.rows ?? []
    md = [
      `# ${title}`,
      '',
      '| Cell | Status | TPS | p99, ms | Errors |',
      '|---|---|---|---|---|',
      ...rows.map(
        (r) =>
          `| ${r.name ?? r.cell_id} | ${r.status ?? ''} | ${fmt(r.metrics?.tps?.value)} | ${fmt(r.metrics?.latency_p99_ms?.value)} | ${fmt(r.metrics?.errors?.value)} |`
      ),
      '',
    ].join('\n')
    csv = [
      'cell,status,tps,latency_p99_ms,errors',
      ...rows.map(
        (r) =>
          `"${r.name ?? r.cell_id}","${r.status ?? ''}",${r.metrics?.tps?.value ?? ''},${r.metrics?.latency_p99_ms?.value ?? ''},${r.metrics?.errors?.value ?? ''}`
      ),
    ].join('\n')
  }
  if (format === 'csv') return { text: csv, contentType: 'text/csv' }
  if (format === 'pdf')
    return {
      text: `> PDF rendering is not available in the mock API — this is the Markdown source of the report.\n\n${md}`,
      contentType: 'text/markdown',
    }
  return { text: md, contentType: 'text/markdown' }
})
