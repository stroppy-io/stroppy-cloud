import type { Run, RunPhase } from '@api/types'
import { describe, expect, it } from 'vitest'
import { barPct } from './run-progress'

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
    expect(barPct(run({}, 50), phases)).toBe(50)
    expect(barPct(run({ phase: 'provisioning' }, 0), phases)).toBe(0)
  })

  it('never reaches the next phase while the current one runs over its plan', () => {
    expect(barPct(run({}, 100), phases)).toBeLessThan(60)
  })

  it('is full only for a completed run', () => {
    expect(barPct(run({ status: 'completed', phase: 'done' }), phases)).toBe(100)
    expect(barPct(run({ status: 'cancelled', phase: 'teardown' }, 0), phases)).toBe(80)
  })
})
