import type { RunMetrics } from '@api/types'
import {
  applyFieldOverrides,
  createDataFrame,
  createTheme,
  type DataFrame,
  FieldColorModeId,
  type FieldConfigSource,
  FieldType,
  type GrafanaTheme2,
} from '@grafana/data'
import { tokens } from '@/kit/tokens'

// Points are `[unix_ms, value]`; tolerate seconds from older servers.
const toMs = (t: number): number => (t < 1e12 ? t * 1000 : t)

const UNIT_MAP: Record<string, string> = {
  ms: 'ms',
  s: 's',
  percent: 'percent',
  '%': 'percent',
  Bps: 'Bps',
  'bytes/s': 'Bps',
  bytes: 'bytes',
  tps: 'ops',
  ops: 'ops',
  '1/s': 'ops',
  count: 'short',
  short: 'short',
}

// Outside the Grafana app the standard field-config registry is empty, so `applyFieldOverrides`
// never merges `defaults` into fields. Colors and draw options are therefore set on the field
// itself; each series takes the kit's series colour by its index.
const FIELD_CONFIG: FieldConfigSource = { defaults: {}, overrides: [] }
const seriesColor = (theme: GrafanaTheme2, i: number) => {
  const palette = tokens.series[theme.colors.mode]
  return { mode: FieldColorModeId.Fixed, fixedColor: palette[i % palette.length] }
}
const SERIES_CONFIG = {
  custom: {
    lineWidth: 1,
    fillOpacity: 8,
    drawStyle: 'line',
    lineInterpolation: 'smooth',
    showPoints: 'never',
    spanNulls: true,
  },
}

// RunMetrics → one DataFrame per series with display processors applied (units, decimals),
// ready for <TimeSeries>/<Sparkline>. Grafana panels show `undefined` axes without this.
export function metricsToFrames(
  metrics: RunMetrics | undefined,
  theme: GrafanaTheme2 = createTheme(),
  keys?: string[]
): DataFrame[] {
  if (!metrics) return []
  const frames = metrics.series
    .filter((s) => !keys || keys.includes(s.key))
    .map((s, i) =>
      createDataFrame({
        name: s.title ?? s.key,
        refId: s.key,
        fields: [
          { name: 'time', type: FieldType.time, values: s.points.map((p) => toMs(p[0])) },
          {
            name: s.title ?? s.key,
            type: FieldType.number,
            values: s.points.map((p) => p[1]),
            config: {
              ...SERIES_CONFIG,
              color: seriesColor(theme, i),
              unit: UNIT_MAP[s.unit ?? ''] ?? s.unit,
              displayName: s.machine ? `${s.title ?? s.key} · ${s.machine}` : (s.title ?? s.key),
            },
          },
        ],
      })
    )
  return applyFieldOverrides({
    data: frames,
    fieldConfig: FIELD_CONFIG,
    replaceVariables: (v) => v,
    theme,
  })
}

export interface SeriesInput {
  name: string
  points: number[][]
  unit?: string
}

// Several series as one set of frames: overrides applied together, so each series gets its
// own palette colour and its own name in the legend.
export function seriesToFrames(series: SeriesInput[], theme: GrafanaTheme2): DataFrame[] {
  const frames = series
    .filter((s) => s.points.length > 0)
    .map((s, i) =>
      createDataFrame({
        name: s.name,
        refId: s.name,
        fields: [
          { name: 'time', type: FieldType.time, values: s.points.map((p) => toMs(p[0])) },
          {
            name: s.name,
            type: FieldType.number,
            values: s.points.map((p) => p[1]),
            config: {
              ...SERIES_CONFIG,
              color: seriesColor(theme, i),
              unit: UNIT_MAP[s.unit ?? ''] ?? s.unit,
              displayName: s.name,
            },
          },
        ],
      })
    )
  return applyFieldOverrides({
    data: frames,
    fieldConfig: FIELD_CONFIG,
    replaceVariables: (v) => v,
    theme,
  })
}

export function seriesToFrame(
  name: string,
  points: number[][],
  unit: string | undefined,
  theme: GrafanaTheme2
): DataFrame | undefined {
  return seriesToFrames([{ name, points, unit }], theme)[0]
}
