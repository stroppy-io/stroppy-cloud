import type { RatingEntry } from '@api/types'
import type { CellLink } from '@components/DataTable/cells'
import { col } from '@components/DataTable/columns'
import {
  DataTable,
  type DataTableColumn,
  type DataTableEmpty,
} from '@components/DataTable/DataTable'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, Tooltip, useStyles2 } from '@grafana/ui'
import { formatMetric, shortId } from '@helpers/format'
import { formatRoleSizes } from '@helpers/sizes'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  rank: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontVariantNumeric: 'tabular-nums',
    color: theme.colors.text.secondary,
  }),
  medal: css({ color: theme.colors.warning.text }),
})

// One league of the rating. Bounded in-memory list (one page per league), so it sorts on the
// client (`clientSort`) — the only table that does. No row menu: the row opens the run (tenant)
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
  const styles = useStyles2(getStyles)
  const columns = useMemo<DataTableColumn<RatingEntry>[]>(
    () => [
      col.custom<RatingEntry>({
        id: 'rank',
        header: t('results.rating.columns.rank'),
        width: 72,
        align: 'right',
        value: (r) => r.rank,
        cell: (r) =>
          r.rank === 1 ? (
            <Tooltip content={t('results.rating.first')}>
              <Icon name="star" className={styles.medal} aria-label="1" />
            </Tooltip>
          ) : (
            <span className={styles.rank}>{r.rank}</span>
          ),
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
        value: (r) => `${r.db_kind} ${r.db_version ?? ''}`,
        render: (r) => {
          const primary = `${r.db_kind}${r.db_version ? ` ${r.db_version}` : ''}`
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
        value: (r) => r.workload_name ?? r.script,
        render: (r) => {
          const primary = r.workload_name ?? r.script
          const secondary = [
            r.script && r.script !== primary ? r.script : undefined,
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
        id: 'provider',
        header: t('results.rating.columns.provider'),
        width: 190,
        value: (r) => r.provider_kind,
        render: (r) => {
          const sizes = formatRoleSizes(r.sizes)
          const secondary = [t('results.rating.league', { league: r.league }), sizes]
            .filter(Boolean)
            .join(' · ')
          return {
            primary: r.provider_kind,
            secondary,
            title: [r.provider_kind, secondary].filter(Boolean).join('\n'),
          }
        },
      }),
      col.time<RatingEntry>({
        id: 'date',
        header: t('results.rating.columns.date'),
        value: (r) => r.run_at,
      }),
    ],
    [t, styles, unit, metricTitle, scope, slug, rows]
  )
  return (
    <DataTable<RatingEntry>
      columns={columns}
      data={rows}
      getRowId={(r) => `${r.league}-${r.rank}-${r.run_id ?? r.share_token ?? r.run_at}`}
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
