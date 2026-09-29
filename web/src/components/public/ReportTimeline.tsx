import type { RunEvent, SharedRun } from '@api/types'
import { eventSubject, eventTitle } from '@components/runs/events/eventTitle'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Icon, Tooltip, useStyles2 } from '@grafana/ui'
import { formatDuration } from '@helpers/format'
import { phaseIcon, runStatusMeta } from '@helpers/run-status'
import { formatTime } from '@helpers/time'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ReportSection, SubTitle, useReportStyles } from './ReportPrimitives'

const getStyles = (theme: GrafanaTheme2) => ({
  bar: css({
    display: 'flex',
    width: '100%',
    height: 28,
    borderRadius: theme.shape.radius.default,
    overflow: 'hidden',
    border: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.secondary,
  }),
  seg: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: theme.spacing(0.5),
    fontSize: theme.typography.bodySmall.fontSize,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    minWidth: 0,
    borderRight: `1px solid ${theme.colors.background.primary}`,
    color: theme.colors.text.primary,
    '&:last-child': { borderRight: 0 },
  }),
  segLabel: css({ overflow: 'hidden', textOverflow: 'ellipsis' }),
  tone: (tone: 'success' | 'error' | 'warning' | 'info' | 'secondary') =>
    css({
      background:
        tone === 'secondary' ? theme.colors.background.secondary : theme.colors[tone].transparent,
    }),
  legend: css({
    display: 'flex',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
    marginTop: theme.spacing(1),
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
  }),
  legendItem: css({ display: 'inline-flex', gap: theme.spacing(0.5), alignItems: 'center' }),
  events: css({
    listStyle: 'none',
    margin: 0,
    padding: 0,
    position: 'relative',
    '&::before': {
      content: '""',
      position: 'absolute',
      left: 79,
      top: 4,
      bottom: 4,
      width: 1,
      background: theme.colors.border.medium,
    },
  }),
  event: css({
    display: 'grid',
    gridTemplateColumns: '68px 24px minmax(0, 1fr)',
    gap: theme.spacing(1),
    alignItems: 'start',
    padding: theme.spacing(0.5, 0),
    breakInside: 'avoid',
  }),
  time: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    textAlign: 'right',
    paddingTop: 2,
  }),
  dot: css({
    width: 24,
    height: 24,
    borderRadius: '50%',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: theme.colors.background.primary,
    border: `1px solid ${theme.colors.border.medium}`,
    zIndex: 1,
  }),
  dotTone: (tone: 'success' | 'error' | 'warning' | 'info' | 'secondary') =>
    css({
      color: tone === 'secondary' ? theme.colors.text.secondary : theme.colors[tone].text,
      borderColor: tone === 'secondary' ? theme.colors.border.medium : theme.colors[tone].border,
    }),
  title: css({ paddingTop: 2 }),
  subject: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    fontFamily: theme.typography.fontFamilyMonospace,
    marginLeft: theme.spacing(1),
  }),
  error: css({
    color: theme.colors.error.text,
    fontSize: theme.typography.bodySmall.fontSize,
    marginTop: 2,
  }),
  more: css({ marginTop: theme.spacing(1), '@media print': { display: 'none' } }),
})

interface Span {
  key: string
  label: string
  start: number
  end: number
  tone: 'success' | 'error' | 'warning' | 'info' | 'secondary'
  icon?: ReturnType<typeof phaseIcon>
  phase?: boolean
}

// Phases are reconstructed from `phase.started` / `phase.completed|failed|skipped` events.
function phaseSpans(events: RunEvent[], runEnd: number): Span[] {
  const spans: Span[] = []
  const open = new Map<string, Span>()
  for (const e of events) {
    const at = new Date(e.at).getTime()
    if (e.kind === 'phase.started' && e.subject) {
      const s: Span = {
        key: e.subject,
        label: e.subject,
        start: at,
        end: runEnd,
        tone: 'info',
        icon: phaseIcon(e.subject),
        phase: true,
      }
      open.set(e.subject, s)
      spans.push(s)
    } else if (e.kind.startsWith('phase.') && e.subject && open.has(e.subject)) {
      const s = open.get(e.subject) as Span
      s.end = at
      const st = e.kind.slice('phase.'.length)
      s.tone =
        st === 'failed' ? 'error' : st === 'skipped' || st === 'cancelled' ? 'secondary' : 'success'
      open.delete(e.subject)
    }
  }
  return spans
}

function eventTone(e: RunEvent): 'success' | 'error' | 'warning' | 'info' | 'secondary' {
  if (e.error || e.kind.endsWith('.failed') || e.status === 'failed') return 'error'
  if (e.kind.endsWith('.completed') || e.status === 'ready' || e.status === 'completed')
    return 'success'
  if (e.kind.endsWith('.started')) return 'info'
  return runStatusMeta(e.status).tone
}

