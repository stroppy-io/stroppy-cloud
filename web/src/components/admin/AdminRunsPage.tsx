import {
  type AdminRun,
  type AdminRunsQuery,
  adminMutations,
  adminQueries,
} from '@api/queries/admin'
import { PageFill } from '@app/Page'
import { toast } from '@app/Toaster'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import { orderSchema, refreshSchema } from '@components/DataTable/list-search'
import { useTablePrefs } from '@components/DataTable/prefs'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { TableSettings } from '@components/DataTable/TableSettings'
import { type ActivePill, DataTableToolbar } from '@components/DataTable/Toolbar'
import { useAutoRefresh } from '@components/DataTable/useAutoRefresh'
import {
  databaseView,
  RunMetricsCell,
  RunTimeCell,
  TriggerMark,
  workloadView,
} from '@components/runs/list/RunCells'
import { css } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import { Badge, Button, ConfirmModal, Stack, Text, useStyles2 } from '@grafana/ui'
import { runStageProgress } from '@helpers/run-status'
import { useCopy } from '@hooks/useCopy'
import {
  keepPreviousData,
  useInfiniteQuery,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

const STATUSES = ['pending', 'running', 'cancelling', 'completed', 'failed', 'cancelled'] as const

// Server sort keys of adminListRuns: the listRuns set plus `tenant`; the endpoint defaults to
// created_at.
const ADMIN_RUN_SORT_KEYS = [
  'default',
  'started_at',
  'finished_at',
  'duration',
  'tps',
  'qps',
  'p50',
  'p99',
  'errors',
  'status',
  'name',
  'db_kind',
  'workload',
  'topology',
  'provider',
  'trigger',
  'author',
  'tenant',
  'created_at',
  'updated_at',
] as const satisfies readonly NonNullable<AdminRunsQuery['sort']>[]

export const adminRunsSearchSchema = z.object({
  status: z.array(z.enum(STATUSES)).optional().catch(undefined),
  tenant: z.string().optional().catch(undefined),
  sort: z.enum(ADMIN_RUN_SORT_KEYS).default('created_at').catch('created_at'),
  order: orderSchema.default('desc').catch('desc'),
  refresh: refreshSchema('5s'),
})
export type AdminRunsSearch = z.infer<typeof adminRunsSearchSchema>
export const ADMIN_RUNS_DEFAULTS = { sort: 'created_at', order: 'desc', refresh: '5s' } as const

const TRIGGER_ICON: Record<AdminRun['trigger'], IconName> = {
  manual: 'user',
  suite: 'layer-group',
  api: 'brackets-curly',
  schedule: 'clock-nine',
}

const getStyles = (theme: GrafanaTheme2) => ({ root: css({ gap: theme.spacing(1) }) })

// The platform-wide run queue (admin). Same cells and server sort keys as the tenant runs list
// plus the tenant.
export function AdminRunsPage({
  search,
  onSearchChange,
}: {
  search: AdminRunsSearch
  onSearchChange: (next: Partial<AdminRunsSearch>) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const qc = useQueryClient()
  const navigate = useNavigate()
  const copy = useCopy()
  const auto = useAutoRefresh(search.refresh)
  const list = useInfiniteQuery({
    ...adminQueries.runs({
      status: search.status,
      tenant: search.tenant,
      sort: search.sort,
      order: search.order,
    }),
    placeholderData: keepPreviousData,
    refetchInterval: auto.refetchInterval,
  })
  const tenants = useInfiniteQuery(adminQueries.tenants())
  const rows = useMemo(() => list.data?.pages.flatMap((p) => p.data) ?? [], [list.data])
  const [cancelling, setCancelling] = useState<AdminRun>()
  const cancel = useMutation({
    mutationFn: (id: string) => adminMutations.cancelRun(id),
    onSuccess: async (r) => {
      await qc.invalidateQueries({ queryKey: ['admin', 'runs'] })
      toast.success(t('admin.runs.cancelled', { name: r.name }))
      setCancelling(undefined)
    },
    onError: (e) => toast.error(e),
  })
  const tenantList = useMemo(() => tenants.data?.pages.flatMap((p) => p.data) ?? [], [tenants.data])
  const tenantSlugs = useMemo(() => new Map(tenantList.map((x) => [x.id, x.slug])), [tenantList])
  const slugOf = (r: AdminRun) => (r.tenant ? tenantSlugs.get(r.tenant.id) : undefined)

  const rowActions = (r: AdminRun): RowAction[] => {
    const slug = slugOf(r)
    const live = r.status === 'running' || r.status === 'pending' || r.status === 'cancelling'
    const noSlug = t('admin.runs.tenantUnknown')
    const open = (tab?: 'logs' | 'metrics') =>
      slug &&
      void navigate({
        to: tab ? `/t/$slug/runs/$id/${tab}` : '/t/$slug/runs/$id',
        params: { slug, id: r.id },
      } as never)
    return [
      {
        key: 'open',
        label: t('common.actions.open'),
        icon: 'eye',
        disabled: !slug,
        disabledReason: noSlug,
        onClick: () => open(),
      },
      {
        key: 'logs',
        label: t('runs.tabs.logs'),
        icon: 'document-info',
        disabled: !slug,
        disabledReason: noSlug,
        onClick: () => open('logs'),
      },
      {
        key: 'metrics',
        label: t('runs.tabs.metrics'),
        icon: 'chart-line',
        disabled: !slug,
        disabledReason: noSlug,
        onClick: () => open('metrics'),
      },
      {
        key: 'tenant',
        label: t('admin.tenants.open'),
        description: r.tenant?.name,
        icon: 'building',
        disabled: !slug,
        disabledReason: noSlug,
        onClick: () => slug && void navigate({ to: '/t/$slug', params: { slug } }),
      },
      {
        key: 'copyLink',
        group: true,
        label: t('admin.table.copyLink'),
        icon: 'link',
        disabled: !slug,
        disabledReason: noSlug,
        onClick: () => copy(`${window.location.origin}/t/${slug}/runs/${r.id}`),
      },
      {
        key: 'copyId',
        label: t('admin.table.copyId'),
        icon: 'copy',
        onClick: () => copy(r.id),
      },
      {
        key: 'cancel',
        group: true,
        label: t('admin.runs.cancel'),
        icon: 'times',
        destructive: true,
        disabled: !live || cancel.isPending,
        disabledReason: t('admin.runs.notLive'),
        onClick: () => setCancelling(r),
      },
    ]
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions/slugOf are per-render closures over tenantSlugs + cancel state
  const columns = useMemo<DataTableColumn<AdminRun>[]>(
    () => [
      col.status<AdminRun>({
        id: 'status',
        title: t('common.fields.status'),
        status: (r) => r.status,
        sortKey: 'status',
        filter: {
          kind: 'checklist',
          options: STATUSES.map((s) => ({ value: s, label: t(`common.status.${s}`) })),
          value: search.status,
          onChange: (v) => onSearchChange({ status: v as AdminRunsSearch['status'] }),
        },
      }),
      col.identity<AdminRun>({
        id: 'name',
        header: t('admin.runs.run'),
        sortKey: 'name',
        render: (r) => {
          const slug = slugOf(r)
          const running = r.status === 'running' || r.status === 'cancelling'
          return {
            title: r.name,
            link: slug ? { to: '/t/$slug/runs/$id', params: { slug, id: r.id } } : undefined,
            lead:
              r.trigger !== 'manual' ? (
                <TriggerMark trigger={r.trigger} icon={TRIGGER_ICON[r.trigger]} />
              ) : undefined,
            subtitle: running
              ? [
                  t(`common.phase.${r.phase}`),
                  r.summary?.segment ? t('runs.segment', { name: r.summary.segment }) : undefined,
                  t('runs.progress', { pct: r.summary?.phase_progress_pct ?? 0 }),
                ]
                  .filter(Boolean)
                  .join(' · ')
              : r.status_reason || r.test_ref.name,
            progress: running ? runStageProgress(r, t) : undefined,
            badges: r.stand_kept ? <Badge text="keep" color="purple" icon="lock" /> : undefined,
          }
        },
      }),
      col.link<AdminRun>({
        id: 'tenant',
        header: t('common.misc.tenant'),
        icon: 'building',
        width: 180,
        sortKey: 'tenant',
        filter: {
          kind: 'checklist',
          single: true,
          options: tenantList.map((x) => ({ value: x.slug, label: x.name })),
          value: search.tenant ? [search.tenant] : undefined,
          onChange: (v) => onSearchChange({ tenant: v?.[0] }),
        },
        render: (r) => {
          const slug = slugOf(r)
          return {
            text: r.tenant?.name ?? slug,
            link: slug ? { to: '/t/$slug', params: { slug } } : undefined,
          }
        },
      }),
      col.stack<AdminRun>({
        id: 'database',
        header: t('runs.columns.database'),
        minWidth: 200,
        sortOptions: [
          { key: 'db_kind', label: t('runs.sort.engine') },
          { key: 'topology', label: t('runs.sort.topology') },
        ],
        render: (r) => databaseView(r, t),
      }),
      col.stack<AdminRun>({
        id: 'workload',
        header: t('runs.columns.workload'),
        minWidth: 220,
        defaultHidden: true,
        sortKey: 'workload',
        render: (r) => workloadView(r, t),
      }),
      col.custom<AdminRun>({
        id: 'metrics',
        header: t('runs.columns.metrics'),
        width: 290,
        defaultHidden: true,
        sortOptions: [
          { key: 'qps', label: 'QPS' },
          { key: 'p99', label: 'p99' },
          { key: 'p50', label: 'p50' },
          { key: 'errors', label: t('runs.sort.errors') },
        ],
        cell: (r) => <RunMetricsCell run={r} />,
      }),
      col.custom<AdminRun>({
        id: 'time',
        header: t('runs.columns.time'),
        width: 180,
        sortOptions: [
          { key: 'started_at', label: t('runs.sort.started') },
          { key: 'finished_at', label: t('runs.sort.finished') },
          { key: 'duration', label: t('runs.sort.duration') },
        ],
        cell: (r) => <RunTimeCell run={r} />,
      }),
      col.text<AdminRun>({
        id: 'author',
        header: t('common.fields.author'),
        width: 170,
        sortKey: 'author',
        value: (r) => r.author.display_name,
      }),
      col.actions<AdminRun>({ title: (r) => r.name, actions: rowActions }),
    ],
    [t, search, onSearchChange, tenantList, tenantSlugs, cancel.isPending]
  )
  const prefs = useTablePrefs('admin-runs', columns)

  const pills: ActivePill[] = [
    ...(search.status ?? []).map((s) => ({
      key: `s:${s}`,
      label: `${t('common.fields.status')}: ${t(`common.status.${s}`)}`,
      onRemove: () => onSearchChange({ status: search.status?.filter((x) => x !== s) }),
    })),
    ...(search.tenant
      ? [
          {
            key: 'tenant',
            label: `${t('common.misc.tenant')}: ${tenantList.find((x) => x.slug === search.tenant)?.name ?? search.tenant}`,
            onRemove: () => onSearchChange({ tenant: undefined }),
          },
        ]
      : []),
  ]
  const clearAll = () => onSearchChange({ status: undefined, tenant: undefined })

  return (
    <PageFill className={styles.root}>
      <Text color="secondary" variant="bodySmall">
        {t('admin.runs.hint')}
      </Text>
      <DataTableToolbar
        pills={pills}
        onClearAll={pills.length ? clearAll : undefined}
        count={rows.length}
        refresh={{
          interval: search.refresh,
          onIntervalChange: (refresh) => onSearchChange({ refresh }),
          onRefresh: () => void list.refetch(),
          isFetching: list.isFetching,
          paused: auto.paused,
        }}
        settings={<TableSettings prefs={prefs} />}
      />
      <DataTable<AdminRun>
        fill
        columns={prefs.visible}
        density={prefs.density}
        data={rows}
        getRowId={(r) => r.id}
        loading={list.isPending}
        error={list.isError ? list.error : undefined}
        onRetry={() => void list.refetch()}
        sort={{ field: search.sort, order: search.order }}
        onSortChange={(s) =>
          onSearchChange({
            sort: (s?.field as AdminRunsSearch['sort']) ?? 'created_at',
            order: s?.order ?? 'desc',
          })
        }
        onOverlayChange={auto.setPaused}
        rowHref={(r) => {
          const slug = slugOf(r)
          return slug ? `/t/${slug}/runs/${r.id}` : undefined
        }}
        filtered={pills.length > 0}
        onClearFilters={clearAll}
        empty={{ message: t('admin.runs.empty') }}
        footer={
          list.hasNextPage ? (
            <Stack justifyContent="space-between" alignItems="center">
              <span>{t('common.misc.showing', { count: rows.length })}</span>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => void list.fetchNextPage()}
                disabled={list.isFetchingNextPage}
              >
                {t('common.actions.loadMore')}
              </Button>
            </Stack>
          ) : undefined
        }
      />
      <ConfirmModal
        isOpen={!!cancelling}
        title={t('admin.runs.cancelTitle', { name: cancelling?.name ?? '' })}
        body={t('admin.runs.cancelBody', { tenant: cancelling?.tenant?.name ?? '' })}
        confirmText={t('admin.runs.cancel')}
        confirmButtonVariant="destructive"
        dismissText={t('common.actions.cancel')}
        disabled={cancel.isPending}
        onConfirm={() => cancelling && cancel.mutate(cancelling.id)}
        onDismiss={() => setCancelling(undefined)}
      />
    </PageFill>
  )
}
