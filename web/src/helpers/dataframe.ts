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
// itself; classic palette = one distinct colour per series index.
const FIELD_CONFIG: FieldConfigSource = { defaults: {}, overrides: [] }
const SERIES_CONFIG = {
  color: { mode: FieldColorModeId.PaletteClassic },
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
    .map((s) =>
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

export function seriesToFrame(
  name: string,
  points: number[][],
  unit: string | undefined,
  theme: GrafanaTheme2
): DataFrame | undefined {
  if (!points.length) return undefined
  const [frame] = applyFieldOverrides({
    data: [
      createDataFrame({
        name,
        fields: [
          { name: 'time', type: FieldType.time, values: points.map((p) => toMs(p[0])) },
          {
            name,
            type: FieldType.number,
            values: points.map((p) => p[1]),
            config: { ...SERIES_CONFIG, unit: UNIT_MAP[unit ?? ''] ?? unit },
          },
        ],
      }),
    ],
    fieldConfig: FIELD_CONFIG,
    replaceVariables: (v) => v,
    theme,
  })
  return frame
}
