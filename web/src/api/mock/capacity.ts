import type { buildLive, buildRun } from './seed'
import type { Database, TenantData, Test, Workload } from './store'
import { durationStr, iso, rng } from './util'

// A busy installation, not a throughput claim. Topologies and hardware stay inside the
// executable contracts: PG <=8 replicas, YDB <=64 nodes per role, <=64 machines including runner per RunSpec, XS..XL presets.
// Concurrency has no fixed system maximum; the demo's admin settings are sized for this fleet.
export function expandCapacityDemo(
  tenant: TenantData,
  makeRun: typeof buildRun,
  makeLive: typeof buildLive
): void {
  const now = Date.now()
  const rand = rng(73091)
  const owner = tenant.members[1].user
  const entity = (id: string, name: string) => ({
    id,
    name,
    author: owner,
    created_at: iso(now - 40 * 86400_000),
    updated_at: iso(now - 3600_000),
    tags: { team: 'performance', campaign: 'capacity' },
  })
  const dbs: Database[] = [
    {
      ...entity('db-capacity-pg', 'PostgreSQL 18 · Patroni · 8 replicas'),
      kind: 'postgres',
      version: '18',
      params: {
        version: '18',
        replicas: 8,
        sync_replicas: 2,
        ha: 'patroni',
        etcd_nodes: 3,
        haproxy: 2,
        pgbouncer: true,
      },
      schema: { id: 'db.postgres.params', version: '1' },
      configs: {},
      topology_preview: {
        label: 'Patroni · 1 primary + 8 replicas',
        node_count: 14,
        nodes: [
          { role: 'db', engine: 'postgres', count: 1 },
          { role: 'db_replica', engine: 'postgres', count: 8 },
          { role: 'etcd', engine: 'etcd', count: 3 },
          { role: 'proxy', engine: 'haproxy', count: 2 },
        ],
        flows: [
          { from: 'proxy', to: 'db', protocol: 'pg', port: 5432 },
          { from: 'db', to: 'db_replica', protocol: 'pg', port: 5432 },
        ],
      },
      requirements: {
        db: { cpu: 8, memory_gb: 32, disk_gb: 400 },
        runner: { cpu: 32, memory_gb: 17 },
      },
    },
    {
      ...entity('db-capacity-ydb', 'YDB 26.2 · 32 storage + 31 compute'),
      kind: 'ydb',
      version: '26.2',
      params: {
        version: '26.2',
        storage_nodes: 32,
        database_nodes: 31,
        fault_tolerance: 'block-4-2',
        failure_domain: 'body',
        pdisks_per_node: 8,
        disk_type: 'SSD',
        storage_groups: 64,
        auto_size_pdisks: true,
        database_path: '/Root/benchmark',
        haproxy: 0,
      },
      schema: { id: 'db.ydb.params', version: '1' },
      configs: {},
      topology_preview: {
        label: 'block-4-2 · 32 storage + 31 compute',
        node_count: 63,
        nodes: [
          { role: 'db', engine: 'ydb', count: 32 },
          { role: 'db_compute', engine: 'ydb', count: 31 },
        ],
        flows: [{ from: 'db_compute', to: 'db', protocol: 'ydb-ic', port: 19001 }],
      },
      requirements: {
        db: { cpu: 4, memory_gb: 8, disk_gb: 340 },
        db_compute: { cpu: 4, memory_gb: 8, disk_gb: 20 },
      },
    },
    {
      ...entity('db-capacity-mysql', 'MySQL 8.4 · primary + 8 replicas'),
      kind: 'mysql',
      version: '8.4',
      params: { version: '8.4', replicas: 8, replication: 'async', proxysql: 2 },
      schema: { id: 'db.mysql.params', version: '1' },
      configs: {},
      topology_preview: {
        label: '1 primary + 8 replicas · ProxySQL',
        node_count: 11,
        nodes: [
          { role: 'db', engine: 'mysql', count: 1 },
          { role: 'db_replica', engine: 'mysql', count: 8 },
          { role: 'proxy', engine: 'proxysql', count: 2 },
        ],
        flows: [{ from: 'proxy', to: 'db', protocol: 'mysql', port: 6033 }],
      },
      requirements: { db: { cpu: 8, memory_gb: 32, disk_gb: 400 } },
    },
  ]
  const workloads: Workload[] = dbs.map((db, i) => ({
    ...entity(
      `wl-capacity-${i}`,
      i === 1 ? 'Baseline · 100 million rows · 2048 VU' : 'TPC-C · 2000 warehouses · 2048 VU'
    ),
    stroppy_version: '6.1.0',
    protocol: i === 1 ? 'ydb_grpc' : i === 2 ? 'mysql' : 'pg',
    schema: { id: 'workload.stroppy', version: '1' },
    segments: [
      {
        name: 'sustained-load',
        workload:
          i === 1
            ? { script: 'baseline', rows: '100000000', load_workers: 128 }
            : { script: 'tpcc/tx', scale_factor: 2000, load_workers: 128 },
        run: { executor: 'constant-vus', vus: 2048, duration: '66m' },
      },
    ],
    requirements: {
      runner: {
        cpu: 32,
        memory_gb: 17,
        disk_gb: 20,
        reason: '2048 VUs / 64; 128 load workers / 4',
      },
    },
    description: `Sustained capacity measurement on ${db.name}.`,
  }))
  const tests: Test[] = dbs.map((db, i) => ({
    ...entity(
      `t-capacity-${i}`,
      `${db.kind === 'ydb' ? 'YDB baseline' : db.kind === 'mysql' ? 'MySQL TPC-C' : 'PostgreSQL TPC-C'} · capacity XL`
    ),
    status: 'ready',
    database: { ref: { id: db.id } },
    workload: { ref: { id: workloads[i].id } },
    sizes: {
      db: { size: 'XL', disk: { type: 'network-ssd', gb: 840 } },
      db_replica: { size: 'XL', disk: { type: 'network-ssd', gb: 840 } },
      db_compute: { size: 'XL', disk: { type: 'network-ssd', gb: 200 } },
      runner: { size: 'XL' },
      proxy: { size: 'L' },
      etcd: { size: 'S' },
    },
    provider_profile_id: 'prov-yc-main',
    rating: { tenant: true, global: false },
    keep: '0s',
    validation: { fits: true },
  }))
  tenant.databases.unshift(...dbs)
  tenant.workloads.unshift(...workloads)
  tenant.tests.unshift(...tests)

  // Exactly eight active rows (six executing + two queued), with 768 completed capacity runs.
  const activeCount = tenant.runs.filter(
    (r) => r.status === 'running' || r.status === 'cancelling'
  ).length
  const pendingCount = tenant.runs.filter((r) => r.status === 'pending').length
  const runningToAdd = 6 - activeCount
  const pendingToAdd = 2 - pendingCount
  const additions = 768 + runningToAdd + pendingToAdd
  for (let i = 0; i < additions; i++) {
    const dbIndex = i % dbs.length
    const isHistory = i < 768
    const status = isHistory ? 'completed' : i < 768 + runningToAdd ? 'running' : 'pending'
    const age = isHistory ? 180 + i * 48 : status === 'pending' ? 0 : 18 + (i % 24)
    const tps = Math.round([24600, 238000, 18900][dbIndex] * (0.92 + rand() * 0.16))
    const run = makeRun({
      id: `run-capacity-${i + 1}`,
      name: `${tests[dbIndex].name} #${4200 + i}`,
      test: tests[dbIndex],
      db: dbs[dbIndex],
      wl: workloads[dbIndex],
      status,
      phase: status === 'completed' ? 'done' : status === 'pending' ? 'queued' : 'workload',
      startedMinAgo: age,
      durationMin: 120,
      author: tenant.members[i % tenant.members.length].user,
      provider: tenant.providers[0],
      tps,
      p99: [38, 7, 46][dbIndex],
      errors: 0,
      trigger: 'api',
      favorite: i % 37 === 0,
      labels: {
        campaign: 'capacity',
        branch: 'release/6.1',
        batch: `batch-${Math.floor(i / 24) + 1}`,
      },
      notes: 'Dedicated XL runners · 2048 virtual users · sustained measurement after warmup.',
    })
    const qps = tps * (dbIndex === 1 ? 1 : 4)
    if (run.result?.metrics) run.result.metrics.queries_per_second = { value: qps, unit: 'qps' }
    if (run.summary) {
      run.summary.headline =
        status === 'pending'
          ? undefined
          : { ...run.summary.headline, tps, qps, latency_p99_ms: [38, 7, 46][dbIndex] }
      run.summary.progress_pct =
        status === 'completed' ? 100 : status === 'pending' ? 0 : 24 + (i % 42)
    }
    const live = makeLive(run, 9000 + i)
    const primary = live.overview.machines.find(
      (m) => m.role === (dbIndex === 1 ? 'db_compute' : 'db')
    )
    live.overview.flows = [
      {
        from: 'runner-1',
        to: primary?.name,
        protocol: workloads[dbIndex].protocol,
        port: dbIndex === 1 ? 2136 : dbIndex === 2 ? 3306 : 5432,
      },
    ]
    for (const component of live.overview.components) {
      const node = dbs[dbIndex].topology_preview?.nodes.find((n) => n.role === component.role)
      component.engine = node?.engine ?? dbs[dbIndex].kind
      if (component.engine === 'ydb')
        component.endpoints = [
          {
            name: 'grpc',
            address: live.overview.machines.find((m) => m.name === component.machine)?.address,
            port: 2136,
          },
        ]
    }
    live.metrics.queries_per_second = (live.metrics.tps ?? []).map(([t, value]) => [
      t,
      value * (dbIndex === 1 ? 1 : 4),
    ])
    if (run.summary && isHistory)
      run.summary.qps_series = live.metrics.queries_per_second
        .filter((_, j) => j % 4 === 0)
        .map(([t, v]) => ({ t, v }))
    if (status === 'pending') {
      run.started_at = null
      live.events = []
      live.logs = []
      live.metrics = {}
      live.overview.machines.forEach((m) => {
        m.status = 'pending'
        m.presence = 'offline'
      })
      live.overview.components.forEach((c) => {
        c.status = 'pending'
      })
      live.overview.phases.forEach((p) => {
        p.status = 'pending'
        p.started_at = null
        p.finished_at = null
      })
      live.overview.workload_segments?.forEach((s) => {
        s.status = 'pending'
        s.started_at = null
        s.finished_at = null
      })
    }
    // Keep historical log/event buffers compact; metric curves remain available for every run.
    if (isHistory) {
      live.logs = live.logs.slice(-24)
      live.events = live.events.slice(-16)
    }
    live.logs = live.logs.map((line) => ({
      ...line,
      level: 'info',
      message:
        line.role === 'runner'
          ? `stroppy: segment=sustained-load vus=2048 queries_per_second=${qps} tps=${tps}`
          : `agent: machine=${line.machine} phase=${line.phase} heartbeat ok`,
    }))
    tenant.runs.push(run)
    tenant.runLive[run.id] = live
    if (run.is_favorite) tenant.favorites.add(`run:${run.id}`)
  }
  tenant.limits = {
    ...tenant.limits,
    max_concurrent_runs: 16,
    max_machines_per_run: 64,
    max_size: 'XL',
  }
  tenant.tests = tenant.tests.map((test) => {
    const runs = tenant.runs
      .filter((r) => r.test_ref.id === test.id)
      .sort((a, b) => b.created_at.localeCompare(a.created_at))
    const last = runs[0]
    return {
      ...test,
      summary: {
        ...test.summary,
        run_count: runs.length,
        last_run: last
          ? { id: last.id, name: last.name, status: last.status, started_at: last.started_at }
          : undefined,
      },
    }
  })
  const machines = tenant.runs
    .filter((r) => r.status === 'running' || r.status === 'cancelling' || r.stand_kept)
    .flatMap((r) =>
      r.snapshot.provider_profile.id === 'prov-yc-main' ? (r.snapshot.machines ?? []) : []
    )
  const usage: Record<string, number> = {
    'compute.instances': machines.length,
    'compute.cores': machines.reduce((n, m) => n + (m.cpu ?? 0), 0),
    'compute.memory': machines.reduce((n, m) => n + (m.memory_gb ?? 0), 0),
    'compute.ssd': machines.reduce((n, m) => n + (m.disk_gb ?? 0), 0),
    'vpc.addresses': machines.length,
  }
  tenant.quotas['prov-yc-main'].quotas.forEach((q) => {
    q.used = usage[q.name] ?? q.used
    q.limit = Math.ceil((q.used ?? 0) / 0.78 / 100) * 100
  })
  tenant.suites.unshift({
    ...entity('suite-capacity', 'Release 6.1 · distributed capacity matrix'),
    tests: tests.map((t) => ({ ref: { id: t.id, name: t.name } })),
    axes: { provider_profiles: ['prov-yc-main'] },
    cells: tests.map((t, i) => ({
      id: `capacity-cell-${i}`,
      name: t.name,
      test: { id: t.id },
      enabled: true,
      generated: true,
    })),
    concurrency: 3,
    summary: { cell_count: 3, enabled_cell_count: 3, run_count: 0 },
  })
  // All displayed totals come from records; no detached "millions of runs" counters.
  for (const r of tenant.runs)
    if (r.finished_at && r.started_at)
      r.duration = durationStr(Date.parse(r.finished_at) - Date.parse(r.started_at))
}
