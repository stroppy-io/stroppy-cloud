import type { RunMetrics } from '@api/types'
import { css } from '@emotion/css'
import {
  type DataFrame,
  dateTime,
  type GrafanaTheme2,
  LoadingState,
  type TimeRange,
} from '@grafana/data'
import {
  LegendDisplayMode,
  PanelChrome,
  Text,
  TimeSeries,
  TooltipDisplayMode,
  useStyles2,
  useTheme2,
} from '@grafana/ui'
import { metricsToFrames } from '@helpers/dataframe'
import { formatMetric } from '@helpers/format'
import { useMeasure } from '@hooks/useMeasure'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({ minWidth: 0, width: '100%' }),
  body: css({ width: '100%', minHeight: 0, minWidth: 0 }),
  empty: css({
    height: '100%',
    display: 'grid',
    placeItems: 'center',
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  aggs: css({
    display: 'flex',
    gap: theme.spacing(1.5),
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    fontVariantNumeric: 'tabular-nums',
  }),
})

export interface MetricPanelProps {
  title: string
  description?: string
  metrics: RunMetrics | undefined
  keys: string[]
  height?: number
  loading?: boolean
  streaming?: boolean
  range?: TimeRange
}

export function rangeOf(metrics: RunMetrics | undefined): TimeRange {
  const start = metrics?.window.start
    ? new Date(metrics.window.start)
    : new Date(Date.now() - 600_000)
  const end = metrics?.window.end ? new Date(metrics.window.end) : new Date()
  return {
    from: dateTime(start),
    to: dateTime(end),
    raw: { from: start.toISOString(), to: end.toISOString() },
  }
}

// One PanelChrome + TimeSeries for a group of series keys; width/height are measured.
export function MetricPanel({
  title,
  description,
  metrics,
  keys,
  height = 260,
  loading,
  streaming,
  range,
}: MetricPanelProps) {
  const styles = useStyles2(getStyles)
  const theme = useTheme2()
  const { t } = useTranslation()
  const [ref, size] = useMeasure<HTMLDivElement>()
  const frames: DataFrame[] = useMemo(
    () => metricsToFrames(metrics, theme, keys).filter((f) => f.length > 0),
    [metrics, theme, keys]
  )
  const timeRange = useMemo(() => range ?? rangeOf(metrics), [range, metrics])
  const structureRev = frames.map((f) => f.refId).join('|').length
  const series = (metrics?.series ?? []).filter((s) => keys.includes(s.key) && s.aggregates)
  const errors = (metrics?.errors ?? []).filter((e) => e.key && keys.includes(e.key))
  return (
    <div className={styles.root}>
      <PanelChrome
        title={title}
        description={description}
        loadingState={
          loading ? LoadingState.Loading : streaming ? LoadingState.Streaming : undefined
        }
        statusMessage={
          errors.length ? errors.map((e) => `${e.key}: ${e.error}`).join('; ') : undefined
        }
        actions={
          series.length === 1 && series[0].aggregates ? (
            <div className={styles.aggs}>
              <span>
                {t('runs.metrics.avg')} {formatMetric(series[0].aggregates.avg, series[0].unit)}
              </span>
              <span>
                {t('runs.metrics.max')} {formatMetric(series[0].aggregates.max, series[0].unit)}
              </span>
              <span>
                {t('runs.metrics.last')} {formatMetric(series[0].aggregates.last, series[0].unit)}
              </span>
            </div>
          ) : undefined
        }
      >
        <div ref={ref} className={styles.body} style={{ height }}>
          {frames.length === 0 ? (
            <div className={styles.empty}>
              <Text color="secondary">
                {loading ? t('common.misc.loading') : t('runs.metrics.noData')}
              </Text>
            </div>
          ) : size.width > 0 ? (
            <TimeSeries
              frames={frames}
              structureRev={structureRev}
              width={size.width}
              height={height}
              timeRange={timeRange}
              timeZone="browser"
              legend={{
                displayMode: frames.length > 1 ? LegendDisplayMode.List : LegendDisplayMode.Hidden,
                placement: 'bottom',
                showLegend: frames.length > 1,
                calcs: [],
              }}
              options={{ tooltip: { mode: TooltipDisplayMode.Multi } }}
            />
          ) : null}
        </div>
      </PanelChrome>
    </div>
  )
}
