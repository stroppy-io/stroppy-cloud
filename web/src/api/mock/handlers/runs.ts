import type { Comparison, LaunchOverrides, Run, RunMetrics, Share } from '@api/types'
import { noContent, notFound, problem, route } from '../router'
import { launchFromTest } from '../run-factory'
import { getSimulation } from '../simulation'
import type { Test } from '../store'
import { facet, iso, matchesSearch, multi, paginate, parseListQuery, sortBy, uuid } from '../util'

function durationSec(r: Run): number {
  if (!r.started_at) return 0
  const end = r.finished_at ? new Date(r.finished_at).getTime() : Date.now()
  return (end - new Date(r.started_at).getTime()) / 1000
}

function filterRuns(runs: Run[], q: URLSearchParams, favorites: Set<string>): Run[] {
  const status = multi(q, 'status')
  const kind = multi(q, 'kind')
  const provider = multi(q, 'provider_profile')
  const trigger = multi(q, 'trigger')
  const testId = q.get('test_id')
  const suiteRunId = q.get('suite_run_id')
  const author = q.get('author')
  const labels = q.get('labels')
  const search = q.get('search')
  const favOnly = q.get('favorites') === 'true'
  const kept = q.get('stand_kept')
  const startedAfter = q.get('started_after')
  const startedBefore = q.get('started_before')
  return runs.filter((r) => {
    if (status.length && !status.includes(r.status)) return false
    if (kind.length && !kind.includes(r.summary?.db_kind ?? '')) return false
    if (provider.length && !provider.includes(r.snapshot.provider_profile.id)) return false
    if (trigger.length && !trigger.includes(r.trigger)) return false
    if (testId && r.test_ref.id !== testId) return false
    if (suiteRunId && r.trigger_ref?.suite_run_id !== suiteRunId) return false
    if (q.get('standalone') === 'true' && r.trigger_ref?.suite_run_id) return false
    if (author && r.author.id !== author && r.author.display_name !== author) return false
    if (favOnly && !favorites.has(`run:${r.id}`)) return false
    if (kept === 'true' && !r.stand_kept) return false
    if (startedAfter && (r.started_at ?? '') < startedAfter) return false
    if (startedBefore && (r.started_at ?? '') > startedBefore) return false
    if (labels) {
      for (const pair of labels.split(',')) {
        const [k, v] = pair.split('=')
        if (!r.labels || !(k in r.labels) || (v !== undefined && r.labels[k] !== v)) return false
      }
    }
    if (
      !matchesSearch(
        `${r.name} ${r.summary?.db_kind ?? ''} ${r.summary?.workload_name ?? ''} ${r.test_ref.name ?? ''} ${r.author.display_name ?? ''}`,
        search ?? undefined
      )
    )
      return false
    return true
  })
}

function sortRuns(runs: Run[], sort: string | undefined, order: 'asc' | 'desc'): Run[] {
  switch (sort) {
    case 'started_at':
      return sortBy(runs, (r) => r.started_at ?? r.created_at, order)
    case 'finished_at':
      return sortBy(runs, (r) => r.finished_at ?? '', order)
    case 'duration':
      return sortBy(runs, durationSec, order)
    case 'tps':
      return sortBy(runs, (r) => r.summary?.headline?.tps ?? -1, order)
    case 'status':
      return sortBy(runs, (r) => r.status, order)
    case 'name':
      return sortBy(runs, (r) => r.name.toLowerCase(), order)
    default:
      return sortBy(runs, (r) => r.created_at, order)
  }
}

route('GET', '/api/v1/t/:slug/runs', ({ store, params, query }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const lq = parseListQuery(query, 'created_at')
  const items = sortRuns(filterRuns(t.runs, query, t.favorites), lq.sort, lq.order).map((r) => ({
    ...r,
    is_favorite: t.favorites.has(`run:${r.id}`),
  }))
  return { json: paginate(items, lq) }
})

