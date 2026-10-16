import type { Run, RunPhase } from '@api/types'
import { formatDuration } from '@helpers/format'
import type { TFunction } from 'i18next'

// phasePct is how far the current phase is at `now`. The workload counts its planned time between
// the server's anchors (workload_from → expected_finish_at), so it moves every second instead of
// jumping on each request; other phases (and a workload without a plan) take the server's
// phase_progress_pct.
export function phasePct(run: Run, now: number): number {
  if (run.status === 'completed') return 100
  const s = run.summary
  if (run.phase === 'workload' && s?.workload_from && s.expected_finish_at) {
    const from = new Date(s.workload_from).getTime()
    const to = new Date(s.expected_finish_at).getTime()
    if (to > from) return Math.max(0, Math.min(100, ((now - from) / (to - from)) * 100))
  }
  return s?.phase_progress_pct ?? 0
}

// barPct places the fill under the phase labels: every phase owns an equal stretch of the bar and
// the current one fills only its own, so a long phase never reaches into the next label.
export function barPct(run: Run, phases: RunPhase[], now: number): number {
  if (run.status === 'completed') return 100
  const current = phases.indexOf(run.phase)
  if (current < 0) return run.phase === 'done' ? 100 : 0
  const inPhase = Math.max(0, Math.min(0.99, phasePct(run, now) / 100))
  return ((current + inPhase) / phases.length) * 100
}

// stageText says what a running segment does while stroppy prepares its data ("Preparing data ·
// step validate_population · 21m"): that time has no plan, so neither a percent nor "over plan"
// fits it. Undefined under load or when the stage is unknown.
export function stageText(run: Run, t: TFunction, now: number): string | undefined {
  const stage = run.summary?.stage
  if (stage?.kind !== 'preparing') return undefined
  const took = Math.max(0, (now - new Date(stage.since).getTime()) / 1000)
  return [
    t('runs.stage.preparing'),
    t('runs.stage.step', { step: stage.step }),
    formatDuration(took),
  ].join(' · ')
}
