import type { LaunchOverrides, Run } from '@api/types'
import { buildLive, buildRun } from './seed'
import { getSimulation } from './simulation'
import type { MockStore, TenantData, Test } from './store'
import { uuid } from './util'

// Creates a pending run from a test (or a run snapshot), registers live state and starts the simulation.
export function launchFromTest(
  store: MockStore,
  slug: string,
  t: TenantData,
  test: Test,
  overrides: LaunchOverrides = {},
  trigger: Run['trigger'] = 'manual',
  triggerRef?: Run['trigger_ref']
): Run {
  const dbId = test.database && 'ref' in test.database ? test.database.ref.id : undefined
  const wlId = test.workload && 'ref' in test.workload ? test.workload.ref.id : undefined
  const db =
    t.databases.find((d) => d.id === dbId) ?? t.databases[0] ?? store.tenants.main.databases[0]
  const wl =
    t.workloads.find((w) => w.id === wlId) ?? t.workloads[0] ?? store.tenants.main.workloads[0]
  const providerId =
    overrides.provider_profile_id ??
    test.provider_profile_id ??
    t.providers.find((p) => p.status === 'ready')?.id
  const provider = t.providers.find((p) => p.id === providerId) ?? store.tenants.main.providers[0]
  const count = t.runs.filter((r) => r.test_ref.id === test.id).length + 1
  const testSnapshot: Test = { ...test, sizes: overrides.sizes ?? test.sizes }
  const run = buildRun({
    id: uuid(),
    name: overrides.name ?? `${test.name} #${count}`,
    test: testSnapshot,
    db,
    wl,
    status: 'pending',
    phase: 'queued',
    startedMinAgo: 0,
    durationMin: 0,
    author: { id: store.me.id, display_name: store.me.display_name },
    provider,
    trigger,
    trigger_ref: triggerRef,
    keep: overrides.keep !== undefined && overrides.keep !== '0s' && overrides.keep !== '',
    labels: overrides.labels,
  })
  run.started_at = null
  run.rating = overrides.rating ?? test.rating ?? { tenant: true, global: false }
  run.summary = { ...run.summary, progress_pct: 0 }
  t.runs.unshift(run)
  t.runLive[run.id] = buildLive(run, Date.now() % 10_000)
  t.runLive[run.id].overview.source = 'live'
  t.runLive[run.id].events = []
  t.runLive[run.id].logs = []
  t.runLive[run.id].metrics = {}
  for (const m of t.runLive[run.id].overview.machines) {
    m.status = 'pending'
    m.presence = 'offline'
  }
  for (const c of t.runLive[run.id].overview.components) c.status = 'pending'
  store.audit(slug, 'run.launch', { kind: 'run', id: run.id, name: run.name })
  // pending → running after a short queue delay
  window.setTimeout(() => getSimulation(store).startRun(slug, run), 2500)
  return run
}
