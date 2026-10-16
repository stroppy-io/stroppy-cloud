import type { Run, RunPhase } from '@api/types'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, Text, Tooltip, useStyles2 } from '@grafana/ui'
import { formatDuration } from '@helpers/format'
import { barPct, phasePct, stageText } from '@helpers/run-progress'
import { PHASE_ORDER, phaseIcon } from '@helpers/run-status'
import { useNow } from '@hooks/useNow'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5), minWidth: 0 }),
  bar: css({
    height: 6,
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.background.secondary,
    overflow: 'hidden',
    position: 'relative',
  }),
  fill: css({
    height: '100%',
    background: theme.colors.info.main,
    transition: 'width 0.6s ease',
  }),
  fillDone: css({ background: theme.colors.success.main }),
  fillFailed: css({ background: theme.colors.error.main }),
  fillCancelling: css({ background: theme.colors.warning.main }),
  fillCancelled: css({ background: theme.colors.text.disabled }),
  // One equal column per phase, the same stretches the bar fills.
  phases: css({
    display: 'grid',
    gridAutoFlow: 'column',
    gridAutoColumns: 'minmax(0, 1fr)',
    gap: theme.spacing(0.5),
    color: theme.colors.text.disabled,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  phase: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: 4,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
  }),
  phaseDone: css({ color: theme.colors.text.secondary }),
  phaseCurrent: css({
    color: theme.colors.text.primary,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  row: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1), flexWrap: 'wrap' }),
})

// Phase strip + progress bar shown in the run header: the bar follows the phase labels
// (`barPct`), the percent is the current phase's (as the run list says it), cancelling has its
// own label and colour, a workload past its plan says by how much.
export function RunProgress({ run, compact }: { run: Run; compact?: boolean }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const phases = PHASE_ORDER.filter((p) => p !== 'queued' && p !== 'done')
  const current = (phases as RunPhase[]).indexOf(run.phase)
  const live = run.status === 'running' || run.status === 'cancelling'
  const now = useNow(1000, live && run.phase === 'workload')
  const pct = Math.round(barPct(run, phases, now))
  const eta = run.summary?.expected_finish_at
  const late = live && run.phase === 'workload' && eta ? (now - new Date(eta).getTime()) / 1000 : 0
  const preparing = live ? stageText(run, t, now) : undefined
  const fillClass =
    run.status === 'completed'
      ? styles.fillDone
      : run.status === 'failed'
        ? styles.fillFailed
        : run.status === 'cancelling'
          ? styles.fillCancelling
          : run.status === 'cancelled'
            ? styles.fillCancelled
            : undefined
  return (
    <div className={styles.root}>
      <div className={styles.row}>
        {run.status === 'cancelling' && (
          <Text variant="bodySmall" color="warning">
            {t('common.status.cancelling')}
          </Text>
        )}
        {preparing ? (
          <Text color="secondary" variant="bodySmall">
            {preparing}
          </Text>
        ) : (
          <>
            <Text color="secondary" variant="bodySmall">
              {t(`common.phase.${run.phase}`)}
              {run.summary?.segment ? ` · ${t('runs.segment', { name: run.summary.segment })}` : ''}
            </Text>
            <Text color="secondary" variant="bodySmall" tabular>
              {t('runs.progress', { pct: Math.round(phasePct(run, now)) })}
            </Text>
          </>
        )}
        {late > 60 && (
          <Text variant="bodySmall" color="warning">
            {t('runs.list.overdue', { time: formatDuration(late) })}
          </Text>
        )}
      </div>
      <div
        className={styles.bar}
        role="progressbar"
        aria-valuenow={pct}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div className={cx(styles.fill, fillClass)} style={{ width: `${pct}%` }} />
      </div>
      {!compact && (
        <div className={styles.phases}>
          {phases.map((p, i) => (
            <Tooltip key={p} content={t(`common.phase.${p}`)}>
              <span
                className={cx(
                  styles.phase,
                  i < current && styles.phaseDone,
                  i === current && styles.phaseCurrent
                )}
              >
                <Icon name={phaseIcon(p)} size="sm" />
                <span>{t(`common.phase.${p}`)}</span>
              </span>
            </Tooltip>
          ))}
        </div>
      )}
    </div>
  )
}
