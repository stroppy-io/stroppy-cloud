import type { RatingEntry } from '@api/types'
import type { CellLink } from '@components/DataTable/cells'
import { col } from '@components/DataTable/columns'
import {
  DataTable,
  type DataTableColumn,
  type DataTableEmpty,
} from '@components/DataTable/DataTable'
import { RankBadge } from '@components/RankBadge'
import { dbLabel, formatMetric, shortId } from '@helpers/format'
import { hardwareLabel, scenarioLabel } from '@helpers/rating'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

// One scenario of the rating. Bounded in-memory list, so it sorts on the client (`clientSort`),
// the only table that does. No row menu: the row opens the run (tenant)
// or its public report (global).
export function RatingTable({
  rows,
  unit,
  metricTitle,
  scope,
  slug,
  loading,
  error,
  onRetry,
  empty,
  filtered,
  onClearFilters,
}: {
  rows: RatingEntry[]
  unit?: string
  metricTitle?: string
  scope: 'tenant' | 'global'
  slug: string
  loading?: boolean
  error?: unknown
  onRetry?: () => void
  empty?: DataTableEmpty
  filtered?: boolean
  onClearFilters?: () => void
}) {
  const { t } = useTranslation()
  const columns = useMemo<DataTableColumn<RatingEntry>[]>(
    () => [
      col.custom<RatingEntry>({
        id: 'rank',
        header: t('results.rating.columns.rank'),
        width: 72,
        align: 'right',
        value: (r) => r.rank,
        cell: (r) => <RankBadge rank={r.rank} />,
      }),
      col.identity<RatingEntry>({
        id: 'run',
        header: t('results.rating.columns.run'),
        minWidth: 200,
        render: (r): { title: string; link?: CellLink; subtitle?: string } =>
          scope === 'tenant'
            ? {
                title: r.run_id ? shortId(r.run_id) : '—',
                link: r.run_id
                  ? { to: '/t/$slug/runs/$id', params: { slug, id: r.run_id } }
                  : undefined,
                subtitle: r.author?.display_name,
              }
            : {
                title: r.tenant_name ?? t('results.rating.anonymous'),
                link: r.share_token
                  ? { to: '/s/$token', params: { token: r.share_token } }
                  : undefined,
                subtitle: r.share_token ? t('results.rating.openShare') : undefined,
              },
      }),
      col.stack<RatingEntry>({
        id: 'database',
        header: t('results.rating.columns.database'),
        minWidth: 180,
        value: (r) => dbLabel(r.db_kind, r.db_version),
        render: (r) => {
          const primary = dbLabel(r.db_kind, r.db_version)
          const secondary = [
            r.topology_label,
            r.node_count ? t('runs.list.nodes', { count: r.node_count }) : undefined,
          ]
            .filter(Boolean)
            .join(' · ')
          return { primary, secondary, title: secondary ? `${primary}\n${secondary}` : primary }
        },
      }),
      col.stack<RatingEntry>({
        id: 'workload',
        header: t('results.rating.columns.workload'),
        minWidth: 200,
        value: (r) => r.workload_name ?? scenarioLabel(r.scenario),
        render: (r) => {
          const scenario = scenarioLabel(r.scenario)
          const primary = r.workload_name ?? scenario
          const secondary = [
            scenario !== primary ? scenario : undefined,
            r.stroppy_version ? `stroppy ${r.stroppy_version}` : undefined,
          ]
            .filter(Boolean)
            .join(' · ')
          return {
            primary,
            secondary,
            title: [primary, secondary].filter(Boolean).join('\n'),
          }
        },
      }),
      col.number<RatingEntry>({
        id: 'value',
        header: metricTitle ?? t('results.rating.columns.value'),
        width: 190,
        bar: true,
        value: (r) => r.value,
        format: (v) => formatMetric(v, unit ?? rows[0]?.unit),
      }),
      col.stack<RatingEntry>({
        id: 'hardware',
        header: t('results.rating.columns.hardware'),
        minWidth: 220,
        value: (r) => r.hardware.key,
        render: (r) => {
          const primary = hardwareLabel(t, r.hardware)
          const secondary = r.provider_kind
          return {
            primary,
            secondary,
            title: [primary, secondary, t('results.rating.hardwareHint')]
              .filter(Boolean)
              .join('\n'),
          }
        },
      }),
      col.time<RatingEntry>({
        id: 'date',
        header: t('results.rating.columns.date'),
        value: (r) => r.run_at,
      }),
    ],
    [t, unit, metricTitle, scope, slug, rows]
  )
  return (
    <DataTable<RatingEntry>
      columns={columns}
      data={rows}
      getRowId={(r) => `${r.scenario}-${r.rank}-${r.run_id ?? r.share_token ?? r.run_at}`}
      clientSort
      loading={loading}
      error={error}
      onRetry={onRetry}
      empty={empty}
      filtered={filtered}
      onClearFilters={onClearFilters}
      rowHref={(r) =>
        scope === 'tenant'
          ? r.run_id
            ? `/t/${slug}/runs/${r.run_id}`
            : undefined
          : r.share_token
            ? `/s/${r.share_token}`
            : undefined
      }
    />
  )
}
