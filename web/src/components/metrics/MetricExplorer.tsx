import type { RunMetricName, RunMetricNames, RunMetricSeries } from '@api/types'
import { ErrorState } from '@app/ErrorState'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2, TimeRange } from '@grafana/data'
import {
  Badge,
  Button,
  FilterInput,
  Icon,
  LoadingPlaceholder,
  MultiSelect,
  Select,
  Text,
  useStyles2,
} from '@grafana/ui'
import {
  decodePick,
  encodePick,
  facetOf,
  filterNames,
  type NameFilter,
  type NameSort,
  type Pick,
  sortNames,
} from '@helpers/metric-explorer'
import { selectedValues } from '@helpers/time-range'
import {
  keepPreviousData,
  type QueryKey,
  type UseQueryOptions,
  useQueries,
  useQuery,
} from '@tanstack/react-query'
import { useVirtualizer } from '@tanstack/react-virtual'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ExplorerChart } from './ExplorerChart'

/** The explorer's state; the run tab keeps it in the URL, the public report in memory. */
export interface ExplorerState extends NameFilter {
  sort?: NameSort
  /** Picked charts, encodePick form. */
  pick?: string[]
}

/** The series query of one picked chart, as the endpoints take it. */
export interface ExplorerSeriesQuery {
  name: string
  fn?: Pick['fn']
  by?: string[]
  agg?: Pick['agg']
  role?: string[]
  machine?: string[]
  container?: string[]
}

/** The queries the explorer runs: the run's endpoints or a share's. */
export interface ExplorerSource<KN extends QueryKey = QueryKey, KS extends QueryKey = QueryKey> {
  names: UseQueryOptions<RunMetricNames, Error, RunMetricNames, KN>
  series: (q: ExplorerSeriesQuery) => UseQueryOptions<RunMetricSeries, Error, RunMetricSeries, KS>
}

const ORIGINS = ['exporter', 'workload', 'container', 'pipeline'] as const
const KINDS = ['counter', 'gauge'] as const
const SORTS: NameSort[] = ['name', '-name', 'series', '-series', 'origin']
const ROW = 52

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    flex: '1 1 auto',
    minHeight: 0,
    display: 'grid',
    gridTemplateColumns: `minmax(${theme.spacing(37.5)}, ${theme.spacing(47.5)}) minmax(0, 1fr)`,
    gap: theme.spacing(2),
    [theme.breakpoints.down('md')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  side: css({
    minHeight: 0,
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(1),
  }),
  filters: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.75) }),
  count: css({
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  list: css({ flex: '1 1 auto', minHeight: 0, overflow: 'auto' }),
  row: css({
    position: 'absolute',
    left: 0,
    right: 0,
    display: 'flex',
    gap: theme.spacing(1),
    alignItems: 'center',
    padding: theme.spacing(0.5, 1),
    borderRadius: theme.shape.radius.default,
    cursor: 'pointer',
    '&:hover': { background: theme.colors.action.hover },
  }),
  picked: css({ background: theme.colors.action.selected }),
  rowBody: css({ minWidth: 0, flex: 1, display: 'flex', flexDirection: 'column' }),
  name: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  }),
  meta: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  }),
  charts: css({
    minHeight: 0,
    overflow: 'auto',
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
  }),
  hint: css({
    display: 'grid',
    placeItems: 'center',
    minHeight: theme.spacing(30),
    border: `1px dashed ${theme.colors.border.medium}`,
    borderRadius: theme.shape.radius.default,
    color: theme.colors.text.secondary,
    textAlign: 'center',
    padding: theme.spacing(2),
  }),
})

