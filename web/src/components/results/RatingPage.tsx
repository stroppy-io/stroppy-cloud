import { catalogQueries } from '@api/queries/catalog'
import { meQueries } from '@api/queries/me'
import { type RatingQuery, ratingQueries } from '@api/queries/results'
import type { RatingEntry } from '@api/types'
import { Page, PageFill } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { DataTableToolbar } from '@components/DataTable/Toolbar'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Alert,
  Button,
  Combobox,
  MultiCombobox,
  RadioButtonGroup,
  Stack,
  Tab,
  TabsBar,
  Text,
  useStyles2,
} from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { keepPreviousData, useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { RatingTable } from './RatingTable'

export const ratingSearchSchema = z.object({
  tab: z.enum(['tenant', 'global']).default('tenant').catch('tenant'),
  metric: z.string().default('tps').catch('tps'),
  kind: z.array(z.string()).optional().catch(undefined),
  version: z.array(z.string()).optional().catch(undefined),
  provider: z.array(z.string()).optional().catch(undefined),
  stroppy: z.array(z.string()).optional().catch(undefined),
  league: z.string().optional().catch(undefined),
  period: z.enum(['7d', '30d', '90d', '1y', 'all']).default('all').catch('all'),
})
export type RatingSearch = z.infer<typeof ratingSearchSchema>

export function toRatingQuery(s: RatingSearch): RatingQuery {
  return {
    metric: s.metric,
    kind: s.kind as RatingQuery['kind'],
    version: s.version,
    provider: s.provider as RatingQuery['provider'],
    stroppy_version: s.stroppy,
    league: s.league,
    period: s.period,
    limit: 50,
  }
}

const getStyles = (theme: GrafanaTheme2) => ({
  tabs: css({ marginBottom: theme.spacing(2) }),
  leagues: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3) }),
  leagueHead: css({
    display: 'flex',
    alignItems: 'baseline',
    gap: theme.spacing(1),
    marginBottom: theme.spacing(1),
  }),
})

const PERIODS = ['7d', '30d', '90d', '1y', 'all'] as const

export function RatingPage({
  search,
  onSearchChange,
}: {
  search: RatingSearch
  onSearchChange: (next: Partial<RatingSearch>) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug } = useTenant()
  const config = useQuery(meQueries.publicConfig())
  const globalEnabled = config.data?.public_rating_enabled ?? false
  const tab = search.tab === 'global' && globalEnabled ? 'global' : 'tenant'
  const query = useMemo(() => toRatingQuery(search), [search])
  const list = useInfiniteQuery({
    ...(tab === 'global' ? ratingQueries.global(query) : ratingQueries.tenant(slug, query)),
    placeholderData: keepPreviousData,
  })
  const metrics = useQuery(catalogQueries.metrics())
  const databases = useQuery(catalogQueries.databases())

  const entries = useMemo(() => list.data?.pages.flatMap((p) => p.data) ?? [], [list.data])
  const metricDef = list.data?.pages[0]?.metric
  const leagues = list.data?.pages[0]?.leagues ?? []
  const distinct = (pick: (e: RatingEntry) => string | undefined) =>
    [...new Set(entries.map(pick).filter((v): v is string => !!v))].sort()
  const kindOptions = (databases.data?.data ?? []).map((d) => ({
    label: d.title ?? d.kind,
    value: d.kind,
  }))
  const versionOptions = distinct((e) => e.db_version).map((v) => ({ label: v, value: v }))
  const providerOptions = distinct((e) => e.provider_kind).map((v) => ({ label: v, value: v }))
  const stroppyOptions = distinct((e) => e.stroppy_version).map((v) => ({ label: v, value: v }))

  const byLeague = useMemo(() => {
    const m = new Map<string, RatingEntry[]>()
    for (const e of entries) {
      const arr = m.get(e.league) ?? []
      arr.push(e)
      m.set(e.league, arr)
    }
    return [...m.entries()].sort(([a], [b]) => a.localeCompare(b))
  }, [entries])

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
    ...(search.league
      ? [
          {
            key: 'league',
            label: t('results.rating.league', { league: search.league }),
            onRemove: () => onSearchChange({ league: undefined }),
          },
        ]
      : []),
  ]
  const clearAll = () =>
    onSearchChange({
      kind: undefined,
      version: undefined,
      provider: undefined,
      stroppy: undefined,
      league: undefined,
    })

  return (
    <Page width="wide" fill>
      <PageHeader title={t('results.rating.title')} icon="star" />
      <TabsBar className={styles.tabs}>
        <Tab
          label={t('results.rating.tabs.tenant')}
          active={tab === 'tenant'}
          onChangeTab={() => onSearchChange({ tab: 'tenant' })}
        />
        <Tab
          label={t('results.rating.tabs.global')}
          active={tab === 'global'}
          icon="globe"
          onChangeTab={() => onSearchChange({ tab: 'global' })}
          tooltip={globalEnabled ? undefined : t('results.rating.globalDisabled')}
        />
      </TabsBar>
      {search.tab === 'global' && !globalEnabled && config.isSuccess && (
        <Alert severity="info" title={t('results.rating.globalDisabled')} />
      )}
      <DataTableToolbar
        pills={pills}
        onClearAll={pills.length ? clearAll : undefined}
        left={
          <Stack gap={1} wrap="wrap">
            <Combobox
              placeholder={t('results.rating.filters.metric')}
              width={22}
              options={(metrics.data?.data ?? [])
                .filter((m) => m.rating_eligible !== false)
                .map((m) => ({ label: m.title, value: m.key, description: m.unit }))}
              value={search.metric}
              onChange={(o) => onSearchChange({ metric: o.value })}
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
            <Combobox
              placeholder={t('results.rating.filters.league')}
              width={16}
              isClearable
              options={leagues.map((l) => ({ label: l, value: l }))}
              value={search.league ?? null}
              onChange={(o) => onSearchChange({ league: o?.value })}
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
      <PageFill scroll>
        {list.isError || list.isPending || entries.length === 0 ? (
          <RatingTable
            rows={[]}
            scope={tab}
            slug={slug}
            loading={list.isPending}
            error={list.isError ? list.error : undefined}
            onRetry={() => void list.refetch()}
            filtered={pills.length > 0}
            onClearFilters={clearAll}
            empty={{ message: t('results.rating.empty.title') }}
          />
        ) : (
          <div className={styles.leagues}>
            {byLeague.map(([league, rows]) => (
              <section key={league}>
                <div className={styles.leagueHead}>
                  <Text element="h2" variant="h4">
                    {t('results.rating.league', { league })}
                  </Text>
                  <Text color="secondary" variant="bodySmall">
                    {t('results.rating.entries', { count: rows.length })} ·{' '}
                    {t('results.rating.leagueHint')}
                  </Text>
                </div>
                <RatingTable
                  rows={rows}
                  unit={metricDef?.unit}
                  metricTitle={metricDef?.title}
                  scope={tab}
                  slug={slug}
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
      </PageFill>
    </Page>
  )
}
