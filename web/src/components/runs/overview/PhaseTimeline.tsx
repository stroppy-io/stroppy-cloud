import type { RunOverview } from '@api/types'
import { StatusBadge } from '@components/StatusBadge'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, IconButton, Text, Tooltip, useStyles2 } from '@grafana/ui'
import { formatDuration } from '@helpers/format'
import { phaseIcon, runStatusMeta } from '@helpers/run-status'
import { formatDateTime, formatTime } from '@helpers/time'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

type Phase = RunOverview['phases'][number]

const getStyles = (theme: GrafanaTheme2) => ({
  list: css({ display: 'flex', flexDirection: 'column', margin: 0, padding: 0, listStyle: 'none' }),
  item: css({
    display: 'grid',
    gridTemplateColumns: '28px 1fr',
    columnGap: theme.spacing(1.5),
    position: 'relative',
    '&:not(:last-child)::before': {
      content: '""',
      position: 'absolute',
      left: 13,
      top: 28,
      bottom: -4,
      width: 2,
      background: theme.colors.border.weak,
    },
  }),
  dot: css({
    width: 28,
    height: 28,
    borderRadius: theme.shape.radius.circle,
    display: 'grid',
    placeItems: 'center',
    border: `2px solid ${theme.colors.border.medium}`,
    background: theme.colors.background.primary,
    color: theme.colors.text.secondary,
    zIndex: 1,
  }),
  dotDone: css({ borderColor: theme.colors.success.border, color: theme.colors.success.text }),
  dotRunning: css({
    borderColor: theme.colors.info.border,
    color: theme.colors.info.text,
    boxShadow: `0 0 0 4px ${theme.colors.info.transparent}`,
  }),
  dotFailed: css({ borderColor: theme.colors.error.border, color: theme.colors.error.text }),
  dotSkipped: css({ borderStyle: 'dashed', color: theme.colors.text.disabled }),
  body: css({ paddingBottom: theme.spacing(2), minWidth: 0 }),
  head: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    flexWrap: 'wrap',
    minHeight: 28,
  }),
  title: css({ fontWeight: theme.typography.fontWeightMedium }),
  titleCurrent: css({ color: theme.colors.text.primary }),
  titleDim: css({ color: theme.colors.text.secondary }),
  times: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    fontVariantNumeric: 'tabular-nums',
    marginLeft: 'auto',
    whiteSpace: 'nowrap',
  }),
  steps: css({
    marginTop: theme.spacing(1),
    borderLeft: `2px solid ${theme.colors.border.weak}`,
    paddingLeft: theme.spacing(1.5),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
  }),
  step: css({
    display: 'flex',
    alignItems: 'baseline',
    gap: theme.spacing(1),
    fontSize: theme.typography.bodySmall.fontSize,
    flexWrap: 'wrap',
  }),
  stepMeta: css({ color: theme.colors.text.secondary }),
  spin: css({
    animation: 'spin 2s linear infinite',
    '@keyframes spin': { to: { transform: 'rotate(360deg)' } },
  }),
  stepError: css({ color: theme.colors.error.text, flexBasis: '100%' }),
})

function useNow(active: boolean) {
  const [, tick] = useState(0)
  useEffect(() => {
    if (!active) return
    const id = window.setInterval(() => tick((n) => n + 1), 1000)
    return () => window.clearInterval(id)
  }, [active])
}

function phaseDuration(p: Phase): number | undefined {
  if (!p.started_at) return undefined
  const end = p.finished_at ? new Date(p.finished_at).getTime() : Date.now()
  return (end - new Date(p.started_at).getTime()) / 1000
}

// Vertical phase timeline: current phase highlighted, steps expandable per phase.
export function PhaseTimeline({ phases }: { phases: Phase[] }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const [open, setOpen] = useState<Record<string, boolean>>({})
  useNow(phases.some((p) => p.status === 'running'))
  return (
    <ol className={styles.list}>
      {phases.map((p) => {
        const dotClass =
          p.status === 'completed'
            ? styles.dotDone
            : p.status === 'running'
              ? styles.dotRunning
              : p.status === 'failed' || p.status === 'cancelled'
                ? styles.dotFailed
                : p.status === 'skipped'
                  ? styles.dotSkipped
                  : undefined
        const dur = phaseDuration(p)
        const steps = p.steps ?? []
        const expanded = open[p.id] ?? p.status === 'running'
        const meta = runStatusMeta(p.status)
        return (
          <li key={p.id} className={styles.item}>
            <div className={cx(styles.dot, dotClass)} aria-hidden>
              <Icon
                name={
                  p.status === 'running'
                    ? 'sync'
                    : p.status === 'completed'
                      ? 'check'
                      : phaseIcon(p.id)
                }
                size="sm"
                className={p.status === 'running' ? styles.spin : undefined}
              />
            </div>
            <div className={styles.body}>
              <div className={styles.head}>
                <span
                  className={cx(
                    styles.title,
                    p.status === 'running' ? styles.titleCurrent : undefined,
                    p.status === 'pending' || p.status === 'skipped' ? styles.titleDim : undefined
                  )}
                >
                  {t(`common.phase.${p.id}`, { defaultValue: p.title })}
                </span>
                {p.status !== 'pending' && (
                  <StatusBadge
                    status={p.status}
                    label={t(`common.status.${meta.labelKey}`, { defaultValue: p.status })}
                  />
                )}
                {steps.length > 0 && (
                  <IconButton
                    name={expanded ? 'angle-down' : 'angle-right'}
                    size="sm"
                    tooltip={t('runs.overview.steps', { count: steps.length })}
                    onClick={() => setOpen((o) => ({ ...o, [p.id]: !expanded }))}
                  />
                )}
                <span className={styles.times}>
                  {p.started_at && (
                    <Tooltip content={formatDateTime(p.started_at)}>
                      <span>{formatTime(p.started_at)}</span>
                    </Tooltip>
                  )}
                  {p.finished_at && (
                    <Tooltip content={formatDateTime(p.finished_at)}>
                      <span> → {formatTime(p.finished_at)}</span>
                    </Tooltip>
                  )}
                  {dur !== undefined && <span> · {formatDuration(dur)}</span>}
                </span>
              </div>
              {expanded && steps.length > 0 && (
                <div className={styles.steps}>
                  {steps.map((s) => (
                    <div key={s.id} className={styles.step}>
                      <StatusBadge status={s.status} iconOnly />
                      <span>{s.title}</span>
                      {(s.machine || s.role) && (
                        <span className={styles.stepMeta}>{s.machine ?? s.role}</span>
                      )}
                      {s.attempt && s.attempt > 1 && (
                        <span className={styles.stepMeta}>
                          {t('runs.events.attempt', { n: s.attempt })}
                        </span>
                      )}
                      {s.started_at && (
                        <span className={styles.stepMeta}>
                          {formatTime(s.started_at)}
                          {s.finished_at ? ` · ${formatDuration(phaseDuration(s as Phase))}` : ''}
                        </span>
                      )}
                      {s.error && (
                        <Text color="error" variant="bodySmall">
                          {s.error}
                        </Text>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </li>
        )
      })}
    </ol>
  )
}