route('GET', '/api/v1/t/:slug/runs:facets', ({ store, params, query }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const runs = filterRuns(t.runs, query, t.favorites)
  return {
    json: {
      data: [
        facet(
          'status',
          runs.map((r) => r.status)
        ),
        facet(
          'kind',
          runs.map((r) => r.summary?.db_kind)
        ),
        facet(
          'provider_profile',
          runs.map((r) => r.snapshot.provider_profile.id)
        ),
        facet(
          'trigger',
          runs.map((r) => r.trigger)
        ),
        facet(
          'author',
          runs.map((r) => r.author.display_name)
        ),
        facet(
          'labels',
          runs.flatMap((r) => Object.entries(r.labels ?? {}).map(([k, v]) => `${k}=${v}`))
        ),
      ],
    },
  }
})

function getRun(
  store: Parameters<Parameters<typeof route>[2]>[0]['store'],
  slug: string,
  id: string
) {
  const t = store.tenant(slug)
  const run = t?.runs.find((r) => r.id === id)
  return { t, run, live: t?.runLive[id] }
}

route('GET', '/api/v1/t/:slug/runs/:id', ({ store, params }) => {
  const { t, run } = getRun(store, params.slug, params.id)
  if (!t || !run) return notFound('run')
  return {
    json: {
      ...run,
      is_favorite: t.favorites.has(`run:${run.id}`),
      shares: t.shares
        .filter((s) => s.target.kind === 'run' && s.target.id === run.id && s.active)
        .map((s) => ({ id: s.id, name: s.title })),
    },
  }
})

route('PATCH', '/api/v1/t/:slug/runs/:id', ({ store, params, body }) => {
  const { t, run } = getRun(store, params.slug, params.id)
  if (!t || !run) return notFound('run')
  const b = body as {
    name?: string
    notes?: string
    labels?: Record<string, string>
    rating?: Run['rating']
  }
  if (b.name !== undefined) {
    if (!b.name.trim())
      return problem(400, 'validation_failed', 'name required', {
        validation: {
          errors: [
            { path: 'name', code: 'REQUIRED', severity: 'ERROR', message: 'Name cannot be empty' },
          ],
        },
      })
    run.name = b.name.trim()
  }
  if (b.notes !== undefined) run.notes = b.notes
  if (b.labels !== undefined) run.labels = b.labels
  if (b.rating !== undefined) run.rating = { ...run.rating, ...b.rating }
  return { json: run }
})

route('DELETE', '/api/v1/t/:slug/runs/:id', ({ store, params }) => {
  const { t, run } = getRun(store, params.slug, params.id)
  if (!t || !run) return notFound('run')
  if (run.status === 'running' || run.status === 'pending' || run.status === 'cancelling')
    return problem(409, 'run_not_terminal', 'Cancel the run before deleting it.')
  t.runs = t.runs.filter((r) => r.id !== run.id)
  delete t.runLive[run.id]
  store.audit(params.slug, 'run.delete', { kind: 'run', id: run.id, name: run.name })
  return noContent()
})

route('GET', '/api/v1/t/:slug/runs/:id/overview', ({ store, params }) => {
  const { run, live } = getRun(store, params.slug, params.id)
  if (!run || !live) return notFound('run')
  return { json: { ...live.overview, observed_at: iso() } }
})

