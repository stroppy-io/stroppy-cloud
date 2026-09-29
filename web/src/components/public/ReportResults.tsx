import { publicQueries } from '@api/queries/results'
import type { SharedRun } from '@api/types'
import { StatusBadge } from '@components/StatusBadge'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Icon, LoadingPlaceholder, useStyles2 } from '@grafana/ui'
import { formatMetric, formatNumber } from '@helpers/format'
import { formatDateTime } from '@helpers/time'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { MetricsCharts } from './MetricsCharts'
import { ReportSection, SimpleTable, SubTitle, useReportStyles } from './ReportPrimitives'

const getStyles = (theme: GrafanaTheme2) => ({
  verdicts: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
    marginTop: theme.spacing(1),
  }),
  verdict: css({
    display: 'flex',
    gap: theme.spacing(1),
    alignItems: 'baseline',
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  ok: css({ color: theme.colors.success.text }),
  bad: css({ color: theme.colors.error.text }),
  check: css({ fontFamily: theme.typography.fontFamilyMonospace }),
})

export function ReportResults({
  run,
  token,
  scope,
  index,
}: {
  run: SharedRun
  token: string
  scope: 'overview' | 'metrics' | 'configs'
  index: number
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const rs = useReportStyles()
  const withMetrics = scope !== 'overview'
  // Snapshot carries metrics already; the endpoint is used when the snapshot came without them.
  const metricsQ = useQuery(publicQueries.shareMetrics(token, {}, withMetrics && !run.metrics))
  const metrics = run.metrics ?? metricsQ.data
  const segments = run.result?.segments ?? []
  const baseline = run.result?.baseline

  return (
    <ReportSection
      id="results"
      index={index}
      title={t('public.results.title')}
      hint={t('public.results.hint')}
    >
      <SubTitle>{t('public.results.segments')}</SubTitle>
      {segments.length === 0 ? (
        <span className={rs.muted}>—</span>
      ) : (
        <SimpleTable>
          <thead>
            <tr>
              <th>{t('public.results.segment.name')}</th>
              <th>{t('public.results.segment.status')}</th>
              <th className="num">{t('public.results.segment.tps')}</th>
              <th className="num">{t('public.results.segment.errors')}</th>
              <th className="num">{t('public.results.segment.exit')}</th>
              <th>{t('common.fields.started')}</th>
              <th>{t('public.results.segment.error')}</th>
            </tr>
          </thead>
          <tbody>
            {segments.map((s) => (
              <tr key={s.name}>
                <td className="mono">{s.name}</td>
                <td>
                  <StatusBadge status={s.status} />
                </td>
                <td className="num">{formatMetric(s.metrics?.tps?.value, 'tps')}</td>
                <td className="num">{formatNumber(s.errors?.failed_queries ?? 0)}</td>
                <td className="num">{s.exit_code ?? '—'}</td>
                <td>{formatDateTime(s.started_at)}</td>
                <td className={s.error ? styles.bad : undefined}>{s.error ?? '—'}</td>
              </tr>
            ))}
          </tbody>
        </SimpleTable>
      )}

      {baseline && (
        <>
          <SubTitle>{t('public.results.baseline')}</SubTitle>
          <div className={baseline.ok ? styles.ok : styles.bad}>
            <Icon name={baseline.ok ? 'check-circle' : 'exclamation-circle'} />{' '}
            {baseline.ok ? t('public.results.baselineOk') : t('public.results.baselineFailed')}
            {baseline.error ? ` — ${baseline.error}` : ''}
          </div>
          {baseline.verdicts && (
            <div className={styles.verdicts}>
              {baseline.verdicts.map((v) => (
                <div key={v.check} className={styles.verdict}>
                  <span className={v.status === 'ok' ? styles.ok : styles.bad}>
                    <Icon name={v.status === 'ok' ? 'check' : 'times'} size="sm" />
                  </span>
                  <span className={styles.check}>{v.check}</span>
                  <span className={rs.muted}>{v.detail ?? v.status}</span>
                </div>
              ))}
            </div>
          )}
        </>
      )}

      <SubTitle>{t('public.results.metrics')}</SubTitle>
      {!withMetrics ? (
        <Alert severity="info" title={t('public.results.metricsNotShared')} />
      ) : metricsQ.isPending && !metrics ? (
        <LoadingPlaceholder text={t('common.misc.loading')} />
      ) : !metrics || metrics.series.length === 0 ? (
        <span className={rs.muted}>{t('public.results.noMetrics')}</span>
      ) : (
        <>
          <p className={rs.muted}>
            {t('public.results.metricsHint')} · {formatDateTime(metrics.window.start)} —{' '}
            {formatDateTime(metrics.window.end)}
          </p>
          <MetricsCharts metrics={metrics} />
        </>
      )}
    </ReportSection>
  )
}
