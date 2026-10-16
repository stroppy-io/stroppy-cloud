import type { Run, RunPhase } from '@api/types'
import { describe, expect, it } from 'vitest'
import { barPct, phasePct, stageText } from './run-progress'

const phases: RunPhase[] = ['provisioning', 'deploying', 'workload', 'collecting', 'teardown']
const run = (over: Partial<Run>, phasePct?: number) =>
  ({
    status: 'running',
    phase: 'workload',
    summary: { phase_progress_pct: phasePct },
    ...over,
  }) as Run

describe('barPct', () => {
  it('fills the current phase within its own stretch', () => {
    expect(barPct(run({}, 50), phases, 0)).toBe(50)
    expect(barPct(run({ phase: 'provisioning' }, 0), phases, 0)).toBe(0)
  })

  it('never reaches the next phase while the current one runs over its plan', () => {
    expect(barPct(run({}, 100), phases, 0)).toBeLessThan(60)
  })

  it('is full only for a completed run', () => {
    expect(barPct(run({ status: 'completed', phase: 'done' }), phases, 0)).toBe(100)
    expect(barPct(run({ status: 'cancelled', phase: 'teardown' }, 0), phases, 0)).toBe(80)
  })
})

describe('phasePct', () => {
  const from = Date.parse('2026-10-16T16:00:00Z')
  const planned = (over: Partial<Run['summary']>) =>
    ({
      status: 'running',
      phase: 'workload',
      summary: {
        phase_progress_pct: 7,
        workload_from: '2026-10-16T16:00:00Z',
        expected_finish_at: '2026-10-16T16:10:00Z',
        ...over,
      },
    }) as Run
  it('counts the workload between the anchors at any instant', () => {
    expect(phasePct(planned({}), from + 150_000)).toBe(25)
    expect(phasePct(planned({}), from + 151_000)).toBeCloseTo(25.17, 1)
    expect(phasePct(planned({}), from + 20 * 60_000)).toBe(100)
  })
  it("falls back to the server's percent without a plan", () => {
    expect(phasePct(planned({ expected_finish_at: undefined }), from)).toBe(7)
    expect(phasePct(run({ phase: 'deploying' }, 40), from)).toBe(40)
  })
})

describe('stageText', () => {
  const t = ((k: string, o?: Record<string, unknown>) => (o?.step ? `${k}:${o.step}` : k)) as never
  const since = '2026-10-16T13:55:32Z'
  const base = { status: 'running', phase: 'workload' } as Run
  it('says the data is being prepared, with the step and how long', () => {
    const r = {
      ...base,
      summary: { stage: { kind: 'preparing', step: 'validate_population', since } },
    } as Run
    expect(stageText(r, t, new Date(since).getTime() + 21 * 60_000)).toBe(
      'runs.stage.preparing · runs.stage.step:validate_population · 21m'
    )
  })
  it('stays silent under load or without a stage', () => {
    const r = {
      ...base,
      summary: { stage: { kind: 'load', step: 'validate_population', since } },
    } as Run
    expect(stageText(r, t, Date.now())).toBeUndefined()
    expect(stageText(base, t, Date.now())).toBeUndefined()
  })
})
