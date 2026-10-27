import { publicQueries } from '@api/queries/results'
import type { SharedRun } from '@api/types'
import {
  type ExplorerSearch,
  explorerSearchOf,
  explorerStateOf,
} from '@components/metrics/explorer-search'
import { type ExplorerSeriesQuery, MetricExplorer } from '@components/metrics/MetricExplorer'
import {
  ReportSection,
  SimpleTable,
  SubTitle,
  useReportStyles,
} from '@components/report/ReportPrimitives'
import { ErrorsPanel, LatencyPanel } from '@components/report/RunResultPanels'
import type { ResultSegment } from '@components/report/segment-metrics'
import { StatusBadge } from '@components/StatusBadge'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { dateTime, type TimeRange } from '@grafana/data'
import { Alert, Icon, LoadingPlaceholder, useStyles2 } from '@grafana/ui'
import { formatMetric, formatNumber } from '@helpers/format'
import { formatDateTime } from '@helpers/time'
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { MetricsCharts } from './MetricsCharts'

const windowRange = (w: { start: string; end: string }): TimeRange => {
  const from = dateTime(w.start)
  const to = dateTime(w.end)
  return { from, to, raw: { from, to } }
}

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

// Snapshot carries metrics already; the endpoint is used when the snapshot came without them.
function useShareMetrics(run: SharedRun, token: string, enabled: boolean) {
  const q = useQuery(publicQueries.shareMetrics(token, {}, enabled && !run.metrics))
  return { metrics: run.metrics ?? q.data, pending: q.isPending }
}

export function ReportResults({
  run,
  token,
  scope,
  index,
  tabbed,
}: {
  run: SharedRun
  token: string
  scope: 'overview' | 'metrics' | 'configs'
  index: number
  tabbed?: boolean
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const rs = useReportStyles()
  const withMetrics = scope !== 'overview'
  const { metrics, pending } = useShareMetrics(run, token, withMetrics)
  const segments = run.result?.segments ?? []
  const baseline = run.result?.baseline

  return (
    <ReportSection
      id="results"
      index={index}
      title={t('public.results.title')}
      hint={t('public.results.hint')}
      tabbed={tabbed}
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

      <LatencyPanel segments={segments as ResultSegment[]} />
      <ErrorsPanel segments={segments as ResultSegment[]} />

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
      ) : pending && !metrics ? (
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

// Every series the run collected, searchable — the run page's explorer over the share token.
export function ReportExplorer({
  run,
  token,
  index,
  tabbed,
  search,
  onSearchChange,
}: {
  run: SharedRun
  token: string
  index: number
  tabbed?: boolean
  search: ExplorerSearch
  onSearchChange: (next: Partial<ExplorerSearch>) => void
}) {
  const { t } = useTranslation()
  const rs = useReportStyles()
  const { metrics, pending } = useShareMetrics(run, token, true)
  const source = useMemo(
    () => ({
      names: publicQueries.shareMetricNames(token),
      series: (q: ExplorerSeriesQuery) => publicQueries.shareMetricSeries(token, q),
    }),
    [token]
  )
  return (
    <ReportSection
      id="explorer"
      index={index}
      title={t('public.explorer.title')}
      hint={t('public.explorer.hint')}
      tabbed={tabbed}
    >
      {pending && !metrics ? (
        <LoadingPlaceholder text={t('common.misc.loading')} />
      ) : !metrics ? (
        <span className={rs.muted}>{t('public.results.noMetrics')}</span>
      ) : (
        <MetricExplorer
          source={source}
          state={explorerStateOf(search)}
          onChange={(next) => onSearchChange(explorerSearchOf(next))}
          range={windowRange(metrics.window)}
          chartHeight={220}
          height={720}
        />
      )}
    </ReportSection>
  )
}
