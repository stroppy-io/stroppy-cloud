import { catalogQueries } from '@api/queries/catalog'
import { type MetricsQuery, runQueries } from '@api/queries/runs'
import type { MetricDef, RunMetrics } from '@api/types'
import { ErrorState } from '@app/ErrorState'
import { TelemetryLayout } from '@components/runs/detail/TelemetryLayout'
import { TelemetryToolbar } from '@components/runs/detail/TelemetryToolbar'
import { telemetrySearchSchema } from '@components/runs/detail/telemetry-search'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Icon,
  LoadingPlaceholder,
  MultiSelect,
  RadioButtonGroup,
  Stack,
  Text,
  ToolbarButton,
  Tooltip,
  useStyles2,
} from '@grafana/ui'
import { formatMetric } from '@helpers/format'
import { selectedValues } from '@helpers/time-range'
import { useTelemetryRange } from '@hooks/useTelemetryRange'
import { useTenant } from '@hooks/useTenant'
import { useTopic } from '@hooks/useTopic'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { MetricPanel } from './MetricPanel'
import { MetricsRawDrawer } from './MetricsRawDrawer'

// URL: shared telemetry window + the visible metric groups + panel density.
export const runMetricsSearchSchema = telemetrySearchSchema.extend({
  group: z.array(z.string()).optional().catch(undefined),
  layout: z.enum(['grid', 'list']).default('grid').catch('grid'),
})
export type RunMetricsSearch = z.infer<typeof runMetricsSearchSchema>

const GROUP_ORDER = ['Throughput', 'Latency', 'Errors', 'Database', 'Runner', 'Host']
const GROUP_KEYS: Record<string, string> = {
  Throughput: 'throughput',
  Latency: 'latency',
  Errors: 'errors',
  Database: 'database',
  Runner: 'runner',
  Host: 'host',
}
const PANEL_HEIGHT = { grid: 240, list: 320 } as const

const getStyles = (theme: GrafanaTheme2) => ({
  // The scrolling part of the tab under the sticky toolbar.
  scroll: css({
    flex: '1 1 auto',
    minHeight: 0,
    overflow: 'auto',
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
    paddingBottom: theme.spacing(1),
  }),
  // Headline row: equal-height stat tiles, one per metric that has a value.
  stats: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))',
    gridAutoRows: theme.spacing(11),
    gap: theme.spacing(1),
  }),
  stat: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(1.5, 2),
    minWidth: 0,
    display: 'flex',
    flexDirection: 'column',
    justifyContent: 'space-between',
    overflow: 'hidden',
  }),
  statLabel: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    display: 'flex',
    justifyContent: 'space-between',
    gap: theme.spacing(1),
    whiteSpace: 'nowrap',
  }),
  statValue: css({
    fontSize: theme.typography.h2.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    fontVariantNumeric: 'tabular-nums',
    lineHeight: 1.2,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  statSub: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(360px, 1fr))',
    gap: theme.spacing(1),
    minWidth: 0,
    [theme.breakpoints.down('md')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  list: css({ gridTemplateColumns: 'minmax(0, 1fr)' }),
  group: css({ minWidth: 0 }),
  groupTitle: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(0.5, 0),
    borderBottom: `1px solid ${theme.colors.border.weak}`,
    marginBottom: theme.spacing(1),
  }),
  groupCount: css({
    marginLeft: 'auto',
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    fontVariantNumeric: 'tabular-nums',
  }),
  points: css({
    whiteSpace: 'nowrap',
    fontVariantNumeric: 'tabular-nums',
    display: 'inline-flex',
    alignItems: 'center',
    padding: theme.spacing(0, 0.5),
  }),
})

function mergeLive(prev: RunMetrics | undefined, payload: RunMetrics): RunMetrics {
  if (!prev) return payload
  const byKey = new Map(prev.series.map((s) => [s.key, s]))
  for (const s of payload.series) {
    const cur = byKey.get(s.key)
    if (!cur) {
      byKey.set(s.key, { ...s })
      continue
    }
    const lastT = cur.points.length ? cur.points[cur.points.length - 1][0] : -1
    const fresh = s.points.filter((p) => p[0] > lastT)
    if (!fresh.length) continue
    const points = [...cur.points, ...fresh]
    if (points.length > 3000) points.splice(0, points.length - 3000)
    const last = points[points.length - 1][1]
    byKey.set(s.key, {
      ...cur,
      points,
      aggregates: cur.aggregates
        ? {
            ...cur.aggregates,
            last,
            max: Math.max(cur.aggregates.max ?? last, last),
            min: Math.min(cur.aggregates.min ?? last, last),
          }
        : { last },
    })
  }
  return {
    ...prev,
    window: {
      ...prev.window,
      end: payload.window.end > prev.window.end ? payload.window.end : prev.window.end,
    },
    series: [...byKey.values()],
  }
}