route('GET', '/api/v1/t/:slug/runs/:id/tree', ({ store, params }) => {
  const { run, live } = getRun(store, params.slug, params.id)
  if (!run || !live) return notFound('run')
  const machines = live.overview.machines.map((m) => ({
    ref: `machine/${m.name}`,
    kind: 'Machine',
    phase: m.status,
    labels: { role: m.role, size: m.size ?? '' },
    children: [
      { ref: `agent/${m.agent_id}`, kind: 'Agent', phase: m.presence },
      ...live.overview.components
        .filter((c) => c.machine === m.name)
        .map((c) => ({
          ref: `container/${c.id}`,
          kind: 'Container',
          phase: c.status,
          labels: { image: c.image ?? '' },
        })),
    ],
  }))
  return {
    json: {
      ref: `run/${run.id}`,
      kind: 'Run',
      phase: run.status,
      labels: { pipeline: 'stroppy-run', revision: 'a1b2c3d' },
      keep_until: run.keep_until,
      children: [
        {
          ref: 'stand/main',
          kind: 'Stand',
          phase: run.stand_kept ? 'kept' : run.status,
          children: machines,
        },
        {
          ref: 'artifacts',
          kind: 'ArtifactStore',
          children: (run.result?.artifacts ?? []).map((a) => ({
            ref: `artifact/${a}`,
            kind: 'Artifact',
          })),
        },
      ],
    },
  }
})

route('GET', '/api/v1/t/:slug/runs/:id/events', ({ store, params, query }) => {
  const { run, live } = getRun(store, params.slug, params.id)
  if (!run || !live) return notFound('run')
  const after = query.get('after')
  const limit = Number(query.get('limit') ?? 200)
  let items = live.events
  if (after) {
    const idx = items.findIndex((e) => e.id === after)
    items = idx >= 0 ? items.slice(idx + 1) : items
  }
  return {
    json: {
      data: items.slice(0, limit),
      meta: {
        next_cursor: items.length > limit ? items[limit - 1].id : null,
        has_more: items.length > limit,
      },
    },
  }
})

route('GET', '/api/v1/t/:slug/runs/:id/logs', ({ store, params, query }) => {
  const { run, live } = getRun(store, params.slug, params.id)
  if (!run || !live) return notFound('run')
  const role = multi(query, 'role')
  const machine = multi(query, 'machine')
  const container = multi(query, 'container')
  const stream = multi(query, 'stream')
  const phase = multi(query, 'phase')
  const level = multi(query, 'level')
  const segment = query.get('segment')
  const q = query.get('q')
  const start = query.get('start')
  const end = query.get('end')
  const limit = Math.min(1000, Number(query.get('limit') ?? 200))
  const cursor = query.get('cursor')
  const direction = query.get('direction') ?? 'older'
  let lines = live.logs.filter(
    (l) =>
      (!role.length || role.includes(l.role ?? '')) &&
      (!machine.length || machine.includes(l.machine ?? '')) &&
      (!container.length || container.includes(l.container ?? '')) &&
      (!stream.length || stream.includes(l.stream ?? '')) &&
      (!phase.length || phase.includes(l.phase ?? '')) &&
      (!level.length || level.includes(l.level ?? '')) &&
      (!segment || l.segment === segment) &&
      (!q || l.message.toLowerCase().includes(q.toLowerCase())) &&
      (!start || l.time >= start) &&
      (!end || l.time <= end)
  )
  if (cursor) {
    const c = Number(cursor)
    lines =
      direction === 'newer'
        ? lines.filter((l) => (l.seq ?? 0) > c)
        : lines.filter((l) => (l.seq ?? 0) < c)
  }
  const page = direction === 'newer' ? lines.slice(0, limit) : lines.slice(-limit)
  return {
    json: {
      data: page,
      older: page.length && lines.length > limit ? String(page[0].seq) : null,
      newer: page.length ? String(page[page.length - 1].seq) : null,
      truncated: lines.length > limit,
    },
  }
})

route('GET', '/api/v1/t/:slug/runs/:id/logs:facets', ({ store, params }) => {
  const { run, live } = getRun(store, params.slug, params.id)
  if (!run || !live) return notFound('run')
  return {
    json: {
      data: [
        facet(
          'role',
          live.logs.map((l) => l.role)
        ),
        facet(
          'machine',
          live.logs.map((l) => l.machine)
        ),
        facet(
          'container',
          live.logs.map((l) => l.container)
        ),
        facet(
          'stream',
          live.logs.map((l) => l.stream)
        ),
        facet(
          'phase',
          live.logs.map((l) => l.phase)
        ),
        facet(
          'level',
          live.logs.map((l) => l.level)
        ),
      ],
    },
  }
})