const PREVIEW = 12

export function ReportTimeline({ run, index }: { run: SharedRun; index: number }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const rs = useReportStyles()
  const [all, setAll] = useState(false)
  const events = useMemo(
    () => [...run.timeline].sort((a, b) => a.at.localeCompare(b.at)),
    [run.timeline]
  )
  const start = events.length
    ? new Date(events[0].at).getTime()
    : new Date(run.started_at ?? Date.now()).getTime()
  const end = run.finished_at
    ? new Date(run.finished_at).getTime()
    : events.length
      ? new Date(events[events.length - 1].at).getTime()
      : start
  const total = Math.max(1, end - start)
  const phases = useMemo(() => phaseSpans(events, end), [events, end])
  const segments = (run.workload_segments ?? []).filter((s) => s.started_at)
  const shown = all ? events : events.slice(0, PREVIEW)

  const renderBar = (spans: Span[]) => (
    <div className={styles.bar} role="img">
      {spans.map((s) => {
        const w = Math.max(2, ((s.end - s.start) / total) * 100)
        return (
          <Tooltip
            key={s.key}
            content={`${s.phase ? t(`common.phase.${s.key}`, { defaultValue: s.label }) : s.label} · ${formatDuration((s.end - s.start) / 1000)}`}
          >
            <div className={cx(styles.seg, styles.tone(s.tone))} style={{ width: `${w}%` }}>
              {s.icon && <Icon name={s.icon} size="sm" />}
              <span className={styles.segLabel}>
                {s.phase ? t(`common.phase.${s.key}`, { defaultValue: s.label }) : s.label}
              </span>
            </div>
          </Tooltip>
        )
      })}
    </div>
  )

  return (
    <ReportSection
      id="timeline"
      index={index}
      title={t('public.timeline.title')}
      hint={t('public.timeline.hint')}
    >
      {phases.length > 0 && (
        <>
          <SubTitle>{t('public.timeline.phases')}</SubTitle>
          {renderBar(phases)}
          <div className={styles.legend}>
            {phases.map((p) => (
              <span key={p.key} className={styles.legendItem}>
                {p.icon && <Icon name={p.icon} size="sm" />}
                {t(`common.phase.${p.key}`, { defaultValue: p.label })} ·{' '}
                {formatDuration((p.end - p.start) / 1000)}
              </span>
            ))}
          </div>
        </>
      )}
      {segments.length > 0 && (
        <>
          <SubTitle>{t('public.timeline.segments')}</SubTitle>
          {renderBar(
            segments.map((s) => ({
              key: s.name ?? '',
              label: s.name ?? '',
              start: new Date(s.started_at ?? start).getTime(),
              end: new Date(s.finished_at ?? end).getTime(),
              tone: 'success' as const,
              icon: 'rocket' as const,
            }))
          )}
        </>
      )}
      <SubTitle>{t('common.fields.events')}</SubTitle>
      {events.length === 0 ? (
        <span className={rs.muted}>{t('public.timeline.empty')}</span>
      ) : (
        <>
          <ol className={styles.events}>
            {shown.map((e) => {
              const tone = eventTone(e)
              const meta = runStatusMeta(e.status)
              return (
                <li key={e.id} className={styles.event}>
                  <span className={styles.time}>{formatTime(e.at)}</span>
                  <span className={cx(styles.dot, styles.dotTone(tone))}>
                    <Icon
                      name={
                        e.kind.startsWith('phase.') && e.subject
                          ? phaseIcon(e.subject)
                          : tone === 'error'
                            ? 'exclamation-circle'
                            : tone === 'success'
                              ? 'check'
                              : meta.icon
                      }
                      size="sm"
                    />
                  </span>
                  <div className={styles.title}>
                    {eventTitle(e, t)}
                    {e.subject && (
                      <span className={styles.subject} title={e.subject}>
                        {eventSubject(e, t)}
                      </span>
                    )}
                    {e.attempt && e.attempt > 1 && (
                      <span className={styles.subject}>
                        {t('public.timeline.attempt', { n: e.attempt })}
                      </span>
                    )}
                    {e.error && <div className={styles.error}>{e.error}</div>}
                  </div>
                </li>
              )
            })}
          </ol>
          {events.length > PREVIEW && (
            <div className={styles.more}>
              <Button size="sm" variant="secondary" fill="text" onClick={() => setAll((v) => !v)}>
                {all
                  ? t('public.timeline.showLess')
                  : t('public.timeline.showAll', { count: events.length })}
              </Button>
            </div>
          )}
        </>
      )}
    </ReportSection>
  )
}
