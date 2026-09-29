import type { Run, RunPhase } from '@api/types'
import type { MockStore, RunLive } from './store'
import { durationStr, iso, rng } from './util'

// Drives running runs through their phases in (accelerated) real time and publishes
// topic events for the mock WebSocket. One tick per second.
const PHASES: RunPhase[] = ['provisioning', 'deploying', 'workload', 'collecting', 'teardown']
const PHASE_SECONDS: Record<string, number> = {
  provisioning: 25,
  deploying: 20,
  workload: 90,
  collecting: 12,
  teardown: 15,
}
const LOG_LINES = [
  'checkpoint complete: wrote 512 buffers (0.4%)',
  'connection received: host=10.0.3.11 port=51322',
  'stroppy: iteration ok p99=41ms',
  'stroppy: vus=64 tps=3011',
  'haproxy: backend primary status UP',
  'patroni: leader lock renewed by master-1',
  'agent: heartbeat ok load=2.31',
  'automatic analyze of table "tpcc.order_line"',
  'WARNING: long-running transaction detected (12s)',
  'ERROR: deadlock detected (retrying)',
]

type Listener = (payload: unknown, cursor?: string) => void

class Simulation {
  private listeners = new Map<string, Set<Listener>>()
  private timer: number | undefined
  private rand = rng(42)

  constructor(private store: MockStore) {}

  start(): void {
    if (this.timer !== undefined) return
    this.timer = window.setInterval(() => this.tick(), 1000)
  }

  subscribe(topic: string, cb: Listener): () => void {
    let set = this.listeners.get(topic)
    if (!set) {
      set = new Set()
      this.listeners.set(topic, set)
    }
    set.add(cb)
    return () => {
      set?.delete(cb)
    }
  }

  emit(topic: string, payload: unknown, cursor?: string): void {
    for (const cb of this.listeners.get(topic) ?? []) cb(payload, cursor)
  }

  // Called by handlers when a run is created via :launch / :rerun / quick-run.
  startRun(slug: string, run: Run): void {
    const t = this.store.tenants[slug]
    if (!t) return
    const live = t.runLive[run.id]
    if (!live) return
    run.status = 'running'
    run.phase = 'provisioning'
    run.started_at = iso()
    live.phaseIndex = 0
    live.phaseStartedAt = Date.now()
    live.overview.status = 'running'
    live.overview.phase = 'provisioning'
    live.overview.source = 'live'
    for (const p of live.overview.phases) {
      p.status = p.id === 'provisioning' ? 'running' : 'pending'
      p.started_at = p.id === 'provisioning' ? iso() : null
      p.finished_at = null
    }
    this.pushEvent(run, live, 'run.started', 'Run started')
    this.pushEvent(run, live, 'phase.started', 'Provisioning started', 'provisioning')
    this.publishRun(slug, run, live)
  }

  cancelRun(slug: string, run: Run): void {
    const t = this.store.tenants[slug]
    const live = t?.runLive[run.id]
    if (!t || !live) return
    run.status = 'cancelling'
    run.status_reason = `cancelled by ${this.store.me.display_name}`
    live.overview.status = 'cancelling'
    this.pushEvent(run, live, 'run.cancelling', 'Cancellation requested', undefined, 'cancelling')
    this.publishRun(slug, run, live)
    // finish teardown quickly
    window.setTimeout(() => this.finish(slug, run, live, 'cancelled'), 4000)
  }

  private tick(): void {
    for (const [slug, t] of Object.entries(this.store.tenants)) {
      for (const run of t.runs) {
        if (run.status !== 'running') continue
        const live = t.runLive[run.id]
        if (!live) continue
        this.advance(slug, run, live)
      }
      for (const sr of t.suiteRuns) {
        if (sr.status !== 'running') continue
        const cells = sr.cells
        let changed = false
        for (const c of cells) {
          if (c.run) {
            const r = t.runs.find((x) => x.id === c.run?.id)
            if (r && r.status !== c.status) {
              c.status = r.status
              c.summary = r.summary
              changed = true
            }
          }
        }
        const done = cells.filter((c) =>
          ['completed', 'failed', 'cancelled'].includes(c.status)
        ).length
        const failed = cells.filter((c) => c.status === 'failed').length
        const running = cells.filter((c) => c.status === 'running').length
        sr.progress = {
          total: cells.length,
          done,
          failed,
          running,
          pending: cells.length - done - running,
          pct: Math.round((done / cells.length) * 100),
        }
        if (done === cells.length) {
          sr.status = failed ? 'failed' : 'completed'
          sr.finished_at = iso()
          sr.duration = durationStr(Date.now() - new Date(sr.started_at ?? sr.created_at).getTime())
          changed = true
        }
        if (changed) this.emit(`suite_run/${sr.id}`, sr)
      }
    }
  }

