import type { RunMetrics } from '@api/types'
import { css } from '@emotion/css'
import { dateTime, type GrafanaTheme2, type TimeRange } from '@grafana/data'
import { LegendDisplayMode, Text, TimeSeries, useStyles2, useTheme2 } from '@grafana/ui'
import { metricsToFrames } from '@helpers/dataframe'
import { formatMetric } from '@helpers/format'
import { useEffect, useMemo, useRef, useState } from 'react'

const getStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
    gap: theme.spacing(1.5),
    [theme.breakpoints.down('md')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  chart: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(1, 1.5, 0.5),
    minWidth: 0,
    breakInside: 'avoid-page',
  }),
  head: css({
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'baseline',
    gap: theme.spacing(1),
    marginBottom: theme.spacing(0.5),
  }),
  agg: css({
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    fontVariantNumeric: 'tabular-nums',
    whiteSpace: 'nowrap',
  }),
  body: css({ width: '100%', height: 200 }),
})

function useWidth<T extends HTMLElement>(): [React.RefObject<T | null>, number] {
  const ref = useRef<T>(null)
  const [w, setW] = useState(0)
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const ro = new ResizeObserver(([entry]) => setW(Math.floor(entry.contentRect.width)))
    ro.observe(el)
    setW(el.clientWidth)
    return () => ro.disconnect()
  }, [])
  return [ref, w]
}

// Group series by key; one small chart per metric key (machines become separate lines).
export function MetricsCharts({ metrics }: { metrics: RunMetrics }) {
  const styles = useStyles2(getStyles)
  const theme = useTheme2()
  const keys = useMemo(() => [...new Set(metrics.series.map((s) => s.key))], [metrics])
  const timeRange = useMemo(() => {
    const from = dateTime(metrics.window.start)
    const to = dateTime(metrics.window.end)
    return { from, to, raw: { from, to } }
  }, [metrics.window.start, metrics.window.end])
  return (
    <div className={styles.grid}>
      {keys.map((key) => {
        const first = metrics.series.find((s) => s.key === key)
        return (
          <Chart
            key={key}
            title={first?.title ?? key}
            agg={first?.aggregates}
            unit={first?.unit}
            frames={metricsToFrames(metrics, theme, [key])}
            timeRange={timeRange}
          />
        )
      })}
    </div>
  )
}

function Chart({
  title,
  agg,
  unit,
  frames,
  timeRange,
}: {
  title: string
  agg?: { avg?: number; max?: number; p95?: number }
  unit?: string
  frames: ReturnType<typeof metricsToFrames>
  timeRange: TimeRange
}) {
  const styles = useStyles2(getStyles)
  const [ref, width] = useWidth<HTMLDivElement>()
  return (
    <div className={styles.chart}>
      <div className={styles.head}>
        <Text weight="medium">{title}</Text>
        {agg && (
          <span className={styles.agg}>
            avg {formatMetric(agg.avg, unit)} · p95 {formatMetric(agg.p95, unit)} · max{' '}
            {formatMetric(agg.max, unit)}
          </span>
        )}
      </div>
      <div ref={ref} className={styles.body}>
        {width > 0 && frames.length > 0 && (
          <TimeSeries
            frames={frames}
            width={width}
            height={200}
            timeRange={timeRange}
            timeZone="browser"
            legend={{
              displayMode: frames.length > 1 ? LegendDisplayMode.List : LegendDisplayMode.Hidden,
              placement: 'bottom',
              calcs: [],
              showLegend: frames.length > 1,
            }}
          />
        )}
      </div>
    </div>
  )
}
