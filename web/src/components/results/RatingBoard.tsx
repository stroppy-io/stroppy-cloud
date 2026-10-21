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
  MultiCombobox,
  RadioButtonGroup,
  Stack,
  Text,
  useStyles2,
} from '@grafana/ui'
import { metricTitle } from '@helpers/metrics'
import { hardwareLabel, hardwareTotal, scenarioLabel } from '@helpers/rating'
import { keepPreviousData, useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { type ReactNode, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { RatingTable } from './RatingTable'

// The rating itself, shared by the tenant rating (both tabs) and the public
// /leaderboard: filters over the top, then one table per workload scenario.
// A tps figure means something only next to runs of the same scenario, so
// scenarios never share a table, and each row says what hardware it ran on.

export const ratingFiltersSchema = z.object({
  metric: z.string().default('tps').catch('tps'),
  kind: z.array(z.string()).optional().catch(undefined),
  version: z.array(z.string()).optional().catch(undefined),
  provider: z.array(z.string()).optional().catch(undefined),
  stroppy: z.array(z.string()).optional().catch(undefined),
  scenario: z.array(z.string()).optional().catch(undefined),
  hardware: z.array(z.string()).optional().catch(undefined),
  period: z.enum(['7d', '30d', '90d', '1y', 'all']).default('all').catch('all'),
})
export type RatingFilters = z.infer<typeof ratingFiltersSchema>
export const RATING_FILTER_DEFAULTS = { metric: 'tps', period: 'all' } as const

export function toRatingQuery(s: RatingFilters): RatingQuery {
  return {
    metric: s.metric,
    kind: s.kind as RatingQuery['kind'],
    version: s.version,
    provider: s.provider as RatingQuery['provider'],
    stroppy_version: s.stroppy,
    scenario: s.scenario,
    hardware: s.hardware,
    period: s.period,
    limit: 50,
  }
}

const getStyles = (theme: GrafanaTheme2) => ({
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
 */
export function RatingBoard({
  scope,
  slug = '',
  catalog,
  search,
  onSearchChange,
  embedded,
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
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const query = useMemo(() => toRatingQuery(search), [search])
  const list = useInfiniteQuery({
    ...(scope === 'global' ? ratingQueries.global(query) : ratingQueries.tenant(slug, query)),
    placeholderData: keepPreviousData,
  })
  const signedIn = catalog === 'signed-in'
  const metrics = useQuery({ ...catalogQueries.metrics(), enabled: signedIn })
  const databases = useQuery({ ...catalogQueries.databases(), enabled: signedIn })
  const publicCatalog = useQuery({ ...catalogQueries.public(), enabled: !signedIn })

  const entries = useMemo(() => list.data?.pages.flatMap((p) => p.data) ?? [], [list.data])
  const metricDef = list.data?.pages[0]?.metric
  const scenarios = list.data?.pages[0]?.scenarios ?? []
  const hardware = list.data?.pages[0]?.hardware ?? []
  const distinct = (pick: (e: RatingEntry) => string | undefined) =>
    [...new Set(entries.map(pick).filter((v): v is string => !!v))].sort()
  const kindOptions = signedIn
    ? (databases.data?.data ?? []).map((d) => ({ label: d.title ?? d.kind, value: d.kind }))
    : (publicCatalog.data?.databases ?? []).map((d) => ({
        label: d.title ?? d.kind,
        value: d.kind,
      }))
  const versionOptions = distinct((e) => e.db_version).map((v) => ({ label: v, value: v }))
  const providerOptions = distinct((e) => e.provider_kind).map((v) => ({ label: v, value: v }))
  const stroppyOptions = distinct((e) => e.stroppy_version).map((v) => ({ label: v, value: v }))

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
  const hardwareName = (key: string) =>
    hardwareLabel(
      t,
      hardware.find((h) => h.key === key)
    )

  const pills = [
    ...(search.kind ?? []).map((v) => ({
      key: `kind:${v}`,
      label: `${t('results.rating.filters.kind')}: ${v}`,
      onRemove: () => onSearchChange({ kind: search.kind?.filter((x) => x !== v) }),
    })),
    ...(search.version ?? []).map((v) => ({
      key: `ver:${v}`,
      label: `${t('results.rating.filters.version')}: ${v}`,
      onRemove: () => onSearchChange({ version: search.version?.filter((x) => x !== v) }),
    })),
    ...(search.provider ?? []).map((v) => ({
      key: `prov:${v}`,
      label: `${t('results.rating.filters.provider')}: ${v}`,
      onRemove: () => onSearchChange({ provider: search.provider?.filter((x) => x !== v) }),
    })),
    ...(search.stroppy ?? []).map((v) => ({
      key: `st:${v}`,
      label: `${t('results.rating.filters.stroppy')}: ${v}`,
      onRemove: () => onSearchChange({ stroppy: search.stroppy?.filter((x) => x !== v) }),
    })),
    ...(search.scenario ?? []).map((v) => ({
      key: `sc:${v}`,
      label: `${t('results.rating.filters.scenario')}: ${scenarioLabel(v)}`,
      onRemove: () => onSearchChange({ scenario: search.scenario?.filter((x) => x !== v) }),
    })),
    ...(search.hardware ?? []).map((v) => ({
      key: `hw:${v}`,
      label: `${t('results.rating.filters.hardware')}: ${hardwareName(v)}`,
      onRemove: () => onSearchChange({ hardware: search.hardware?.filter((x) => x !== v) }),
    })),
  ]
  const clearAll = () =>
    onSearchChange({
      kind: undefined,
      version: undefined,
      provider: undefined,
      stroppy: undefined,
      scenario: undefined,
      hardware: undefined,
    })

  return (
    <>
      <DataTableToolbar
        pills={pills}
        onClearAll={pills.length ? clearAll : undefined}
        left={
          <Stack gap={1} wrap="wrap">
            {signedIn && (
              <Combobox
                placeholder={t('results.rating.filters.metric')}
                width={22}
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
            )}
            <MultiCombobox
              placeholder={t('results.rating.filters.scenario')}
              width={18}
              options={scenarios.map((s) => ({ label: scenarioLabel(s), value: s }))}
              value={search.scenario ?? []}
              onChange={(o) =>
                onSearchChange({ scenario: o.length ? o.map((x) => x.value) : undefined })
              }
            />
            <MultiCombobox
              placeholder={t('results.rating.filters.kind')}
              width={18}
              options={kindOptions}
              value={search.kind ?? []}
              onChange={(o) =>
                onSearchChange({ kind: o.length ? o.map((x) => x.value) : undefined })
              }
            />
            <MultiCombobox
              placeholder={t('results.rating.filters.version')}
              width={16}
              options={versionOptions}
              value={search.version ?? []}
              onChange={(o) =>
                onSearchChange({ version: o.length ? o.map((x) => x.value) : undefined })
              }
            />
            <MultiCombobox
              placeholder={t('results.rating.filters.provider')}
              width={16}
              options={providerOptions}
              value={search.provider ?? []}
              onChange={(o) =>
                onSearchChange({ provider: o.length ? o.map((x) => x.value) : undefined })
              }
            />
            <MultiCombobox
              placeholder={t('results.rating.filters.stroppy')}
              width={16}
              options={stroppyOptions}
              value={search.stroppy ?? []}
              onChange={(o) =>
                onSearchChange({ stroppy: o.length ? o.map((x) => x.value) : undefined })
              }
            />
            <MultiCombobox
              placeholder={t('results.rating.filters.hardware')}
              width={26}
              options={hardware.map((h) => ({
                label: hardwareLabel(t, h),
                value: h.key,
                description: hardwareTotal(t, h),
              }))}
              value={search.hardware ?? []}
              onChange={(o) =>
                onSearchChange({ hardware: o.length ? o.map((x) => x.value) : undefined })
              }
            />
          </Stack>
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
        {list.isError || list.isPending || entries.length === 0 ? (
          <RatingTable
            rows={[]}
            scope={scope}
            slug={slug}
            loading={list.isPending}
            error={list.isError ? list.error : undefined}
            onRetry={() => void list.refetch()}
            filtered={pills.length > 0}
            onClearFilters={clearAll}
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
                    {scenarioLabel(scenario)}
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