export function RunMetricsTab({
  id,
  search,
  onSearchChange,
}: {
  id: string
  search: RunMetricsSearch
  onSearchChange: (next: Partial<RunMetricsSearch>) => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const range = useTelemetryRange(slug, id, search, onSearchChange)
  const { run, terminal } = range
  const catalog = useQuery(catalogQueries.metrics())

  const query = useMemo<MetricsQuery>(() => {
    const q: MetricsQuery = { start: range.start }
    if (range.end) q.end = range.end
    return q
  }, [range.start, range.end])

  const metrics = useQuery({
    ...runQueries.metrics(slug, id, query),
    placeholderData: keepPreviousData,
    refetchInterval: terminal ? false : range.refreshMs || 30_000,
  })
  useTopic<RunMetrics, RunMetrics>({
    topic: `run.metrics/${id}`,
    queryKey: runQueries.metrics(slug, id, query).queryKey,
    enabled: !!run && !terminal && !query.end,
    batchMs: 500,
    merge: (prev, payload) => mergeLive(prev, payload),
  })
  const [raw, setRaw] = useState(false)

  const defs = catalog.data?.data ?? []
  const groups = useMemo(() => {
    const present = new Set(metrics.data?.series.map((s) => s.key) ?? [])
    const byGroup = new Map<string, MetricDef[]>()
    for (const d of defs) {
      if (d.db_kinds?.length && run?.summary?.db_kind && !d.db_kinds.includes(run.summary.db_kind))
        continue
      const list = byGroup.get(d.group) ?? []
      list.push(d)
      byGroup.set(d.group, list)
    }
    // series without a catalog entry still get a panel
    for (const k of present)
      if (!defs.some((d) => d.key === k)) {
        const list = byGroup.get('Other') ?? []
        list.push({
          key: k,
          title: k,
          unit: '',
          higher_is_better: false,
          group: 'Other',
          scope: 'result',
        })
        byGroup.set('Other', list)
      }
    return [...byGroup.entries()].sort(
      ([a], [b]) =>
        (GROUP_ORDER.indexOf(a) === -1 ? 99 : GROUP_ORDER.indexOf(a)) -
        (GROUP_ORDER.indexOf(b) === -1 ? 99 : GROUP_ORDER.indexOf(b))
    )
  }, [defs, metrics.data, run])
  const groupLabel = (g: string) =>
    t(`runs.metrics.groups.${GROUP_KEYS[g] ?? 'other'}`, { defaultValue: g })
  const selectedGroups = search.group ?? []
  const visibleGroups = selectedGroups.length
    ? groups.filter(([g]) => selectedGroups.includes(g))
    : groups

  const headline = run?.result?.metrics ?? {}
  const live = (key: string) => metrics.data?.series.find((s) => s.key === key)?.aggregates
  const stat = (key: string, label: string, unit: string) => {
    const final = headline[key]?.value
    const agg = live(key)
    const value = final ?? agg?.avg ?? agg?.last
    if (value === undefined) return null
    return (
      <div key={key} className={styles.stat}>
        <div className={styles.statLabel}>
          <span>{label}</span>
          {final === undefined && agg && (
            <Tooltip content={t('runs.metrics.liveHint')}>
              <span>
                <Icon name="sync" size="sm" />
              </span>
            </Tooltip>
          )}
        </div>
        <div className={styles.statValue}>{formatMetric(value, unit)}</div>
        {agg && (
          <div className={styles.statSub}>
            {final === undefined ? t('runs.metrics.avg') : t('runs.metrics.last')}:{' '}
            {formatMetric(final === undefined ? agg.avg : agg.last, unit)} · {t('runs.metrics.max')}{' '}
            {formatMetric(agg.max, unit)}
          </div>
        )}
      </div>
    )
  }

  const totalPoints = metrics.data?.series.reduce((a, s) => a + s.points.length, 0) ?? 0
  const panelHeight = PANEL_HEIGHT[search.layout]
  const toolbar = (
    <TelemetryToolbar
      range={range}
      isLoading={metrics.isFetching}
      onRefresh={() => void metrics.refetch()}
      filters={selectedGroups.map((g) => ({
        key: g,
        label: `${t('runs.metrics.group')}: ${groupLabel(g)}`,
        onRemove: () => {
          const rest = selectedGroups.filter((x) => x !== g)
          onSearchChange({ group: rest.length ? rest : undefined })
        },
      }))}
      onResetFilters={() => onSearchChange({ group: undefined })}
      actions={
        <>
          {metrics.data && (
            <Text color="secondary" variant="bodySmall">
              <span className={styles.points}>
                {t('runs.metrics.points', { count: totalPoints })}
              </span>
            </Text>
          )}
          <RadioButtonGroup<RunMetricsSearch['layout']>
            size="md"
            value={search.layout}
            options={[
              { value: 'grid', icon: 'apps', ariaLabel: t('runs.metrics.layout.grid') },
              { value: 'list', icon: 'list-ul', ariaLabel: t('runs.metrics.layout.list') },
            ]}
            onChange={(v) => onSearchChange({ layout: v })}
          />
          <ToolbarButton
            icon="brackets-curly"
            onClick={() => setRaw(true)}
            tooltip={t('runs.metrics.raw.title')}
          >
            {t('runs.metrics.raw.button')}
          </ToolbarButton>
        </>
      }
    >
      <MultiSelect
        aria-label={t('runs.metrics.group')}
        placeholder={t('runs.metrics.allGroups')}
        width={28}
        closeMenuOnSelect={false}
        options={groups.map(([g]) => ({ label: groupLabel(g), value: g }))}
        value={selectedGroups}
        onChange={(o) => {
          const v = selectedValues(o)
          onSearchChange({ group: v.length ? v : undefined })
        }}
      />
    </TelemetryToolbar>
  )

  return (
    <TelemetryLayout toolbar={toolbar}>
      {metrics.isError ? (
        <ErrorState error={metrics.error} onRetry={() => void metrics.refetch()} compact />
      ) : (
        <div className={styles.scroll}>
          <div className={styles.stats}>
            {stat('tps', 'TPS', 'tps')}
            {stat('latency_p99_ms', 'p99', 'ms')}
            {stat('latency_p95_ms', 'p95', 'ms')}
            {stat('latency_p50_ms', 'p50', 'ms')}
            {stat('errors', t('runs.overview.result.errors'), 'count')}
          </div>

          {metrics.isPending && !metrics.data ? (
            <LoadingPlaceholder text={t('common.misc.loading')} />
          ) : totalPoints === 0 ? (
            <Stack direction="column" alignItems="center" gap={1}>
              <Text weight="medium">{t('runs.metrics.emptyTitle')}</Text>
              <Text color="secondary">
                {terminal ? t('runs.metrics.emptyTerminal') : t('runs.metrics.emptyWaiting')}
              </Text>
            </Stack>
          ) : (
            visibleGroups.map(([group, list]) => {
              const keysHere = list.map((d) => d.key)
              const has = keysHere.some((k) =>
                metrics.data?.series.some((s) => s.key === k && s.points.length)
              )
              if (!has && !selectedGroups.includes(group)) return null
              const single = list.length > 1 && (group === 'Latency' || group === 'Throughput')
              const panelCount = single ? 1 : list.length
              return (
                <section key={group} className={styles.group}>
                  <div className={styles.groupTitle}>
                    <Text element="h2" variant="h5">
                      {groupLabel(group)}
                    </Text>
                    <span className={styles.groupCount}>
                      {t('runs.metrics.panels', { count: panelCount })}
                    </span>
                  </div>
                  <div
                    className={
                      search.layout === 'list' ? `${styles.grid} ${styles.list}` : styles.grid
                    }
                  >
                    {single ? (
                      <MetricPanel
                        title={groupLabel(group)}
                        metrics={metrics.data}
                        keys={keysHere}
                        range={range.range}
                        height={panelHeight}
                        streaming={!terminal}
                        loading={metrics.isFetching && !metrics.data}
                      />
                    ) : (
                      list.map((d) => (
                        <MetricPanel
                          key={d.key}
                          title={d.title}
                          description={d.description}
                          metrics={metrics.data}
                          keys={[d.key]}
                          range={range.range}
                          height={panelHeight}
                          streaming={!terminal}
                          loading={metrics.isFetching && !metrics.data}
                        />
                      ))
                    )}
                  </div>
                </section>
              )
            })
          )}
        </div>
      )}
      {raw && (
        <MetricsRawDrawer
          slug={slug}
          runId={id}
          start={range.start}
          end={range.end ?? range.range.to.toISOString()}
          onClose={() => setRaw(false)}
        />
      )}
    </TelemetryLayout>
  )
}
