import type { RatingEntry } from '@api/types'
import type { CellLink } from '@components/DataTable/cells'
import { col } from '@components/DataTable/columns'
import {
  DataTable,
  type DataTableColumn,
  type DataTableEmpty,
} from '@components/DataTable/DataTable'
import { RankBadge } from '@components/RankBadge'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import { dbLabel, formatMetric, shortId } from '@helpers/format'
import { hardwareLabel, scenarioLabel } from '@helpers/rating'
import { relativeTime } from '@helpers/time'
import { topologyText } from '@helpers/topology'
import { useMeasure } from '@hooks/useMeasure'
import { useRouter } from '@tanstack/react-router'
import { type MouseEvent, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

// The table gives up columns by priority as it narrows, instead of
// scrolling sideways: the date folds under the hardware and the value's bar
// shortens below FULL_FROM; only on a phone (below TABLE_FROM) do rows
// become cards.
const FULL_FROM = 1100
const TABLE_FROM = 640

const getStyles = (theme: GrafanaTheme2) => ({
  cards: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  card: css({
    display: 'grid',
    gridTemplateColumns: 'auto minmax(0, 1fr) auto',
    columnGap: theme.spacing(1.5),
    rowGap: theme.spacing(0.25),
    alignItems: 'baseline',
    padding: theme.spacing(1.25, 1.5),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    color: theme.colors.text.primary,
    textDecoration: 'none',
    '&:hover': {
      background: theme.colors.action.hover,
      color: theme.colors.text.primary,
      textDecoration: 'none',
    },
    '&:focus-visible': { outline: `2px solid ${theme.colors.primary.main}`, outlineOffset: 2 },
  }),
  rank: css({ gridRow: '1 / span 4', alignSelf: 'start' }),
  db: css({ fontWeight: theme.typography.fontWeightMedium, minWidth: 0 }),
  value: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontWeight: theme.typography.fontWeightMedium,
    whiteSpace: 'nowrap',
    textAlign: 'right',
  }),
  line: css({
    gridColumn: '2 / span 2',
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    minWidth: 0,
    overflowWrap: 'anywhere',
  }),
})

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
  newTab,
  publicPage,
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
  /** Rows open their run in a new tab (an embed elsewhere sends readers to the app). */
  newTab?: boolean
  /**
   * A page for guests (the public leaderboard, the site's embed): every row
   * there reads "<team> · Public report" — the row itself opens the report,
   * so the Run column goes.
   */
  publicPage?: boolean
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const router = useRouter()
  const [box, size] = useMeasure<HTMLDivElement>()
  const full = size.width === 0 || size.width >= FULL_FROM
  const columns = useMemo<DataTableColumn<RatingEntry>[]>(
    () => [
      col.custom<RatingEntry>({
        id: 'rank',
        header: t('results.rating.columns.rank'),
        width: full ? 72 : 64,
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
                  ? {
                      to: '/s/$token',
                      params: { token: r.share_token },
                      target: newTab ? '_blank' : undefined,
                    }
                  : undefined,
                subtitle: r.share_token ? t('results.rating.openShare') : undefined,
              },
      }),
      col.stack<RatingEntry>({
        id: 'database',
        header: t('results.rating.columns.database'),
        minWidth: full ? 180 : 140,
        value: (r) => dbLabel(r.db_kind, r.db_version),
        render: (r) => {
          const primary = dbLabel(r.db_kind, r.db_version)
          const secondary = [
            topologyText(r.topology),
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
        minWidth: full ? 200 : 150,
        value: (r) => r.workload_name ?? scenarioLabel(r.scenario),
        render: (r) => {
          const scenario = scenarioLabel(r.scenario)
          const primary = r.workload_name ?? scenario
          // The scenario heads the section the table sits in: only the tool's version here.
          const secondary = r.stroppy_version ? `stroppy ${r.stroppy_version}` : undefined
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
        width: full ? 190 : 144,
        bar: true,
        value: (r) => r.value,
        format: (v) => formatMetric(v, unit ?? rows[0]?.unit),
      }),
      col.stack<RatingEntry>({
        id: 'hardware',
        header: t('results.rating.columns.hardware'),
        minWidth: full ? 220 : 170,
        value: (r) => r.hardware.key,
        render: (r) => {
          const primary = hardwareLabel(t, r.hardware)
          // Without its own column the date reads under the hardware.
          const secondary = full
            ? r.provider_kind
            : [r.provider_kind, relativeTime(r.run_at)].filter(Boolean).join(' · ')
          return {
            primary,
            secondary,
            title: [primary, secondary, t('results.rating.hardwareHint')]
              .filter(Boolean)
              .join('\n'),
          }
        },
      }),
      ...(full
        ? [
            col.time<RatingEntry>({
              id: 'date',
              header: t('results.rating.columns.date'),
              value: (r) => r.run_at,
            }),
          ]
        : []),
    ],
    [t, unit, metricTitle, scope, slug, rows, newTab, full]
  )
  const shown = useMemo(
    () => (publicPage && scope === 'global' ? columns.filter((c) => c.id !== 'run') : columns),
    [columns, publicPage, scope]
  )
  const rowHref = (r: RatingEntry) =>
    scope === 'tenant'
      ? r.run_id
        ? `/t/${slug}/runs/${r.run_id}`
        : undefined
      : r.share_token
        ? `/s/${r.share_token}`
        : undefined
  const narrow = size.width > 0 && size.width < TABLE_FROM
  if (narrow && rows.length > 0 && !loading && !error) {
    const follow = (e: MouseEvent, href: string) => {
      // In-app rows stay in the SPA; an embed's open a tab (target), a
      // modified click is the browser's.
      if (newTab || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return
      e.preventDefault()
      router.history.push(href)
    }
    return (
      <div ref={box} className={styles.cards}>
        {rows.map((r) => {
          const href = rowHref(r)
          const topo = [
            topologyText(r.topology),
            r.node_count ? t('runs.list.nodes', { count: r.node_count }) : undefined,
          ]
            .filter(Boolean)
            .join(' · ')
          const workload = [
            r.workload_name ?? scenarioLabel(r.scenario),
            r.stroppy_version ? `stroppy ${r.stroppy_version}` : undefined,
          ]
            .filter(Boolean)
            .join(' · ')
          const where = [hardwareLabel(t, r.hardware), r.provider_kind, relativeTime(r.run_at)]
            .filter(Boolean)
            .join(' · ')
          return (
            <a
              key={`${r.scenario}-${r.rank}-${r.run_id ?? r.share_token ?? r.run_at}`}
              className={styles.card}
              href={href}
              target={newTab ? '_blank' : undefined}
              rel={newTab ? 'noreferrer' : undefined}
              onClick={href ? (e) => follow(e, href) : undefined}
            >
              <span className={styles.rank}>
                <RankBadge rank={r.rank} />
              </span>
              <span className={styles.db}>{dbLabel(r.db_kind, r.db_version)}</span>
              <span className={styles.value}>{formatMetric(r.value, unit ?? r.unit)}</span>
              {topo && <span className={styles.line}>{topo}</span>}
              <span className={styles.line}>{workload}</span>
              <span className={styles.line}>{where}</span>
              {!publicPage && scope === 'global' && r.tenant_name && (
                <span className={styles.line}>{r.tenant_name}</span>
              )}
            </a>
          )
        })}
      </div>
    )
  }
  return (
    <div ref={box}>
      <DataTable<RatingEntry>
        columns={shown}
        data={rows}
        getRowId={(r) => `${r.scenario}-${r.rank}-${r.run_id ?? r.share_token ?? r.run_at}`}
        clientSort
        loading={loading}
        error={error}
        onRetry={onRetry}
        empty={empty}
        filtered={filtered}
        onClearFilters={onClearFilters}
        rowHref={rowHref}
        // Embedded in another site the frame keeps the board: a row opens its
        // report in a new tab, the same as the link in its cell.
        onRowClick={
          newTab
            ? (r) => {
                const href = rowHref(r)
                if (href) window.open(href, '_blank', 'noopener')
              }
            : undefined
        }
      />
    </div>
  )
}
