import type { Run } from '@api/types'
import { afterAll, beforeAll, describe, expect, it, vi } from 'vitest'
import type { MockStore } from './store'

// The mock is browser code (window timers); node needs the alias before it loads.
const g = globalThis as unknown as { window?: typeof globalThis }
g.window ??= globalThis

let store: MockStore
let main: MockStore['tenants'][string]
const LIVE = ['pending', 'running', 'cancelling']

beforeAll(async () => {
  vi.useFakeTimers()
  store = (await import('./store')).store
  await import('./handlers/index')
  main = store.tenants.main
})
afterAll(() => vi.useRealTimers())

async function list(path: string): Promise<Run[]> {
  const { match } = await import('./router')
  const url = new URL(path, 'http://mock')
  const hit = match('GET', url.pathname)
  if (!hit) throw new Error(`no route ${path}`)
  const res = await hit.route.handler({
    params: hit.params,
    query: url.searchParams,
    body: undefined,
    store,
    headers: new Headers(),
  })
  return (res as { json: { data: Run[] } }).json.data
}

describe('mock seed', () => {
  it('gives running workload runs a plan, a live headline and a sparkline', () => {
    const running = main.runs.filter((r) => r.status === 'running' && r.phase === 'workload')
    expect(running.length).toBeGreaterThanOrEqual(3)
    for (const r of running) {
      const s = r.summary
      expect(s?.workload_name).toBeTruthy()
      expect(Date.parse(s?.expected_finish_at ?? '')).toBeGreaterThan(Date.now())
      expect(s?.headline?.qps).toBeGreaterThan(0)
      expect(s?.qps_series?.length).toBeGreaterThanOrEqual(20)
      expect(s?.qps_series?.length).toBeLessThanOrEqual(30)
    }
    // At least one stays running for several minutes.
    const longest = Math.max(...running.map((r) => Date.parse(r.summary?.expected_finish_at ?? '')))
    expect(longest - Date.now()).toBeGreaterThan(10 * 60_000)
  })

  it('has a pending run and finished runs with full headlines, some with errors', () => {
    expect(main.runs.some((r) => r.status === 'pending')).toBe(true)
    const done = main.runs.filter((r) => r.status === 'completed')
    for (const r of done) {
      const h = r.summary?.headline
      expect(r.summary?.workload_name).toBeTruthy()
      expect(r.summary?.expected_finish_at).toBeUndefined()
      expect(h?.qps).toBeGreaterThan(0)
      expect(h?.latency_p50_ms).toBeGreaterThan(0)
      expect(h?.latency_p99_ms).toBeGreaterThan(0)
      expect(h?.errors).toBeGreaterThanOrEqual(0)
    }
    expect(done.some((r) => (r.summary?.headline?.errors ?? 0) > 0)).toBe(true)
  })
})

describe('mock run sorting', () => {
  it('default: favorites, then live, then newest', async () => {
    const data = await list('/api/v1/t/main/runs?limit=200')
    const fav = data.map((r) => main.favorites.has(`run:${r.id}`))
    expect(fav.indexOf(false)).toBeGreaterThan(0)
    expect(fav.slice(fav.indexOf(false))).not.toContain(true)
    const rest = data.slice(fav.indexOf(false))
    const live = rest.map((r) => LIVE.includes(r.status))
    expect(live.slice(live.indexOf(false))).not.toContain(true)
  })

  it.each(['qps', 'p50', 'errors'] as const)('%s desc, missing values last', async (sort) => {
    const pick = (r: Run) =>
      sort === 'qps'
        ? r.summary?.headline?.qps
        : sort === 'p50'
          ? r.summary?.headline?.latency_p50_ms
          : (r.summary?.headline?.errors ?? 0)
    const data = await list(`/api/v1/t/main/runs?sort=${sort}&order=desc&limit=200`)
    const values = data.map(pick)
    const present = values.filter((v) => v !== undefined) as number[]
    expect(present).toEqual([...present].sort((a, b) => b - a))
    expect(values.slice(present.length).every((v) => v === undefined)).toBe(true)
  })
})

describe('mock simulation', () => {
  it('moves live runs forward and finishes them with a full headline', async () => {
    const { getSimulation } = await import('./simulation')
    getSimulation(store)
    const tracked = main.runs.find((r) => r.id === 'run-live-1') as Run
    const before = tracked.summary?.qps_series?.at(-1)?.t ?? 0
    vi.advanceTimersByTime(30_000)
    expect(tracked.summary?.qps_series?.at(-1)?.t).toBeGreaterThan(before)
    expect(tracked.summary?.expected_finish_at).toBeTruthy()

    vi.advanceTimersByTime(15 * 60_000)
    const finished = main.runs.filter(
      (r) => ['run-live-1', 'run-live-2', 'run-live-3'].includes(r.id) && r.status === 'completed'
    )
    expect(finished.length).toBeGreaterThan(0)
    for (const r of finished) {
      expect(r.summary?.expected_finish_at).toBeUndefined()
      expect(r.summary?.headline).toMatchObject({
        qps: expect.any(Number),
        latency_p50_ms: expect.any(Number),
        latency_p99_ms: expect.any(Number),
        errors: expect.any(Number),
      })
      expect(r.summary?.qps_series?.length).toBeGreaterThan(10)
    }
    // Capacity runs (66-minute segments) are still going.
    expect(main.runs.filter((r) => r.status === 'running').length).toBeGreaterThan(0)
  })
})
