import { catalogQueries } from '@api/queries/catalog'
import { type RatingQuery, ratingQueries } from '@api/queries/results'
import type { RatingEntry } from '@api/types'
import { PageFill } from '@app/Page'
import { DataTableToolbar } from '@components/DataTable/Toolbar'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Button,
  Combobox,
  InlineField,
  RadioButtonGroup,
  Stack,
  Text,
  useStyles2,
} from '@grafana/ui'
import { metricTitle } from '@helpers/metrics'
import {
  defaultHardwareByScenario,
  hardwareCounts,
  hardwareGroups,
  hardwareLabel,
  hardwareTotal,
  scenarioLabel,
  sharedHardware,
} from '@helpers/rating'
import { keepPreviousData, useInfiniteQuery, useQuery } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { type ReactNode, useEffect, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { RatingFilter } from './RatingFilter'
import { RatingTable } from './RatingTable'

// The rating itself, shared by the tenant rating (both tabs) and the public
// /leaderboard: filters over the top, then one table per workload scenario.
// A tps figure means something only next to runs of the same scenario, so
// scenarios never share a table; numbers from different stands do not rank
// either, so each scenario shows one hardware class (by default its own, the
// one with the most rated rows) unless the viewer asks for all.

/**
 * One hardware class: its key, or `all` for every class at once. Unset means
 * each scenario's own default class (the one with the most rated rows in
 * it). Older links carry a JSON array; its first class is taken.
 */
export const hardwareParam = z
  .union([z.string(), z.array(z.string()).transform((a) => a[0])])
  .optional()
  .catch(undefined)
export const ALL_HARDWARE = 'all'

/** A list filter: the router writes a JSON array, a hand-made link a single value. */
const listParam = z
  .union([z.array(z.string()), z.string().transform((s) => [s])])
  .optional()
  .catch(undefined)

export const ratingFiltersSchema = z.object({
  /** One metric for every scenario; unset = each scenario by its benchmark's own. */
  metric: z.string().optional().catch(undefined),
  kind: listParam,
  version: listParam,
  provider: listParam,
  scenario: listParam,
  hardware: hardwareParam,
  // The median's window; 30 days as the API, so older versions age out.
  period: z.enum(['7d', '30d', '90d', '1y', 'all']).default('30d').catch('30d'),
})
export type RatingFilters = z.infer<typeof ratingFiltersSchema>
export const RATING_FILTER_DEFAULTS = { period: '30d' } as const

/** The metric control's value for "each scenario by its own metric". */
const OWN_METRIC = ''

/**
 * The rows the default hardware class is counted on: everything rated for
 * this metric and period (the API's largest page), whatever else is picked,
 * so the default does not jump while the viewer narrows the board.
 */
export function ratingCensusQuery(s: Pick<RatingFilters, 'metric' | 'period'>): RatingQuery {
  return { metric: s.metric, period: s.period, limit: 200 }
}

/**
 * The classes the board shows: the one in the URL, `all`, or by default each
 * scenario's own class counted on the census rows (`byScenario`). `key` goes
 * to the query (undefined = every class, the board then keeps each
 * scenario's default); `value` is what the hardware control shows
 * (undefined = by scenario).
 */
export function boardHardware(
  param: string | undefined,
  census: readonly RatingEntry[] | undefined
): {
  key: string | undefined
  value: string | undefined
  isDefault: boolean
  byScenario: Map<string, string>
} {
  if (param === ALL_HARDWARE)
    return { key: undefined, value: ALL_HARDWARE, isDefault: false, byScenario: new Map() }
  if (param) return { key: param, value: param, isDefault: false, byScenario: new Map() }
  return {
    key: undefined,
    value: undefined,
    isDefault: true,
    byScenario: census ? defaultHardwareByScenario(census) : new Map(),
  }
}

export function toRatingQuery(s: RatingFilters, hardware: string | undefined): RatingQuery {
  return {
    metric: s.metric,
    kind: s.kind as RatingQuery['kind'],
    version: s.version,
    provider: s.provider as RatingQuery['provider'],
    scenario: s.scenario,
    hardware: hardware ? [hardware] : undefined,
    period: s.period,
    limit: 50,
  }
}

const getStyles = (theme: GrafanaTheme2) => ({
  filters: css({
    display: 'flex',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: theme.spacing(1),
  }),
  // The row's gap spaces the fields; InlineField's own margin would double it.
  field: css({ margin: 0 }),
  sections: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3) }),
  sectionHead: css({
    display: 'flex',
    alignItems: 'baseline',
    flexWrap: 'wrap',
    columnGap: theme.spacing(1),
    marginBottom: theme.spacing(1),
    // The scenario name stays whole; the hint wraps under it on a narrow screen.
    '& > :first-child': { whiteSpace: 'nowrap' },
  }),
  tables: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  standHead: css({
    display: 'flex',
    alignItems: 'baseline',
    flexWrap: 'wrap',
    columnGap: theme.spacing(1),
    marginBottom: theme.spacing(0.5),
  }),
})

