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
  defaultHardware,
  hardwareCounts,
  hardwareLabel,
  hardwareTotal,
  scenarioLabel,
} from '@helpers/rating'
import { keepPreviousData, useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { type ReactNode, useEffect, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { RatingFilter } from './RatingFilter'
import { RatingTable } from './RatingTable'

// The rating itself, shared by the tenant rating (both tabs) and the public
// /leaderboard: filters over the top, then one table per workload scenario.
// A tps figure means something only next to runs of the same scenario, so
// scenarios never share a table; numbers from different stands do not rank
// either, so the board shows one hardware class unless the viewer asks for all.

/**
 * One hardware class: its key, or `all` for every class at once. Unset means
 * the default class (the one with the most rated rows). Older links carry a
 * JSON array; its first class is taken.
 */
export const hardwareParam = z
  .union([z.string(), z.array(z.string()).transform((a) => a[0])])
  .optional()
  .catch(undefined)
export const ALL_HARDWARE = 'all'

export const ratingFiltersSchema = z.object({
  metric: z.string().default('tps').catch('tps'),
  kind: z.array(z.string()).optional().catch(undefined),
  version: z.array(z.string()).optional().catch(undefined),
  provider: z.array(z.string()).optional().catch(undefined),
  scenario: z.array(z.string()).optional().catch(undefined),
  hardware: hardwareParam,
  period: z.enum(['7d', '30d', '90d', '1y', 'all']).default('all').catch('all'),
})
export type RatingFilters = z.infer<typeof ratingFiltersSchema>
export const RATING_FILTER_DEFAULTS = { metric: 'tps', period: 'all' } as const

/**
 * The rows the default hardware class is counted on: everything rated for
 * this metric and period (the API's largest page), whatever else is picked,
 * so the default does not jump while the viewer narrows the board.
 */
export function ratingCensusQuery(s: Pick<RatingFilters, 'metric' | 'period'>): RatingQuery {
  return { metric: s.metric, period: s.period, limit: 200 }
}

/**
 * The class the board shows: the one in the URL, `all`, or the default
 * counted on the census rows. `key` goes to the query (undefined = every
 * class); `value` is what the hardware control shows.
 */
export function boardHardware(
  param: string | undefined,
  census: readonly RatingEntry[] | undefined
): { key: string | undefined; value: string; isDefault: boolean } {
  if (param === ALL_HARDWARE) return { key: undefined, value: ALL_HARDWARE, isDefault: false }
  if (param) return { key: param, value: param, isDefault: false }
  const key = census ? defaultHardware(census) : undefined
  return { key, value: key ?? ALL_HARDWARE, isDefault: true }
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
})

const PERIODS = ['7d', '30d', '90d', '1y', 'all'] as const

/**
 * `scope` picks the data (a tenant's opted-in runs or the public rating);
 * `catalog` what the viewer may read to fill the filters: the signed-in
 * catalog (every rating metric) or the public one (databases only — the metric
 * stays the one the rating reports).
 *
 * Filters, in this order: hardware (one class, by default the one with the
 * most rated rows — numbers from different stands do not rank), database,
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
  const metricDef = list.data?.pages[0]?.metric
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

  // Places count within a scenario: the server ranks the whole page.
  const byScenario = useMemo(() => {
    const m = new Map<string, RatingEntry[]>()
    for (const e of entries) {
      const arr = m.get(e.scenario) ?? []
      arr.push({ ...e, rank: arr.length + 1 })
      m.set(e.scenario, arr)
    }
    return [...m.entries()].sort(([a], [b]) => a.localeCompare(b))
  }, [entries])

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
              allLabel={t('results.rating.filters.allHardware')}
              maxWidth={44}
              loading={hw.isDefault && census.isPending}
              options={hardwareOptions}
              values={hw.value === ALL_HARDWARE ? undefined : [hw.value]}
              onChange={(v) => onSearchChange({ hardware: v?.[0] ?? ALL_HARDWARE })}
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
                  options={(metrics.data?.data ?? [])
                    .filter((m) => m.rating_eligible !== false)
                    .map((m) => ({
                      label: metricTitle(m.key, m.title),
                      value: m.key,
                      description: m.unit,
                    }))}
                  value={search.metric}
                  onChange={(o) => onSearchChange({ metric: o.value })}
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
            {byScenario.map(([scenario, rows]) => (
              <section key={scenario}>
                <div className={styles.sectionHead}>
                  <Text element="h2" variant="h4">
                    {scenarioLabel(t, scenario)}
                  </Text>
                  <Text color="secondary" variant="bodySmall">
                    {t('results.rating.entries', { count: rows.length })} ·{' '}
                    {t('results.rating.scenarioHint')}
                  </Text>
                </div>
                <RatingTable
                  rows={rows}
                  unit={metricDef?.unit}
                  metricTitle={metricDef ? metricTitle(metricDef.key, metricDef.title) : undefined}
                  scope={scope}
                  slug={slug}
                  newTab={embedded}
                  publicPage={catalog === 'public'}
                />
              </section>
            ))}
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