  private advance(slug: string, run: Run, live: RunLive): void {
    const nowMs = Date.now()
    const phase = PHASES[live.phaseIndex]
    if (!phase) return
    const elapsed = (nowMs - live.phaseStartedAt) / 1000
    const total = PHASE_SECONDS[phase]
    const pct = Math.min(
      99,
      Math.round(((live.phaseIndex + Math.min(1, elapsed / total)) / PHASES.length) * 100)
    )
    run.summary = { ...run.summary, progress_pct: pct }
    live.overview.progress_pct = pct
    live.overview.observed_at = iso(nowMs)

    // machines/components come up during provisioning/deploying
    if (phase === 'provisioning') {
      const readyCount = Math.floor((elapsed / total) * live.overview.machines.length)
      live.overview.machines.forEach((m, i) => {
        const was = m.status
        m.status = i < readyCount ? 'ready' : 'creating'
        m.presence = i < readyCount ? 'online' : 'offline'
        if (was !== 'ready' && m.status === 'ready')
          this.pushEvent(run, live, 'machine.ready', `Machine ${m.name} ready`, m.name, 'ready')
      })
      live.overview.pending_activity = {
        activity: 'crossplane.wait_ready',
        attempt: 1,
        since: iso(live.phaseStartedAt),
      }
    } else if (phase === 'deploying') {
      live.overview.pending_activity = undefined
      const readyCount = Math.floor((elapsed / total) * live.overview.components.length)
      live.overview.components.forEach((c, i) => {
        const was = c.status
        c.status = i < readyCount ? 'ready' : 'creating'
        if (was !== 'ready' && c.status === 'ready')
          this.pushEvent(run, live, 'container.ready', `${c.id} healthy`, c.id, 'ready')
      })
    } else if (phase === 'workload') {
      const seg = live.overview.workload_segments?.[0]
      if (seg && seg.status !== 'running') {
        seg.status = 'running'
        seg.started_at = iso()
        run.summary = { ...run.summary, segment: seg.name }
        this.pushEvent(
          run,
          live,
          'segment.started',
          `Segment ${seg.name} started`,
          seg.name,
          'running'
        )
      }
      // live metrics: append a sample every tick
      const base = 2800 + 400 * Math.sin(elapsed / 9)
      const sample = (key: string, v: number) => {
        live.metrics[key] ??= []
        const arr = live.metrics[key]
        arr.push([nowMs, v])
        if (arr.length > 600) arr.shift()
      }
      sample('tps', Math.max(0, base * Math.min(1, elapsed / 10) + (this.rand() - 0.5) * 120))
      sample('latency_p99_ms', 45 + 12 * Math.abs(Math.sin(elapsed / 5)) + this.rand() * 4)
      sample('latency_p95_ms', 30 + 6 * Math.abs(Math.sin(elapsed / 6)))
      sample('latency_p50_ms', 14 + 2 * Math.sin(elapsed / 7))
      sample('errors', this.rand() < 0.05 ? 1 : 0)
      sample('db_cpu', 55 + 20 * Math.abs(Math.sin(elapsed / 11)))
      sample('db_cache_hit', 98 + this.rand())
      sample('db_io_read', 10e6 + 6e6 * Math.abs(Math.sin(elapsed / 13)))
      sample('db_io_write', 28e6 + 12e6 * Math.abs(Math.cos(elapsed / 10)))
      sample('runner_cpu', 35 + this.rand() * 8)
      this.emit(`run.metrics/${run.id}`, {
        window: { start: iso(live.phaseStartedAt), end: iso(nowMs), segment: seg?.name },
        series: Object.entries(live.metrics).map(([key, points]) => ({
          key,
          points: points.slice(-1),
        })),
      })
    }

    // log lines every tick
    const machines = live.overview.machines
    const n = 1 + Math.floor(this.rand() * 3)
    const newLines = []
    for (let i = 0; i < n; i++) {
      const m = machines[Math.floor(this.rand() * machines.length)]
      const msg = LOG_LINES[Math.floor(this.rand() * LOG_LINES.length)]
      const line = {
        time: iso(nowMs),
        seq: ++live.seq,
        message: msg,
        level: msg.startsWith('ERROR') ? 'error' : msg.startsWith('WARNING') ? 'warn' : 'info',
        stream: 'stdout',
        role: m?.role,
        machine: m?.name,
        container: m?.role === 'runner' ? 'stroppy' : m?.role,
        phase,
        segment: phase === 'workload' ? live.overview.workload_segments?.[0]?.name : undefined,
      }
      live.logs.push(line)
      newLines.push(line)
    }
    if (live.logs.length > 5000) live.logs.splice(0, live.logs.length - 5000)
    this.emit(`run.logs/${run.id}`, { data: newLines }, String(live.seq))

    if (elapsed >= total) {
      // phase finished
      const p = live.overview.phases.find((x) => x.id === phase)
      if (p) {
        p.status = 'completed'
        p.finished_at = iso(nowMs)
      }
      this.pushEvent(run, live, 'phase.finished', `${phase} finished`, phase, 'completed')
      if (phase === 'workload') {
        const seg = live.overview.workload_segments?.[0]
        if (seg) {
          seg.status = 'completed'
          seg.finished_at = iso(nowMs)
        }
        this.pushEvent(
          run,
          live,
          'segment.finished',
          `Segment ${seg?.name ?? 'workload'} finished`,
          seg?.name,
          'completed'
        )
      }
      // 12% of runs fail at deploy, for realism
      if (phase === 'deploying' && this.rand() < 0.12) {
        run.status_reason = 'container master-1/postgres failed healthcheck after 3 attempts'
        this.finish(slug, run, live, 'failed')
        return
      }
      live.phaseIndex += 1
      live.phaseStartedAt = nowMs
      const next = PHASES[live.phaseIndex]
      if (!next) {
        this.finish(slug, run, live, 'completed')
        return
      }
      run.phase = next
      live.overview.phase = next
      const np = live.overview.phases.find((x) => x.id === next)
      if (np) {
        np.status = 'running'
        np.started_at = iso(nowMs)
      }
      this.pushEvent(run, live, 'phase.started', `${next} started`, next, 'running')
    }
    this.publishRun(slug, run, live)
  }