export function MetricExplorer<KN extends QueryKey, KS extends QueryKey>({
  source,
  state,
  onChange,
  range,
  chartHeight = 260,
  height,
}: {
  source: ExplorerSource<KN, KS>
  state: ExplorerState
  onChange: (next: Partial<ExplorerState>) => void
  range: TimeRange
  chartHeight?: number
  /** A fixed height where the page scrolls as a whole; otherwise it fills its parent. */
  height?: number
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const names = useQuery(source.names)
  const all = names.data?.data ?? []
  const sort = state.sort ?? 'name'
  const shown = useMemo(() => sortNames(filterNames(all, state), sort), [all, state, sort])
  const picks = useMemo(
    () => (state.pick ?? []).map(decodePick).filter((p): p is Pick => !!p),
    [state.pick]
  )
  const pickedNames = new Set(picks.map((p) => p.name))
  const scope = { role: state.role, machine: state.machine, container: state.container }
  const series = useQueries({
    queries: picks.map((p) => ({
      ...source.series({
        name: p.name,
        fn: p.fn,
        by: p.by.length ? p.by : undefined,
        agg: p.agg,
        ...scope,
      }),
      // A changed split keeps the chart drawn until the new answer arrives.
      placeholderData: keepPreviousData,
    })),
  })

  const listRef = useRef<HTMLDivElement>(null)
  const virtual = useVirtualizer({
    count: shown.length,
    getScrollElement: () => listRef.current,
    estimateSize: () => ROW,
    overscan: 12,
  })

  const setPicks = (next: Pick[]) =>
    onChange({ pick: next.length ? next.map(encodePick) : undefined })
  const toggle = (m: RunMetricName) =>
    pickedNames.has(m.name)
      ? setPicks(picks.filter((p) => p.name !== m.name))
      : setPicks([{ name: m.name, by: [] }, ...picks])
  const facet = (pick: (m: RunMetricName) => string[]) =>
    facetOf(all, pick).map((v) => ({ label: `${v.value} (${v.count})`, value: v.value }))
  const multi = (
    key: 'origin' | 'kind' | 'role' | 'machine' | 'container',
    label: string,
    options: { label: string; value: string }[]
  ) =>
    options.length > 0 && (
      <MultiSelect
        aria-label={label}
        placeholder={label}
        options={options}
        value={state[key] ?? []}
        closeMenuOnSelect={false}
        onChange={(o) => {
          const v = selectedValues(o)
          onChange({ [key]: v.length ? v : undefined })
        }}
      />
    )
  const filtered =
    !!state.q ||
    !!state.origin?.length ||
    !!state.kind?.length ||
    !!state.role?.length ||
    !!state.machine?.length ||
    !!state.container?.length

  if (names.isError)
    return <ErrorState error={names.error} onRetry={() => void names.refetch()} compact />

  return (
    <div className={styles.root} style={height ? { height } : undefined}>
      <aside className={styles.side} aria-label={t('runs.metrics.explorer.list')}>
        <div className={styles.filters}>
          <ExplorerSearch
            value={state.q ?? ''}
            placeholder={t('runs.metrics.explorer.search')}
            onChange={(v) => onChange({ q: v || undefined })}
          />
          {multi(
            'origin',
            t('runs.metrics.explorer.origin'),
            ORIGINS.filter((o) => all.some((m) => m.origin === o)).map((o) => ({
              label: `${t(`runs.metrics.origin.${o}`)} (${all.filter((m) => m.origin === o).length})`,
              value: o,
            }))
          )}
          {multi(
            'role',
            t('runs.metrics.explorer.role'),
            facet((m) => m.roles)
          )}
          {multi(
            'machine',
            t('runs.metrics.explorer.machine'),
            facet((m) => m.machines)
          )}
          {multi(
            'container',
            t('runs.metrics.explorer.container'),
            facet((m) => m.containers)
          )}
          {multi(
            'kind',
            t('runs.metrics.explorer.kind'),
            KINDS.map((k) => ({ label: t(`runs.metrics.kind.${k}`), value: k }))
          )}
          <Select<NameSort>
            aria-label={t('runs.metrics.explorer.sort')}
            prefix={<Icon name="sort-amount-down" />}
            value={sort}
            options={SORTS.map((s) => ({
              value: s,
              label: t(`runs.metrics.explorer.sorts.${s.replace('-', 'desc_')}`),
            }))}
            onChange={(o) => onChange({ sort: o.value === 'name' ? undefined : o.value })}
          />
        </div>
        <div className={styles.count}>
          <span>
            {t('runs.metrics.explorer.shown', { shown: shown.length, total: all.length })}
          </span>
          {filtered && (
            <Button
              size="sm"
              variant="secondary"
              fill="text"
              onClick={() =>
                onChange({
                  q: undefined,
                  origin: undefined,
                  kind: undefined,
                  role: undefined,
                  machine: undefined,
                  container: undefined,
                })
              }
            >
              {t('runs.metrics.explorer.reset')}
            </Button>
          )}
        </div>
        <div ref={listRef} className={styles.list}>
          {names.isPending ? (
            <LoadingPlaceholder text={t('common.misc.loading')} />
          ) : shown.length === 0 ? (
            <Text color="secondary">
              {all.length ? t('runs.metrics.explorer.nothing') : t('runs.metrics.emptyTitle')}
            </Text>
          ) : (
            <div style={{ height: virtual.getTotalSize(), position: 'relative' }}>
              {virtual.getVirtualItems().map((v) => {
                const m = shown[v.index]
                const picked = pickedNames.has(m.name)
                return (
                  // biome-ignore lint/a11y/useSemanticElements: a virtualised row, not a form control
                  <div
                    key={m.name}
                    role="checkbox"
                    aria-checked={picked}
                    tabIndex={0}
                    className={cx(styles.row, picked && styles.picked)}
                    style={{ top: v.start, height: ROW }}
                    onClick={() => toggle(m)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault()
                        toggle(m)
                      }
                    }}
                    title={m.name}
                  >
                    <Icon name={picked ? 'check-circle' : 'circle'} />
                    <div className={styles.rowBody}>
                      <span className={styles.name}>{m.name}</span>
                      <span className={styles.meta}>
                        {[
                          t(`runs.metrics.kind.${m.kind}`),
                          t('runs.metrics.explorer.seriesCount', { count: m.series }),
                          m.containers.join(', ') || m.machines.join(', '),
                        ]
                          .filter(Boolean)
                          .join(' · ')}
                      </span>
                    </div>
                    <Badge
                      text={t(`runs.metrics.origin.${m.origin}`)}
                      color={ORIGIN_COLOR[m.origin]}
                    />
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </aside>
      <div className={styles.charts}>
        {picks.length === 0 ? (
          <div className={styles.hint}>
            <div>
              <Text element="p" weight="medium">
                {t('runs.metrics.explorer.pickTitle')}
              </Text>
              <Text element="p" color="secondary">
                {t('runs.metrics.explorer.pickHint', { count: all.length })}
              </Text>
            </div>
          </div>
        ) : (
          picks.map((p, i) => (
            <ExplorerChart
              key={p.name}
              pick={p}
              data={series[i]?.data}
              loading={!!series[i]?.isFetching}
              error={series[i]?.isError ? String(series[i].error) : undefined}
              range={range}
              height={chartHeight}
              onChange={(next) => setPicks(picks.map((x) => (x.name === p.name ? next : x)))}
              onRemove={() => setPicks(picks.filter((x) => x.name !== p.name))}
            />
          ))
        )}
      </div>
    </div>
  )
}

const ORIGIN_COLOR = {
  exporter: 'blue',
  workload: 'green',
  container: 'purple',
  pipeline: 'orange',
} as const

// The list filters as you type; the URL is written 300 ms after the last keystroke, so a
// navigation per key never eats input.
function ExplorerSearch({
  value,
  placeholder,
  onChange,
}: {
  value: string
  placeholder: string
  onChange: (v: string) => void
}) {
  const [local, setLocal] = useState(value)
  useEffect(() => setLocal(value), [value])
  useEffect(() => {
    if (local === value) return
    const id = setTimeout(() => onChange(local), 300)
    return () => clearTimeout(id)
  }, [local, value, onChange])
  const styles = useStyles2(getSearchStyles)
  return (
    <div className={styles.search}>
      <FilterInput value={local} onChange={setLocal} placeholder={placeholder} />
    </div>
  )
}

// Grafana's Input measures its prefix to pad the text; the measure reads 0 at times and the
// icon then covers the first letters. The search icon is always there: pad for it outright.
const getSearchStyles = (theme: GrafanaTheme2) => ({
  search: css({ input: { paddingLeft: `${theme.spacing(4)} !important` } }),
})