route('POST', '/api/v1/t/:slug/runs/:id/logs:raw', ({ store, params, body }) => {
  const { run, live } = getRun(store, params.slug, params.id)
  if (!run || !live) return notFound('run')
  const b = body as { query: string; limit?: number }
  if (!b?.query?.trim())
    return problem(400, 'validation_failed', 'query required', {
      validation: {
        errors: [
          {
            path: 'query',
            code: 'REQUIRED',
            severity: 'ERROR',
            message: 'Enter a LogsQL query, e.g. `_msg:ERROR`',
          },
        ],
      },
    })
  const m = b.query.match(/_msg:"?([^"\s]+)"?/)
  const needle = m?.[1]?.toLowerCase()
  const data = live.logs
    .filter((l) => !needle || l.message.toLowerCase().includes(needle))
    .slice(-(b.limit ?? 200))
  return { json: { data, older: null, newer: null, truncated: false } }
})

function metricsFor(
  live: NonNullable<ReturnType<typeof getRun>['live']>,
  query: URLSearchParams,
  store: Parameters<Parameters<typeof route>[2]>[0]['store']
): RunMetrics {
  const keys = multi(query, 'keys')
  const start = query.get('start')
  const end = query.get('end')
  const startMs = start ? new Date(start).getTime() : undefined
  const endMs = end ? new Date(end).getTime() : undefined
  const series = Object.entries(live.metrics)
    .filter(([k]) => !keys.length || keys.includes(k))
    .map(([key, pts]) => {
      const points = pts.filter(
        ([t]) => (startMs === undefined || t >= startMs) && (endMs === undefined || t <= endMs)
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
  const all = series.flatMap((s) => s.points.map(([t]) => t))
  return {
    window: {
      start: iso(all.length ? Math.min(...all) : Date.now() - 60_000),
      end: iso(all.length ? Math.max(...all) : Date.now()),
      segment: query.get('segment') ?? live.overview.workload_segments?.[0]?.name,
    },
    series,
  }
}

route('GET', '/api/v1/t/:slug/runs/:id/metrics', ({ store, params, query }) => {
  const { run, live } = getRun(store, params.slug, params.id)
  if (!run || !live) return notFound('run')
  return { json: metricsFor(live, query, store) }
})

route('POST', '/api/v1/t/:slug/runs/:id/metrics:raw', ({ store, params, body }) => {
  const { run, live } = getRun(store, params.slug, params.id)
  if (!run || !live) return notFound('run')
  const b = body as { query: string; start: string; end: string }
  if (!b?.query?.trim())
    return problem(400, 'validation_failed', 'query required', {
      validation: {
        errors: [
          {
            path: 'query',
            code: 'REQUIRED',
            severity: 'ERROR',
            message: 'Enter a PromQL expression, e.g. `rate(pg_stat_database_xact_commit[1m])`',
          },
        ],
      },
    })
  const key = Object.keys(live.metrics).find((k) => b.query.includes(k)) ?? 'tps'
  const q = new URLSearchParams({ keys: key, start: b.start, end: b.end })
  return { json: metricsFor(live, q, store) }
})

route('POST', '/api/v1/t/:slug/runs/:id/grafana-session', ({ store, params }) => {
  const { run } = getRun(store, params.slug, params.id)
  if (!run) return notFound('run')
  const from = run.started_at ? new Date(run.started_at).getTime() : Date.now() - 3600_000
  const to = run.finished_at ? new Date(run.finished_at).getTime() : Date.now()
  const qs = `?orgId=1&from=${from}&to=${to}&var-run=${run.id}&kiosk`
  return {
    json: {
      expires_at: iso(Date.now() + 3600_000),
      dashboards: [
        { id: 'stroppy-run', title: 'Run overview', url: `/grafana/d/stroppy-run/run${qs}` },
        {
          id: 'db',
          title: `${run.summary?.db_kind ?? 'db'} internals`,
          url: `/grafana/d/stroppy-${run.summary?.db_kind}/db${qs}`,
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

route('GET', '/api/v1/t/:slug/runs/:id/artifacts', ({ store, params }) => {
  const { run } = getRun(store, params.slug, params.id)
  if (!run) return notFound('run')
  const items = (run.result?.artifacts ?? []).map((id) =>
    id === 'art-raw'
      ? {
          id,
          name: 'stroppy-raw.jsonl.gz',
          kind: 'stroppy_raw',
          content_type: 'application/gzip',
          size_bytes: 18_422_133,
          digest: 'sha256:9f2c…',
          created_at: run.finished_at ?? undefined,
        }
      : id === 'art-report'
        ? {
            id,
            name: 'report.html',
            kind: 'report',
            content_type: 'text/html',
            size_bytes: 412_991,
            created_at: run.finished_at ?? undefined,
          }
        : { id, name: id, kind: 'other', size_bytes: 0 }
  )
  if (items.length)
    items.push(
      {
        id: 'art-cfg',
        name: 'effective-configs.tar',
        kind: 'config',
        content_type: 'application/x-tar',
        size_bytes: 32_768,
        created_at: run.finished_at ?? undefined,
      },
      {
        id: 'art-logs',
        name: 'logs-bundle.tar.gz',
        kind: 'log_bundle',
        content_type: 'application/gzip',
        size_bytes: 5_120_000,
        created_at: run.finished_at ?? undefined,
      }
    )
  return { json: { data: items } }
})

route('GET', '/api/v1/t/:slug/runs/:id/artifacts/:artifactId', ({ store, params }) => {
  const { run } = getRun(store, params.slug, params.id)
  if (!run) return notFound('run')
  return {
    json: { url: `/mock/artifacts/${params.artifactId}`, expires_at: iso(Date.now() + 600_000) },
  }
})

route('GET', '/api/v1/t/:slug/runs/:id/export', ({ store, params, query }) => {
  const { run } = getRun(store, params.slug, params.id)
  if (!run) return notFound('run')
  const format = query.get('format') ?? 'json'
  if (format === 'json') return { json: run }
  const md = `# ${run.name}\n\n- status: ${run.status}\n- database: ${run.summary?.db_kind} ${run.summary?.db_version}\n- workload: ${run.summary?.workload_name}\n- tps: ${run.summary?.headline?.tps ?? '—'}\n- p99: ${run.summary?.headline?.latency_p99_ms ?? '—'} ms\n`
  return { json: { format, content: md } }
})

route('POST', '/api/v1/t/:slug/runs/:id:cancel', ({ store, params }) => {
  const { run } = getRun(store, params.slug, params.id)
  if (!run) return notFound('run')
  if (run.status !== 'running' && run.status !== 'pending')
    return problem(409, 'run_not_cancellable', `Run is ${run.status}`)
  if (run.status === 'pending') {
    run.status = 'cancelled'
    run.status_reason = 'cancelled before start'
    run.finished_at = iso()
  } else getSimulation(store).cancelRun(params.slug, run)
  store.audit(params.slug, 'run.cancel', { kind: 'run', id: run.id, name: run.name })
  return { json: run }
})

route('POST', '/api/v1/t/:slug/runs/:id:keep-extend', ({ store, params, body }) => {
  const { run } = getRun(store, params.slug, params.id)
  if (!run) return notFound('run')
  const d = (body as { duration?: string })?.duration
  const m = d?.match(/^(\d+)(h|m)$/)
  if (!m)
    return problem(400, 'validation_failed', 'duration like 2h or 30m', {
      validation: {
        errors: [
          {
            path: 'duration',
            code: 'FORMAT',
            severity: 'ERROR',
            message: 'Use a Go duration such as 30m or 2h',
          },
        ],
      },
    })
  const ms = Number(m[1]) * (m[2] === 'h' ? 3600_000 : 60_000)
  run.stand_kept = true
  run.keep_until = iso(
    Math.max(Date.now(), run.keep_until ? new Date(run.keep_until).getTime() : 0) + ms
  )
  return { json: run }
})

route('POST', '/api/v1/t/:slug/runs/:id:keep-release', ({ store, params }) => {
  const { run } = getRun(store, params.slug, params.id)
  if (!run) return notFound('run')
  run.stand_kept = false
  run.keep_until = null
  return { json: run }
})

route('POST', '/api/v1/t/:slug/runs/:id:rerun', ({ store, params, body }) => {
  const { t, run } = getRun(store, params.slug, params.id)
  if (!t || !run) return notFound('run')
  const overrides = (body ?? {}) as LaunchOverrides & { resume?: boolean }
  const test = t.tests.find((x) => x.id === run.test_ref.id)
  const snapshotTest: Test = test ?? {
    id: run.test_ref.id,
    name: run.test_ref.name ?? run.name,
    author: run.author,
    created_at: run.created_at,
    updated_at: run.created_at,
    status: 'ready',
    sizes: run.snapshot.sizes,
    provider_profile_id: run.snapshot.provider_profile.id,
    rating: run.rating,
    database: { inline: run.snapshot.database },
    workload: { inline: run.snapshot.workload },
  }
  const newRun = launchFromTest(
    store,
    params.slug,
    t,
    {
      ...snapshotTest,
      sizes: run.snapshot.sizes,
      provider_profile_id: run.snapshot.provider_profile.id,
    },
    {
      ...overrides,
      name:
        overrides.name ??
        `${run.name.replace(/ #\d+$/, '')} #${t.runs.filter((r) => r.test_ref.id === run.test_ref.id).length + 1}`,
    },
    'manual',
    { parent_run_id: run.id }
  )
  if (overrides.resume && !run.stand_kept)
    newRun.notes = `Resume requested but the stand of ${run.name} is gone — degraded to a full rerun.`
  return { status: 201, json: newRun }
})

route('POST', '/api/v1/t/:slug/runs/:id:save-as-test', ({ store, params, body }) => {
  const { t, run } = getRun(store, params.slug, params.id)
  if (!t || !run) return notFound('run')
  const b = body as { name: string; save_database_as?: string; save_workload_as?: string }
  if (!b?.name?.trim())
    return problem(400, 'validation_failed', 'name required', {
      validation: {
        errors: [
          { path: 'name', code: 'REQUIRED', severity: 'ERROR', message: 'Give the test a name' },
        ],
      },
    })
  const now = iso()
  const author = { id: store.me.id, display_name: store.me.display_name }
  let database: Test['database'] = { inline: run.snapshot.database }
  let workload: Test['workload'] = { inline: run.snapshot.workload }
  if (b.save_database_as) {
    const d = {
      id: uuid(),
      name: b.save_database_as,
      author,
      created_at: now,
      updated_at: now,
      ...run.snapshot.database,
      topology_preview: {
        label: run.summary?.topology_label ?? 'custom',
        node_count: run.summary?.node_count,
        nodes: [],
      },
    }
    t.databases.unshift(d)
    database = { ref: { id: d.id, name: d.name } }
  }
  if (b.save_workload_as) {
    const w = {
      id: uuid(),
      name: b.save_workload_as,
      author,
      created_at: now,
      updated_at: now,
      ...run.snapshot.workload,
    }
    t.workloads.unshift(w)
    workload = { ref: { id: w.id, name: w.name } }
  }
  const test: Test = {
    id: uuid(),
    name: b.name.trim(),
    author,
    created_at: now,
    updated_at: now,
    status: 'ready',
    database,
    workload,
    sizes: run.snapshot.sizes,
    provider_profile_id: run.snapshot.provider_profile.id,
    keep: run.snapshot.keep,
    rating: run.rating,
    validation: { fits: true },
  }
  t.tests.unshift(test)
  return { status: 201, json: test }
})

export function createShare(
  t: NonNullable<ReturnType<typeof getRun>['t']>,
  store: Parameters<Parameters<typeof route>[2]>[0]['store'],
  target: Share['target'],
  body: { ttl?: string; scope?: Share['scope']; title?: string }
): Share {
  const token = `shr_${Math.random().toString(36).slice(2, 8)}`
  const ttlMs = body.ttl
    ? Number(body.ttl.match(/^(\d+)/)?.[1] ?? 0) *
      (body.ttl.endsWith('d') ? 86400_000 : body.ttl.endsWith('h') ? 3600_000 : 60_000)
    : 0
  const share: Share = {
    id: uuid(),
    token,
    url: `/s/${token}`,
    target,
    scope: body.scope ?? 'overview',
    title: body.title,
    active: true,
    expires_at: ttlMs ? iso(Date.now() + ttlMs) : null,
    captured_at: iso(),
    view_count: 0,
    created_by: { id: store.me.id, display_name: store.me.display_name },
    created_at: iso(),
  }
  t.shares.unshift(share)
  return share
}

route('POST', '/api/v1/t/:slug/runs/:id:share', ({ store, params, body }) => {
  const { t, run } = getRun(store, params.slug, params.id)
  if (!t || !run) return notFound('run')
  const share = createShare(
    t,
    store,
    { kind: 'run', id: run.id, name: run.name },
    (body ?? {}) as { ttl?: string; scope?: Share['scope']; title?: string }
  )
  store.audit(params.slug, 'share.create', {
    kind: 'share',
    id: share.id,
    name: share.title ?? run.name,
  })
  return { status: 201, json: share }
})

export function buildComparison(
  t: NonNullable<ReturnType<typeof getRun>['t']>,
  store: Parameters<Parameters<typeof route>[2]>[0]['store'],
  runIds: string[],
  baseline?: string,
  deadband = 2
): Comparison | undefined {
  const runs = runIds.map((id) => t.runs.find((r) => r.id === id)).filter((r): r is Run => !!r)
  if (runs.length < 2) return undefined
  const base = runs.find((r) => r.id === baseline) ?? runs[0]
  const keys = store.catalog.metrics.filter((m) => runs.some((r) => r.result?.metrics?.[m.key]))
  const metrics = keys.map((def) => {
    const bv = base.result?.metrics?.[def.key]?.value
    return {
      key: def.key,
      title: def.title,
      unit: def.unit,
      group: def.group,
      higher_is_better: def.higher_is_better,
      cells: runs.map((r) => {
        const v = r.result?.metrics?.[def.key]?.value
        if (v === undefined) return { run_id: r.id, present: false }
        if (r.id === base.id)
          return { run_id: r.id, present: true, value: v, verdict: 'baseline' as const }
        const diff = bv ? ((v - bv) / bv) * 100 : 0
        const better = def.higher_is_better ? diff > deadband : diff < -deadband
        const worse = def.higher_is_better ? diff < -deadband : diff > deadband
        return {
          run_id: r.id,
          present: true,
          value: v,
          diff_pct: Math.round(diff * 10) / 10,
          verdict: better ? ('better' as const) : worse ? ('worse' as const) : ('same' as const),
        }
      }),
    }
  })
  const columns = runs.map((r) => ({
    run_id: r.id,
    name: r.name,
    status: r.status,
    summary: r.summary,
    started_at: r.started_at ?? undefined,
    duration: r.duration ?? undefined,
    verdict: {
      better: metrics.filter((m) => m.cells.find((c) => c.run_id === r.id)?.verdict === 'better')
        .length,
      worse: metrics.filter((m) => m.cells.find((c) => c.run_id === r.id)?.verdict === 'worse')
        .length,
      same: metrics.filter((m) => m.cells.find((c) => c.run_id === r.id)?.verdict === 'same')
        .length,
      missing: metrics.filter((m) => !m.cells.find((c) => c.run_id === r.id)?.present).length,
    },
  }))
  const spec_diff: Comparison['spec_diff'] = {}
  for (const r of runs) {
    if (r.id === base.id) continue
    const changes: { path: string; op: 'add' | 'remove' | 'replace'; a?: unknown; b?: unknown }[] =
      []
    const bp = base.snapshot.database.params
    const rp = r.snapshot.database.params
    for (const k of new Set([...Object.keys(bp), ...Object.keys(rp)]))
      if (JSON.stringify(bp[k]) !== JSON.stringify(rp[k]))
        changes.push({
          path: `database.params.${k}`,
          op: bp[k] === undefined ? 'add' : rp[k] === undefined ? 'remove' : 'replace',
          a: bp[k],
          b: rp[k],
        })
    if (base.snapshot.database.kind !== r.snapshot.database.kind)
      changes.push({
        path: 'database.kind',
        op: 'replace',
        a: base.snapshot.database.kind,
        b: r.snapshot.database.kind,
      })
    if (base.snapshot.database.version !== r.snapshot.database.version)
      changes.push({
        path: 'database.version',
        op: 'replace',
        a: base.snapshot.database.version,
        b: r.snapshot.database.version,
      })
    for (const role of new Set([
      ...Object.keys(base.snapshot.sizes),
      ...Object.keys(r.snapshot.sizes),
    ]))
      if (base.snapshot.sizes[role]?.size !== r.snapshot.sizes[role]?.size)
        changes.push({
          path: `sizes.${role}.size`,
          op: 'replace',
          a: base.snapshot.sizes[role]?.size,
          b: r.snapshot.sizes[role]?.size,
        })
    if (base.snapshot.workload_name !== r.snapshot.workload_name)
      changes.push({
        path: 'workload',
        op: 'replace',
        a: base.snapshot.workload_name,
        b: r.snapshot.workload_name,
      })
    spec_diff[r.id] = { changes }
  }
  return { baseline_run_id: base.id, columns, metrics, spec_diff }
}

route('POST', '/api/v1/t/:slug/compare', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const b = body as { run_ids: string[]; baseline_run_id?: string; deadband_pct?: number }
  if (!b?.run_ids || b.run_ids.length < 2 || b.run_ids.length > 16)
    return problem(400, 'validation_failed', 'pick 2..16 runs', {
      validation: {
        errors: [
          {
            path: 'run_ids',
            code: 'RANGE',
            severity: 'ERROR',
            message: 'Select between 2 and 16 runs',
          },
        ],
      },
    })
  const cmp = buildComparison(t, store, b.run_ids, b.baseline_run_id, b.deadband_pct)
  return cmp ? { json: cmp } : problem(404, 'not_found', 'some runs were not found')
})

route('POST', '/api/v1/t/:slug/compare:share', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const b = body as {
    run_ids: string[]
    baseline_run_id?: string
    ttl?: string
    scope?: Share['scope']
    title?: string
  }
  const share = createShare(t, store, { kind: 'comparison', id: uuid(), run_ids: b.run_ids }, b)
  return { status: 201, json: share }
})

route('PUT', '/api/v1/t/:slug/favorites/:kind/:id', ({ store, params }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  t.favorites.add(`${params.kind}:${params.id}`)
  return noContent()
})

route('DELETE', '/api/v1/t/:slug/favorites/:kind/:id', ({ store, params }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  t.favorites.delete(`${params.kind}:${params.id}`)
  return noContent()
})

route('POST', '/api/v1/t/:slug/examples/:exampleId:quick-run', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const ex = store.catalog.examples.find((e) => e.id === params.exampleId)
  if (!ex) return notFound('example')
  const template = store.tenants.main.tests.find((x) => x.status === 'ready') as Test
  const run = launchFromTest(
    store,
    params.slug,
    t,
    { ...template, id: `example:${ex.id}`, name: ex.title },
    (body ?? {}) as LaunchOverrides
  )
  return { status: 201, json: run }
})
