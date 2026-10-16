import type { RunMetricSeries } from '@api/types'
import { css, cx } from '@emotion/css'
import {
  formattedValueToString,
  type GrafanaTheme2,
  getValueFormat,
  LoadingState,
  type TimeRange,
} from '@grafana/data'
import {
  Alert,
  Icon,
  IconButton,
  LegendDisplayMode,
  MultiSelect,
  PanelChrome,
  RadioButtonGroup,
  Select,
  Text,
  TimeSeries,
  TooltipDisplayMode,
  useStyles2,
  useTheme2,
} from '@grafana/ui'
import { seriesToFrames } from '@helpers/dataframe'
import {
  type ExplorerAgg,
  type ExplorerFn,
  type Pick,
  type SeriesSortKey,
  seriesLabel,
  sortSeries,
  unitOf,
} from '@helpers/metric-explorer'
import { selectedValues } from '@helpers/time-range'
import { useMeasure } from '@hooks/useMeasure'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({ minWidth: 0 }),
  controls: css({
    display: 'flex',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(0, 0, 1),
  }),
  chart: css({ width: '100%', minWidth: 0 }),
  empty: css({
    height: '100%',
    display: 'grid',
    placeItems: 'center',
    color: theme.colors.text.secondary,
  }),
  table: css({
    width: '100%',
    borderCollapse: 'collapse',
    fontSize: theme.typography.bodySmall.fontSize,
    fontVariantNumeric: 'tabular-nums',
    'th, td': {
      padding: theme.spacing(0.5, 1),
      borderBottom: `1px solid ${theme.colors.border.weak}`,
      textAlign: 'right',
      whiteSpace: 'nowrap',
    },
    'th:first-child, td:first-child': {
      textAlign: 'left',
      whiteSpace: 'normal',
      wordBreak: 'break-all',
    },
    th: {
      color: theme.colors.text.secondary,
      fontWeight: theme.typography.fontWeightMedium,
      cursor: 'pointer',
      userSelect: 'none',
      position: 'sticky',
      top: 0,
      background: theme.colors.background.primary,
    },
    'tbody tr': { cursor: 'pointer' },
    'tbody tr:hover': { background: theme.colors.action.hover },
  }),
  tableWrap: css({ maxHeight: theme.spacing(32), overflow: 'auto', marginTop: theme.spacing(1) }),
  hidden: css({ color: theme.colors.text.disabled, textDecoration: 'line-through' }),
  swatch: css({
    display: 'inline-block',
    width: theme.spacing(1),
    height: theme.spacing(1),
    borderRadius: theme.shape.radius.circle,
    marginRight: theme.spacing(0.75),
    verticalAlign: 'middle',
  }),
  sortIcon: css({ marginLeft: theme.spacing(0.25), verticalAlign: 'middle' }),
})

const COLUMNS: { key: SeriesSortKey; label: string }[] = [
  { key: 'label', label: 'series' },
  { key: 'avg', label: 'avg' },
  { key: 'min', label: 'min' },
  { key: 'max', label: 'max' },
  { key: 'last', label: 'last' },
  { key: 'p95', label: 'p95' },
]

