import type { SharedRun } from '@api/types'
import { headlineLatency, type ResultSegment } from '@components/report/segment-metrics'
import { StatusBadge } from '@components/StatusBadge'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, useStyles2 } from '@grafana/ui'
import { dbLabel, formatDuration, formatMetric, formatNumber } from '@helpers/format'
import { formatDateTime } from '@helpers/time'
import { topologyText } from '@helpers/topology'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  hero: css({
    marginTop: theme.spacing(3),
    padding: theme.spacing(3),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    breakInside: 'avoid-page',
  }),
  eyebrow: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    flexWrap: 'wrap',
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
  }),
  h1: css({
    margin: theme.spacing(1, 0, 0),
    fontSize: theme.typography.h1.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: 1.2,
    wordBreak: 'break-word',
  }),
  lead: css({
    display: 'grid',
    gridTemplateColumns: 'max-content 1fr',
    columnGap: theme.spacing(2),
    rowGap: theme.spacing(0.5),
    margin: theme.spacing(2, 0, 0),
    fontSize: theme.typography.h5.fontSize,
    lineHeight: 1.4,
    dt: { color: theme.colors.text.secondary },
    dd: { margin: 0 },
  }),
  facts: css({
    display: 'flex',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    margin: theme.spacing(1, 0, 3),
  }),
  fact: css({ display: 'inline-flex', gap: theme.spacing(0.5), alignItems: 'center' }),
  metrics: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(4, minmax(0, 1fr))',
    gap: theme.spacing(1.5),
    [theme.breakpoints.down('md')]: { gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' },
  }),
  metric: css({
    padding: theme.spacing(1.5, 2),
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
    minWidth: 0,
  }),
  mLabel: css({
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  mValue: css({
    fontSize: theme.typography.h2.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: 1.15,
    fontVariantNumeric: 'tabular-nums',
    marginTop: theme.spacing(0.5),
  }),
  mSub: css({ fontSize: theme.typography.bodySmall.fontSize, color: theme.colors.text.secondary }),
  good: css({ color: theme.colors.success.text }),
  bad: css({ color: theme.colors.error.text }),
  hint: css({ color: theme.colors.text.secondary }),
})

interface SegmentShape {
  workload?: { script?: string; scale_factor?: number }
  run?: { vus?: number; duration?: string; iterations?: number }
}

// What the run measured, read first: the database with its shape, then the load (script, scale,
// virtual users, planned length of the first segment).
function useLead(run: SharedRun): { db: string; load: string } {
  const { t } = useTranslation()
  const s = run.summary
  const seg = (run.workload?.segments?.[0] ?? {}) as SegmentShape
  const join = (parts: (string | undefined)[]) => parts.filter(Boolean).join(' · ')
  return {
    db: join([
      dbLabel(s.db_kind, s.db_version),
      topologyText(s.topology ?? run.topology),
      s.node_count ? t('public.summary.machines', { count: s.node_count }) : '',
    ]),
    load: join([
      seg.workload?.script ?? s.workload_name,
      seg.workload?.scale_factor !== undefined
        ? t('public.summary.scale', { value: seg.workload.scale_factor })
        : '',
      seg.run?.vus ? t('public.summary.vus', { count: seg.run.vus }) : '',
      seg.run?.duration ? formatDuration(seg.run.duration) : '',
    ]),
  }
}

export function ReportSummary({ run, title }: { run: SharedRun; title?: string }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const h = run.result?.summary
  const lat = headlineLatency(run.result?.segments as ResultSegment[] | undefined)
  const lead = useLead(run)
  const name = title ?? run.name
  return (
    <header className={styles.hero} id="summary">
      <div className={styles.eyebrow}>
        <span>{t('public.kind.run')}</span>
        <StatusBadge status={run.status} />
      </div>
      <h1 className={styles.h1}>{name}</h1>
      {(lead.db || lead.load) && (
        <dl className={styles.lead}>
          {lead.db && (
            <>
              <dt>{t('public.summary.database')}</dt>
              <dd>{lead.db}</dd>
            </>
          )}
          {lead.load && (
            <>
              <dt>{t('public.summary.load')}</dt>
              <dd>{lead.load}</dd>
            </>
          )}
        </dl>
      )}
      <div className={styles.facts}>
        <span className={styles.fact}>
          <Icon name="calendar-alt" size="sm" />
          {formatDateTime(run.started_at)}
        </span>
        <span className={styles.fact}>
          <Icon name="clock-nine" size="sm" />
          {t('public.summary.tookTotal', { duration: formatDuration(run.duration ?? h?.duration) })}
        </span>
        {run.summary.provider_kind && (
          <span className={styles.fact}>
            <Icon name="cloud" size="sm" />
            {run.summary.provider_kind}
          </span>
        )}
        {name !== run.name && <span className={styles.fact}>{run.name}</span>}
      </div>
      {h ? (
        <div className={styles.metrics}>
          <div className={styles.metric}>
            <div className={styles.mLabel}>{t('public.summary.tps')}</div>
            <div className={styles.mValue}>{formatNumber(h.tps, 0)}</div>
            <div className={styles.mSub}>{t('public.summary.tpsHint')}</div>
          </div>
          <div className={styles.metric}>
            <div className={styles.mLabel}>{t('public.summary.p99')}</div>
            <div className={styles.mValue}>{formatMetric(lat?.p99 ?? h.latency_p99_ms, 'ms')}</div>
            <div className={styles.mSub}>
              p50 {formatMetric(lat?.p50 ?? h.latency_p50_ms, 'ms')} · p95{' '}
              {formatMetric(lat?.p95 ?? h.latency_p95_ms, 'ms')}
            </div>
          </div>
          <div className={styles.metric}>
            <div className={styles.mLabel}>{t('public.summary.errors')}</div>
            <div className={cx(styles.mValue, (h.errors ?? 0) > 0 ? styles.bad : styles.good)}>
              {formatNumber(h.errors ?? 0)}
            </div>
            <div className={styles.mSub}>
              {run.result?.metrics?.error_rate
                ? t('public.summary.errorRate', {
                    rate: formatMetric(run.result.metrics.error_rate.value, 'percent'),
                  })
                : t('public.summary.errorsHint')}
            </div>
          </div>
          <div className={styles.metric}>
            <div className={styles.mLabel}>{t('public.summary.loadDuration')}</div>
            <div className={styles.mValue}>{formatDuration(h.duration ?? run.duration)}</div>
            <div className={styles.mSub}>
              {t('public.summary.segments', {
                count: run.workload_segments?.length ?? run.result?.segments?.length ?? 0,
              })}
            </div>
          </div>
        </div>
      ) : (
        <div className={styles.hint}>{t('public.summary.noHeadline')}</div>
      )}
    </header>
  )
}
