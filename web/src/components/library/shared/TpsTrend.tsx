import type { Schemas } from '@api/types'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Sparkline, Text, Tooltip, useStyles2, useTheme2 } from '@grafana/ui'
import { seriesToFrame } from '@helpers/dataframe'
import { formatMetric } from '@helpers/format'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

type Trend = NonNullable<Schemas['TestRunHistory']['trend']>

const getStyles = (theme: GrafanaTheme2) => ({
  wrap: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(1) }),
  empty: css({ color: theme.colors.text.disabled, fontSize: theme.typography.bodySmall.fontSize }),
})

// tps over the completed runs of a test — a Sparkline for lists, or a wider strip for the detail page.
export function TpsTrend({
  trend,
  width = 96,
  height = 24,
  showLast,
}: {
  trend: Trend | undefined
  width?: number
  height?: number
  showLast?: boolean
}) {
  const theme = useTheme2()
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const points = useMemo(
    () => (trend?.points ?? []).map((p) => [new Date(p.at).getTime(), p.value]),
    [trend]
  )
  const frame = useMemo(
    () => seriesToFrame(trend?.metric ?? 'tps', points, 'tps', theme),
    [points, theme, trend?.metric]
  )
  if (!frame || points.length < 2)
    return (
      <span className={styles.empty}>
        {points.length === 1 ? formatMetric(points[0][1], 'tps') : t('library.tests.noTrend')}
      </span>
    )
  const last = points[points.length - 1][1]
  const first = points[0][1]
  const delta = first ? ((last - first) / first) * 100 : 0
  return (
    <Tooltip
      content={t('library.tests.trendTooltip', {
        count: points.length,
        last: formatMetric(last, 'tps'),
        delta: `${delta >= 0 ? '+' : ''}${delta.toFixed(1)}%`,
      })}
    >
      <span className={styles.wrap}>
        <Sparkline
          width={width}
          height={height}
          theme={theme}
          sparkline={{ x: frame.fields[0], y: frame.fields[1] }}
          config={{
            ...frame.fields[1].config,
            // GraphFieldConfig lives in @grafana/schema (transitive) — plain literals are what uPlot reads.
            custom: { drawStyle: 'line', lineWidth: 1, fillOpacity: 12 } as never,
            color: {
              mode: 'fixed' as never,
              fixedColor: delta < -5 ? theme.colors.error.text : theme.colors.success.text,
            },
          }}
        />
        {showLast && <Text variant="bodySmall">{formatMetric(last, 'tps')}</Text>}
      </span>
    </Tooltip>
  )
}