export function ExplorerChart({
  pick,
  data,
  loading,
  error,
  range,
  height,
  onChange,
  onRemove,
}: {
  pick: Pick
  data: RunMetricSeries | undefined
  loading: boolean
  error?: string
  range: TimeRange
  height: number
  onChange: (next: Pick) => void
  onRemove: () => void
}) {
  const styles = useStyles2(getStyles)
  const theme = useTheme2()
  const { t } = useTranslation()
  const [ref, size] = useMeasure<HTMLDivElement>()
  const [sort, setSort] = useState<{ key: SeriesSortKey; desc: boolean }>({
    key: 'avg',
    desc: true,
  })
  const [hidden, setHidden] = useState<Set<string>>(new Set())
  const fn: ExplorerFn = data?.fn ?? pick.fn ?? 'value'
  const unit = unitOf(pick.name, fn)
  const format = useMemo(() => getValueFormat(unit ?? 'short'), [unit])
  const fmt = (v: number | undefined) =>
    v === undefined || Number.isNaN(v) ? '—' : formattedValueToString(format(v, 2))

  const rows = useMemo(
    () => (data?.series ?? []).map((s) => ({ ...s, label: seriesLabel(s, pick.name) })),
    [data, pick.name]
  )
  // Colours follow the series order the server ranked (highest average first).
  const colourOf = useMemo(() => {
    const frames = seriesToFrames(
      rows.map((r) => ({ name: r.label, points: r.points, unit })),
      theme
    )
    const m = new Map<string, string>()
    for (const f of frames) {
      const c = f.fields[1]?.config.color?.fixedColor
      if (c) m.set(f.name ?? '', c)
    }
    return m
  }, [rows, unit, theme])
  const frames = useMemo(() => {
    const out = seriesToFrames(
      rows.map((r) => ({
        name: r.label,
        points: hidden.has(r.label) ? [] : r.points,
        unit,
      })),
      theme
    )
    // Fills of many series pile up into one grey block: lines only past a handful.
    if (out.length > 4)
      for (const f of out) {
        const field = f.fields[1]
        if (field) field.config.custom = { ...field.config.custom, fillOpacity: 0 }
      }
    return out
  }, [rows, hidden, unit, theme])
  const sorted = useMemo(() => sortSeries(rows, sort.key, sort.desc), [rows, sort])
  const groupOptions = [
    { label: t('runs.metrics.explorer.byMachine'), value: 'machine' },
    { label: t('runs.metrics.explorer.byContainer'), value: 'container' },
    ...(data?.labels ?? []).map((l) => ({ label: l, value: l })),
  ]
  const toggle = (label: string) =>
    setHidden((prev) => {
      const next = new Set(prev)
      if (next.has(label)) next.delete(label)
      else next.add(label)
      return next
    })

  return (
    <div className={styles.root}>
      <PanelChrome
        title={pick.name}
        loadingState={loading ? LoadingState.Loading : undefined}
        actions={
          <IconButton
            name="times"
            tooltip={t('runs.metrics.explorer.remove')}
            onClick={onRemove}
            size="md"
          />
        }
      >
        <div className={styles.controls}>
          <RadioButtonGroup<ExplorerFn>
            size="sm"
            value={fn}
            options={[
              { value: 'value', label: t('runs.metrics.explorer.fn.value') },
              { value: 'rate', label: t('runs.metrics.explorer.fn.rate') },
            ]}
            onChange={(v) => onChange({ ...pick, fn: v })}
          />
          <MultiSelect
            aria-label={t('runs.metrics.explorer.groupBy')}
            placeholder={t('runs.metrics.explorer.groupBy')}
            width={34}
            options={groupOptions}
            value={pick.by}
            onChange={(o) => onChange({ ...pick, by: selectedValues(o) })}
          />
          {pick.by.length > 0 && (
            <Select<ExplorerAgg>
              aria-label={t('runs.metrics.explorer.agg')}
              width={14}
              value={pick.agg ?? 'sum'}
              options={(['sum', 'avg', 'max', 'min'] as const).map((a) => ({
                value: a,
                label: t(`runs.metrics.explorer.aggs.${a}`),
              }))}
              onChange={(o) => onChange({ ...pick, agg: o.value })}
            />
          )}
          {data && (
            <Text color="secondary" variant="bodySmall">
              {t('runs.metrics.explorer.seriesCount', { count: data.total })}
            </Text>
          )}
        </div>
        {error && <Alert severity="error" title={error} />}
        {data?.truncated && (
          <Text color="secondary" variant="bodySmall">
            {t('runs.metrics.explorer.truncated', { shown: data.series.length, total: data.total })}
          </Text>
        )}
        <div ref={ref} className={styles.chart} style={{ height }}>
          {frames.length === 0 ? (
            <div className={styles.empty}>
              <Text color="secondary">
                {loading ? t('common.misc.loading') : t('runs.metrics.noData')}
              </Text>
            </div>
          ) : size.width > 0 ? (
            <TimeSeries
              frames={frames}
              structureRev={frames.length + hidden.size}
              width={size.width}
              height={height}
              timeRange={range}
              timeZone="browser"
              legend={{
                displayMode: LegendDisplayMode.Hidden,
                placement: 'bottom',
                showLegend: false,
                calcs: [],
              }}
              options={{ tooltip: { mode: TooltipDisplayMode.Multi } }}
            />
          ) : null}
        </div>
        {rows.length > 0 && (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  {COLUMNS.map((c) => (
                    <th
                      key={c.key}
                      onClick={() =>
                        setSort((s) => ({
                          key: c.key,
                          desc: s.key === c.key ? !s.desc : c.key !== 'label',
                        }))
                      }
                      aria-sort={
                        sort.key === c.key ? (sort.desc ? 'descending' : 'ascending') : 'none'
                      }
                    >
                      {t(`runs.metrics.explorer.columns.${c.label}`)}
                      {sort.key === c.key && (
                        <Icon
                          className={styles.sortIcon}
                          name={sort.desc ? 'arrow-down' : 'arrow-up'}
                          size="sm"
                        />
                      )}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {sorted.map((r) => (
                  <tr
                    key={r.label}
                    className={cx(hidden.has(r.label) && styles.hidden)}
                    onClick={() => toggle(r.label)}
                    title={t('runs.metrics.explorer.toggleSeries')}
                  >
                    <td>
                      <span
                        className={styles.swatch}
                        style={{ background: colourOf.get(r.label) }}
                      />
                      {r.label}
                    </td>
                    <td>{fmt(r.aggregates?.avg)}</td>
                    <td>{fmt(r.aggregates?.min)}</td>
                    <td>{fmt(r.aggregates?.max)}</td>
                    <td>{fmt(r.aggregates?.last)}</td>
                    <td>{fmt(r.aggregates?.p95)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </PanelChrome>
    </div>
  )
}