/** "1 × 32 vCPU · 128 GB · Intel Ice Lake · SSD · yandex" for rows on one stand; undefined when they differ. */
function standOf(t: TFunction, rows: readonly RatingEntry[]): string | undefined {
  const shared = sharedHardware(rows)
  if (!shared) return undefined
  const providers = [...new Set(rows.map((r) => r.provider_kind).filter(Boolean))]
  return [hardwareLabel(t, shared), providers.length === 1 ? providers[0] : undefined]
    .filter(Boolean)
    .join(' · ')
}

const PERIODS = ['7d', '30d', '90d', '1y', 'all'] as const

/**
 * `scope` picks the data (a tenant's opted-in runs or the public rating);
 * `catalog` what the viewer may read to fill the filters: the signed-in
 * catalog (every rating metric) or the public one (databases only — the metric
 * stays the one the rating reports).
 *
 * Filters, in this order: hardware (one class, all, or by default each
 * scenario's own — numbers from different stands do not rank), database,
 * scenario, cloud, database version; then metric and period. Each control
 * shows its value, "All" included.
 */
export function RatingBoard({
  scope,
  slug = '',
  catalog,
  search,
  onSearchChange,
  embedded,
  onReady,
}: {
  scope: 'tenant' | 'global'
  slug?: string
  catalog: 'signed-in' | 'public'
  search: RatingFilters
  onSearchChange: (next: Partial<RatingFilters>) => void
  /**
   * In an <iframe> elsewhere: the board takes its content's height (the
   * frame grows instead of scrolling) and rows open the app in a new tab.
   */
  embedded?: boolean
  /** Called once the rows (or an error) are there to draw. */
  onReady?: () => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const options = (q: RatingQuery) =>
    scope === 'global' ? ratingQueries.global(q) : ratingQueries.tenant(slug, q)

  // Everything rated for this metric and period: the default hardware class
  // is counted on it, and the cloud and version choices come from it.
  const census = useInfiniteQuery(
    options(ratingCensusQuery({ metric: search.metric, period: search.period }))
  )
  const censusRows = census.data?.pages[0]?.data
  const hw = boardHardware(search.hardware, censusRows)
  // The default class is known once the census is in (or failed: then all classes).
  const hardwareResolved = !hw.isDefault || !census.isPending
  const query = useMemo(() => toRatingQuery(search, hw.key), [search, hw.key])
  const list = useInfiniteQuery({
    ...options(query),
    enabled: hardwareResolved,
    placeholderData: keepPreviousData,
  })
  const settled = hardwareResolved && !list.isPending
  useEffect(() => {
    if (settled) onReady?.()
  }, [settled, onReady])

  const signedIn = catalog === 'signed-in'
  const metrics = useQuery({ ...catalogQueries.metrics(), enabled: signedIn })
  const databases = useQuery({ ...catalogQueries.databases(), enabled: signedIn })
  const publicCatalog = useQuery({ ...catalogQueries.public(), enabled: !signedIn })

  const entries = useMemo(() => list.data?.pages.flatMap((p) => p.data) ?? [], [list.data])
  const metricDefs = list.data?.pages[0]?.metrics ?? []
  const facets = list.data?.pages[0] ?? census.data?.pages[0]
  const scenarios = facets?.scenarios ?? []
  const hardware = facets?.hardware ?? []
  const counts = useMemo(() => hardwareCounts(censusRows ?? []), [censusRows])
  const distinct = (pick: (e: RatingEntry) => string | undefined) =>
    [...new Set((censusRows ?? entries).map(pick).filter((v): v is string => !!v))].sort()
  const kindOptions = signedIn
    ? (databases.data?.data ?? []).map((d) => ({ label: d.title ?? d.kind, value: d.kind }))
    : (publicCatalog.data?.databases ?? []).map((d) => ({
        label: d.title ?? d.kind,
        value: d.kind,
      }))
  const versionOptions = distinct((e) => e.db_version).map((v) => ({ label: v, value: v }))
  const providerOptions = distinct((e) => e.provider_kind).map((v) => ({ label: v, value: v }))
  const all = t('results.rating.filters.all')
  const hardwareOptions = [...hardware]
    .sort((a, b) => (counts.get(b.key) ?? 0) - (counts.get(a.key) ?? 0))
    .map((h) => ({
      label: hardwareLabel(t, h),
      value: h.key,
      description: [
        counts.has(h.key) && t('results.rating.entries', { count: counts.get(h.key) }),
        hardwareTotal(t, h),
      ]
        .filter(Boolean)
        .join(' · '),
    }))

  // A scenario's rows, split by stand unless the viewer asked for every stand
  // in one table; places count within a table (the server ranks the page).
  const byScenario = useMemo(() => {
    const m = new Map<string, RatingEntry[]>()
    for (const e of entries) m.set(e.scenario, [...(m.get(e.scenario) ?? []), e])
    const ranked = (rows: RatingEntry[]) => rows.map((e, i) => ({ ...e, rank: i + 1 }))
    return [...m.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([scenario, rows]) => ({
        scenario,
        count: rows.length,
        tables: (hw.value === ALL_HARDWARE ? [{ rows }] : hardwareGroups(rows)).map((g) => ({
          rows: ranked(g.rows),
        })),
      }))
  }, [entries, hw.value])

  const narrowed =
    search.hardware !== undefined ||
    !!search.kind?.length ||
    !!search.version?.length ||
    !!search.provider?.length ||
    !!search.scenario?.length
  const reset = () =>
    onSearchChange({
      hardware: undefined,
      kind: undefined,
      version: undefined,
      provider: undefined,
      scenario: undefined,
    })

  return (
    <>
      <DataTableToolbar
        left={
          <div className={styles.filters}>
            <RatingFilter
              id="rating-hardware"
              label={t('results.rating.filters.hardware')}
              allLabel={t('results.rating.filters.byScenario')}
              maxWidth={44}
              loading={hw.isDefault && census.isPending}
              options={[
                { label: t('results.rating.filters.allHardware'), value: ALL_HARDWARE },
                ...hardwareOptions,
              ]}
              values={hw.value === undefined ? undefined : [hw.value]}
              onChange={(v) => onSearchChange({ hardware: v?.[0] })}
            />
            <RatingFilter
              id="rating-kind"
              label={t('results.rating.filters.kind')}
              allLabel={all}
              options={kindOptions}
              values={search.kind}
              onChange={(kind) => onSearchChange({ kind })}
            />
            <RatingFilter
              id="rating-scenario"
              label={t('results.rating.filters.scenario')}
              allLabel={all}
              options={scenarios.map((s) => ({ label: scenarioLabel(t, s), value: s }))}
              values={search.scenario}
              onChange={(scenario) => onSearchChange({ scenario })}
            />
            <RatingFilter
              id="rating-provider"
              label={t('results.rating.filters.provider')}
              allLabel={all}
              options={providerOptions}
              values={search.provider}
              onChange={(provider) => onSearchChange({ provider })}
            />
            <RatingFilter
              id="rating-version"
              label={t('results.rating.filters.version')}
              allLabel={all}
              options={versionOptions}
              values={search.version}
              onChange={(version) => onSearchChange({ version })}
            />
            {signedIn && (
              <InlineField label={t('results.rating.filters.metric')} className={styles.field}>
                <Combobox
                  id="rating-metric"
                  width="auto"
                  minWidth={16}
                  maxWidth={32}
                  options={[
                    {
                      label: t('results.rating.filters.ownMetric'),
                      value: OWN_METRIC,
                      description: t('results.rating.filters.ownMetricHint'),
                    },
                    ...(metrics.data?.data ?? [])
                      .filter((m) => m.rating_eligible !== false)
                      .map((m) => ({
                        label: metricTitle(m.key, m.title),
                        value: m.key,
                        description: m.unit,
                      })),
                  ]}
                  value={search.metric ?? OWN_METRIC}
                  onChange={(o) => onSearchChange({ metric: o.value || undefined })}
                />
              </InlineField>
            )}
            {narrowed && (
              <Button size="sm" variant="secondary" fill="text" onClick={reset}>
                {t('results.rating.filters.reset')}
              </Button>
            )}
          </div>
        }
        right={
          <RadioButtonGroup
            size="sm"
            options={PERIODS.map((p) => ({ label: t(`results.rating.period.${p}`), value: p }))}
            value={search.period}
            onChange={(v) => onSearchChange({ period: v })}
          />
        }
      />
      <Body embedded={embedded}>
        {list.isError || !settled || entries.length === 0 ? (
          <RatingTable
            rows={[]}
            scope={scope}
            slug={slug}
            loading={!settled}
            error={list.isError ? list.error : undefined}
            onRetry={() => void list.refetch()}
            filtered={narrowed}
            onClearFilters={reset}
            empty={{ message: t('results.rating.empty.title') }}
            newTab={embedded}
            publicPage={catalog === 'public'}
          />
        ) : (
          <div className={styles.sections}>
            {byScenario.map(({ scenario, count, tables }) => {
              // A scenario ranks by one metric; its rows all carry it.
              const def = metricDefs.find((m) => m.key === tables[0]?.rows[0]?.metric)
              // A stand is said once above its table instead of on every row:
              // in the section head when the scenario has one, else above each.
              const one = tables.length === 1
              return (
                <section key={scenario}>
                  <div className={styles.sectionHead}>
                    <Text element="h2" variant="h4">
                      {scenarioLabel(t, scenario)}
                    </Text>
                    <span title={t('results.rating.scenarioHint')}>
                      <Text color="secondary" variant="bodySmall">
                        {[
                          t('results.rating.entries', { count }),
                          one ? standOf(t, tables[0]?.rows ?? []) : undefined,
                        ]
                          .filter(Boolean)
                          .join(' · ')}
                      </Text>
                    </span>
                  </div>
                  <div className={styles.tables}>
                    {tables.map(({ rows }) => (
                      <div key={rows[0]?.hardware.key ?? scenario}>
                        {!one && (
                          <div className={styles.standHead}>
                            <Text element="h3" variant="body" weight="medium">
                              {standOf(t, rows) ?? '—'}
                            </Text>
                            <Text color="secondary" variant="bodySmall">
                              {t('results.rating.entries', { count: rows.length })}
                            </Text>
                          </div>
                        )}
                        <RatingTable
                          rows={rows}
                          unit={def?.unit}
                          metricTitle={def ? metricTitle(def.key, def.title) : undefined}
                          scope={scope}
                          slug={slug}
                          newTab={embedded}
                          publicPage={catalog === 'public'}
                          hardware={!sharedHardware(rows)}
                        />
                      </div>
                    ))}
                  </div>
                </section>
              )
            })}
            {list.hasNextPage && (
              <Stack justifyContent="center">
                <Button
                  variant="secondary"
                  onClick={() => void list.fetchNextPage()}
                  disabled={list.isFetchingNextPage}
                >
                  {t('common.actions.loadMore')}
                </Button>
              </Stack>
            )}
          </div>
        )}
      </Body>
    </>
  )
}

// The page's scroll area, or the content itself inside an embed.
function Body({ embedded, children }: { embedded?: boolean; children: ReactNode }) {
  return embedded ? <div>{children}</div> : <PageFill scroll>{children}</PageFill>
}