  private finish(
    slug: string,
    run: Run,
    live: RunLive,
    status: 'completed' | 'failed' | 'cancelled'
  ): void {
    if (!this.store.tenants[slug]) return
    const nowMs = Date.now()
    run.status = status
    run.phase = status === 'completed' ? 'done' : run.phase
    run.finished_at = iso(nowMs)
    run.duration = durationStr(nowMs - new Date(run.started_at ?? run.created_at).getTime())
    run.summary = { ...run.summary, progress_pct: 100, segment: undefined }
    live.overview.status = status
    live.overview.phase = run.phase
    live.overview.progress_pct = 100
    live.overview.source = 'persisted'
    for (const p of live.overview.phases) {
      if (p.status === 'running') {
        p.status = status === 'completed' ? 'completed' : status
        p.finished_at = iso(nowMs)
      } else if (p.status === 'pending') p.status = 'skipped'
    }
    for (const m of live.overview.machines) {
      m.status = 'deleted'
      m.presence = 'terminated'
    }
    for (const c of live.overview.components) c.status = 'deleted'
    if (status === 'completed') {
      const tps = live.metrics.tps?.length
        ? live.metrics.tps.reduce((a, [, v]) => a + v, 0) / live.metrics.tps.length
        : 2900
      const p99 = live.metrics.latency_p99_ms?.length
        ? Math.max(...live.metrics.latency_p99_ms.map(([, v]) => v))
        : 50
      run.result = {
        metrics: {
          tps: { value: Math.round(tps), unit: 'tps' },
          latency_p99_ms: { value: Math.round(p99), unit: 'ms' },
          latency_p95_ms: { value: Math.round(p99 * 0.7), unit: 'ms' },
          latency_p50_ms: { value: Math.round(p99 * 0.3), unit: 'ms' },
          errors: { value: 0, unit: 'count' },
        },
        summary: {
          tps: Math.round(tps),
          latency_p99_ms: Math.round(p99),
          latency_p95_ms: Math.round(p99 * 0.7),
          latency_p50_ms: Math.round(p99 * 0.3),
          errors: 0,
          duration: run.duration ?? undefined,
        },
        segments: (live.overview.workload_segments ?? []).map((s) => ({
          name: s.name,
          status: 'completed' as const,
          exit_code: 0,
        })),
        baseline: { ok: true, verdicts: [{ check: 'noop-tier', status: 'ok' }] },
        artifacts: ['art-raw', 'art-report'],
      }
      run.summary = {
        ...run.summary,
        headline: { tps: Math.round(tps), latency_p99_ms: Math.round(p99) },
      }
    }
    this.pushEvent(
      run,
      live,
      `run.${status}`,
      `Run ${status}`,
      undefined,
      status,
      run.status_reason
    )
    this.publishRun(slug, run, live)
  }

  private pushEvent(
    run: Run,
    live: RunLive,
    kind: string,
    title: string,
    subject?: string,
    status?: string,
    error?: string
  ): void {
    const ev = {
      id: `${run.id}-e${live.events.length + 1}`,
      at: iso(),
      kind,
      title,
      subject,
      status,
      error,
    }
    live.events.push(ev)
    this.emit(`run.events/${run.id}`, { data: [ev] }, String(live.events.length))
  }

  private publishRun(slug: string, run: Run, live: RunLive): void {
    this.emit(`run/${run.id}`, run)
    this.emit(`run.overview/${run.id}`, live.overview)
    this.emit(`tenant.runs/${slug}`, { data: [run] })
  }
}

let instance: Simulation | undefined
export function getSimulation(store: MockStore): Simulation {
  if (!instance) {
    instance = new Simulation(store)
    instance.start()
  }
  return instance
}
export type { Simulation }
