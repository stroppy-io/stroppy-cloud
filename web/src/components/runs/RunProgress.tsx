import type { Run, RunPhase } from '@api/types'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, Text, Tooltip, useStyles2 } from '@grafana/ui'
import { PHASE_ORDER, phaseIcon } from '@helpers/run-status'
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
  fillCancelled: css({ background: theme.colors.text.disabled }),
  phases: css({
    display: 'flex',
    justifyContent: 'space-between',
    gap: theme.spacing(0.5),
    color: theme.colors.text.disabled,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  phase: css({ display: 'inline-flex', alignItems: 'center', gap: 4, whiteSpace: 'nowrap' }),
  phaseDone: css({ color: theme.colors.text.secondary }),
  phaseCurrent: css({
    color: theme.colors.text.primary,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  row: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1), flexWrap: 'wrap' }),
})

// Phase strip + progress bar shown in the run header; reflects `run.phase` and `summary.progress_pct`.
export function RunProgress({ run, compact }: { run: Run; compact?: boolean }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const pct =
    run.status === 'completed' ? 100 : Math.max(0, Math.min(100, run.summary?.progress_pct ?? 0))
  const phases = PHASE_ORDER.filter((p) => p !== 'queued' && p !== 'done')
  const current = (phases as RunPhase[]).indexOf(run.phase)
  const fillClass =
    run.status === 'completed'
      ? styles.fillDone
      : run.status === 'failed'
        ? styles.fillFailed
        : run.status === 'cancelled'
          ? styles.fillCancelled
          : undefined
  return (
    <div className={styles.root}>
      <div className={styles.row}>
        <Text color="secondary" variant="bodySmall">
          {t(`common.phase.${run.phase}`)}
          {run.summary?.segment ? ` · ${t('runs.segment', { name: run.summary.segment })}` : ''}
        </Text>
        <Text color="secondary" variant="bodySmall" tabular>
          {t('runs.progress', { pct })}
        </Text>
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
