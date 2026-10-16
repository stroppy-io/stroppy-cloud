import type { Run, RunPhase } from '@api/types'

// barPct places the fill under the phase labels: every phase owns an equal stretch of the bar and
// the current one fills only its own, so a long phase never reaches into the next label.
export function barPct(run: Run, phases: RunPhase[]): number {
  if (run.status === 'completed') return 100
  const current = phases.indexOf(run.phase)
  if (current < 0) return run.phase === 'done' ? 100 : 0
  const inPhase = Math.max(0, Math.min(0.99, (run.summary?.phase_progress_pct ?? 0) / 100))
  return ((current + inPhase) / phases.length) * 100
}
