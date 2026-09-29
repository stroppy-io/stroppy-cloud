import type { SharedRun } from '@api/types'
import { StatusBadge } from '@components/StatusBadge'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Text, useStyles2 } from '@grafana/ui'
import { formatDuration, formatMetric, formatNumber } from '@helpers/format'
import { formatDateTime } from '@helpers/time'
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
  top: css({
    display: 'flex',
    alignItems: 'flex-start',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
    marginBottom: theme.spacing(2),
  }),
  h1: css({
    margin: 0,
    fontSize: theme.typography.h1.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: 1.2,
    wordBreak: 'break-word',
  }),
  facts: css({
    display: 'flex',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    marginTop: theme.spacing(1),
  }),
  fact: css({ display: 'inline-flex', gap: theme.spacing(0.5), alignItems: 'baseline' }),
  factV: css({ color: theme.colors.text.primary, fontVariantNumeric: 'tabular-nums' }),
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
  hint: css({ color: theme.colors.text.secondary, marginTop: theme.spacing(1) }),
})

export function ReportSummary({ run, title }: { run: SharedRun; title?: string }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const h = run.result?.summary
  const s = run.summary
  return (
    <div className={styles.hero} id="summary">
      <div className={styles.top}>
        <div style={{ minWidth: 0, flex: 1 }}>
          <h1 className={styles.h1}>{title ?? run.name}</h1>
          {title && <Text color="secondary">{run.name}</Text>}
          <div className={styles.facts}>
            <span className={styles.fact}>
              {t('public.summary.started')}
              <span className={styles.factV}>{formatDateTime(run.started_at)}</span>
            </span>
            <span className={styles.fact}>
              {t('public.summary.duration')}
              <span className={styles.factV}>{formatDuration(run.duration ?? h?.duration)}</span>
            </span>
            <span className={styles.fact}>
              {t('public.setup.database')}
              <span className={styles.factV}>
                {s.db_kind} {s.db_version}
              </span>
            </span>
            <span className={styles.fact}>
              {t('public.setup.topology')}
              <span className={styles.factV}>
                {s.topology_label ?? run.topology?.label ?? '—'}
                {s.node_count ? ` · ${s.node_count}n` : ''}
              </span>
            </span>
            <span className={styles.fact}>
              {t('public.setup.workload')}
              <span className={styles.factV}>{s.workload_name ?? '—'}</span>
            </span>
            {s.league && (
              <span className={styles.fact}>
                {t('public.setup.league')}
                <span className={styles.factV}>{s.league}</span>
              </span>
            )}
          </div>
        </div>
        <StatusBadge status={run.status} />
      </div>
      {h ? (
        <div className={styles.metrics}>
          <div className={styles.metric}>
            <div className={styles.mLabel}>{t('public.summary.tps')}</div>
            <div className={styles.mValue}>{formatNumber(h.tps, 0)}</div>
            <div className={styles.mSub}>tps</div>
          </div>
          <div className={styles.metric}>
            <div className={styles.mLabel}>{t('public.summary.p99')}</div>
            <div className={styles.mValue}>{formatMetric(h.latency_p99_ms, 'ms')}</div>
            <div className={styles.mSub}>
              p50 {formatMetric(h.latency_p50_ms, 'ms')} · p95{' '}
              {formatMetric(h.latency_p95_ms, 'ms')}
            </div>
          </div>
          <div className={styles.metric}>
            <div className={styles.mLabel}>{t('public.summary.errors')}</div>
            <div className={cx(styles.mValue, (h.errors ?? 0) > 0 ? styles.bad : styles.good)}>
              {formatNumber(h.errors ?? 0)}
            </div>
            <div className={styles.mSub}>
              {run.result?.metrics?.error_rate
                ? formatMetric(run.result.metrics.error_rate.value, 'percent')
                : '—'}
            </div>
          </div>
          <div className={styles.metric}>
            <div className={styles.mLabel}>{t('public.summary.duration')}</div>
            <div className={styles.mValue}>{formatDuration(h.duration ?? run.duration)}</div>
            <div className={styles.mSub}>
              {run.workload_segments?.length ?? run.result?.segments?.length ?? 0}{' '}
              {t('public.setup.segments').toLowerCase()}
            </div>
          </div>
        </div>
      ) : (
        <div className={styles.hint}>{t('public.summary.noHeadline')}</div>
      )}
    </div>
  )
}
