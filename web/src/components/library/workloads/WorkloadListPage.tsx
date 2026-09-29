import { catalogQueries } from '@api/queries/catalog'
import { keys } from '@api/queries/keys'
import { libraryMutations, type WorkloadListQuery, workloadQueries } from '@api/queries/library'
import { runMutations } from '@api/queries/runs'
import type { Schemas } from '@api/types'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import {
  authorParam,
  csvSchema,
  joinSearch,
  orderSchema,
  refreshSchema,
  sizeSchema,
} from '@components/DataTable/list-search'
import { useTablePrefs } from '@components/DataTable/prefs'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { TableSettings } from '@components/DataTable/TableSettings'
import { type ActivePill, DataTableToolbar } from '@components/DataTable/Toolbar'
import { useAutoRefresh } from '@components/DataTable/useAutoRefresh'
import { SeedExamplesButton } from '@components/examples/SeedExamplesButton'
import { RelativeTime } from '@components/RelativeTime'
import { UserLabel } from '@components/UserAvatar'
import { Button, ConfirmModal, Stack } from '@grafana/ui'
import { formatDuration } from '@helpers/format'
import { toYaml } from '@helpers/yaml'
import { useTenant } from '@hooks/useTenant'
import { downloadText, safeFilename } from '@lib/download'
import {
  keepPreviousData,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import {
  copyToClipboard,
  FavoriteMark,
  scriptsLine,
  segmentsCount,
  segmentsTitle,
  segmentViews,
  TagChips,
  totalDuration,
  usageView,
  vusText,
} from '../shared/cells'
import { ImportModal } from '../shared/ImportModal'

type Workload = Schemas['Workload']

const PROTOCOLS = ['pg', 'mysql', 'picodata', 'ydb_grpc', 'ydb_grpcs', 'cockroach', 'noop'] as const

export const workloadListSearchSchema = z.object({
  q: z.string().optional().catch(undefined),
  name: z.string().optional().catch(undefined),
  protocol: z.array(z.enum(PROTOCOLS)).optional().catch(undefined),
  version: csvSchema,
  script: z.string().optional().catch(undefined),
  tags: z.string().optional().catch(undefined),
  author: csvSchema,
  favorites: z.boolean().optional().catch(undefined),
  sort: z
    .enum([
      'name',
      'created_at',
      'updated_at',
      'protocol',
      'stroppy_version',
      'segments',
      'usages',
      'author',
    ])
    .default('updated_at')
    .catch('updated_at'),
  order: orderSchema.default('desc').catch('desc'),
  size: sizeSchema,
  refresh: refreshSchema('off'),
})
export type WorkloadListSearch = z.infer<typeof workloadListSearchSchema>
export const WORKLOAD_LIST_DEFAULTS = {
  sort: 'updated_at',
  order: 'desc',
  size: 50,
  refresh: 'off',
} as const

export function toWorkloadListQuery(s: WorkloadListSearch): WorkloadListQuery {
  return {
    search: joinSearch(s.q, s.name),
    protocol: s.protocol,
    stroppy_version: s.version,
    script: s.script,
    tags: s.tags,
    author: authorParam(s.author),
    favorites: s.favorites,
    sort: s.sort,
    order: s.order,
    limit: s.size,
  }
}

const FILTER_KEYS = [
  'q',
  'name',
  'protocol',
  'version',
  'script',
  'tags',
  'author',
  'favorites',
] as const

export interface SegmentSummary {
  name: string
  script?: string
  vus?: number
  limit?: string
  executor?: string
}

// Read the fields lists show out of a `workload.segment` value.
export function summarizeSegment(s: Record<string, unknown>): SegmentSummary {
  const w = s.workload as { script?: string } | undefined
  const r = s.run as
    | { executor?: string; vus?: number; duration?: string; iterations?: number }
    | undefined
  return {
    name: String(s.name ?? ''),
    script: w?.script,
    vus: r?.vus,
    executor: r?.executor,
    limit: r?.duration ?? (r?.iterations !== undefined ? `${r.iterations} it` : undefined),
  }
}

export function WorkloadListPage({
  search,
  onSearchChange,
}: {
  search: WorkloadListSearch
  onSearchChange: (next: Partial<WorkloadListSearch>) => void
}) {
  const { t } = useTranslation()
  const { slug, can } = useTenant()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [importOpen, setImportOpen] = useState(false)
  const [deleting, setDeleting] = useState<Workload | undefined>()
  const query = useMemo(() => toWorkloadListQuery(search), [search])
  const auto = useAutoRefresh(search.refresh)
  const list = useInfiniteQuery({
    ...workloadQueries.list(slug, query),
    placeholderData: keepPreviousData,
    refetchInterval: auto.refetchInterval,
  })
  const stroppy = useQuery(catalogQueries.stroppy())
  const rows = useMemo(() => list.data?.pages.flatMap((p) => p.data) ?? [], [list.data])
  const authors = useMemo(
    () => [...new Map(rows.map((r) => [r.author.id, r.author])).values()],
    [rows]
  )
  const canEdit = can('edit-library')
  const canRun = can('run')
  const invalidate = () => qc.invalidateQueries({ queryKey: keys.t(slug) })
  const onError = (e: unknown) => toast.error(e)

  const clone = useMutation({
    mutationFn: (wl: Workload) => libraryMutations.cloneWorkload(slug, wl.id),
    onSuccess: (created) => {
      void invalidate()
      toast.success(t('library.actions.cloned', { name: created.name }))
      void navigate({ to: '/t/$slug/library/workloads/$id', params: { slug, id: created.id } })
    },
    onError,
  })
  const exportDoc = useMutation({
    mutationFn: async ({ wl, format }: { wl: Workload; format: 'json' | 'yaml' }) => {
      const doc = await qc.fetchQuery(workloadQueries.export(slug, wl.id))
      const name = safeFilename(wl.name)
      if (format === 'json') downloadText(`${name}.json`, JSON.stringify(doc, null, 2))
      else downloadText(`${name}.yaml`, `${toYaml(doc)}\n`, 'application/yaml')
    },
    onError,
  })
  const favorite = useMutation({
    mutationFn: (wl: Workload) => runMutations.favorite(slug, 'workload', wl.id, !wl.is_favorite),
    onSuccess: () => void invalidate(),
    onError,
  })
  const remove = useMutation({
    mutationFn: (wl: Workload) => libraryMutations.deleteWorkload(slug, wl.id),
    onSuccess: (_r, wl) => {
      void invalidate()
      toast.success(t('library.actions.deleted', { name: wl.name }))
      setDeleting(undefined)
    },
    onError: (e) => {
      onError(e)
      setDeleting(undefined)
    },
  })

  const rowActions = (r: Workload): RowAction[] => {
    const noPerm = t('runs.actions.noPermission')
    const usages = r.usages?.length ?? 0
    return [
      {
        key: 'open',
        label: t('common.actions.open'),
        icon: 'eye',
        onClick: () =>
          void navigate({ to: '/t/$slug/library/workloads/$id', params: { slug, id: r.id } }),
      },
      {
        key: 'edit',
        label: t('common.actions.edit'),
        icon: 'edit',
        disabled: !canEdit,
        disabledReason: noPerm,
        onClick: () =>
          void navigate({
            to: '/t/$slug/library/workloads/$id',
            params: { slug, id: r.id },
            search: { tab: 'segments' } as never,
          }),
      },
      {
        key: 'duplicate',
        label: t('common.actions.clone'),
        icon: 'copy',
        disabled: !canEdit || clone.isPending,
        disabledReason: noPerm,
        onClick: () => clone.mutate(r),
      },
      {
        key: 'newTest',
        label: t('library.actions.newTestFrom'),
        icon: 'vial',
        disabled: !canEdit && !canRun,
        disabledReason: noPerm,
        onClick: () =>
          void navigate({
            to: '/t/$slug/library/tests/new',
            params: { slug },
            search: { workload: r.id } as never,
          }),
      },
      {
        key: 'favorite',
        group: true,
        label: r.is_favorite ? t('common.actions.unfavorite') : t('common.actions.favorite'),
        icon: r.is_favorite ? 'favorite' : 'star',
        disabled: favorite.isPending,
        onClick: () => favorite.mutate(r),
      },
      {
        key: 'copyLink',
        label: t('library.actions.copyLink'),
        icon: 'link',
        onClick: () =>
          copyToClipboard(`${window.location.origin}/t/${slug}/library/workloads/${r.id}`, t),
      },
      {
        key: 'copyId',
        label: t('library.actions.copyId'),
        icon: 'clipboard-alt',
        onClick: () => copyToClipboard(r.id, t),
      },
      {
        key: 'exportJson',
        group: true,
        label: t('library.actions.exportJson'),
        icon: 'download-alt',
        onClick: () => exportDoc.mutate({ wl: r, format: 'json' }),
      },
      {
        key: 'exportYaml',
        label: t('library.actions.exportYaml'),
        icon: 'file-alt',
        onClick: () => exportDoc.mutate({ wl: r, format: 'yaml' }),
      },
      {
        key: 'delete',
        group: true,
        label: t('common.actions.delete'),
        icon: 'trash-alt',
        destructive: true,
        disabled: !canEdit || usages > 0,
        disabledReason: !canEdit ? noPerm : t('library.actions.inUseShort', { count: usages }),
        onClick: () => setDeleting(r),
      },
    ]
  }

  // Protocol and stroppy version share one checklist (one filter per column): values are
  // prefixed `p:` / `v:` and split back into the two server params.
  const protocolFilterValue = [
    ...(search.protocol ?? []).map((p) => `p:${p}`),
    ...(search.version ?? []).map((v) => `v:${v}`),
  ]

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure; search/rows/stroppy drive the recompute
  const columns = useMemo<DataTableColumn<Workload>[]>(
    () => [
      col.identity<Workload>({
        id: 'name',
        header: t('library.columns.workload'),
        sortKey: 'name',
        filter: {
          kind: 'text',
          value: search.name,
          placeholder: t('library.columns.name'),
          onChange: (v) => onSearchChange({ name: v }),
        },
        render: (r) => ({
          title: r.name,
          link: { to: '/t/$slug/library/workloads/$id', params: { slug, id: r.id } },
          subtitle:
            r.tags && Object.keys(r.tags).length ? (
              <>
                <TagChips tags={r.tags} onPick={(tag) => onSearchChange({ tags: tag })} />
                <span title={r.description}>{r.description}</span>
              </>
            ) : (
              r.description
            ),
          badges: r.is_favorite ? <FavoriteMark /> : undefined,
        }),
      }),
      col.stack<Workload>({
        id: 'protocol',
        header: t('library.columns.protocol'),
        width: 180,
        sortOptions: [
          { key: 'protocol', label: t('library.sort.protocol') },
          { key: 'stroppy_version', label: t('library.sort.stroppy') },
        ],
        filter: {
          kind: 'checklist',
          options: [
            ...PROTOCOLS.map((p) => ({
              value: `p:${p}`,
              label: p,
              count: rows.filter((r) => r.protocol === p).length,
            })),
            ...(stroppy.data?.versions ?? []).map((v) => ({
              value: `v:${v.version}`,
              label: `stroppy ${v.version}`,
              count: rows.filter((r) => r.stroppy_version === v.version).length,
            })),
          ],
          value: protocolFilterValue.length ? protocolFilterValue : undefined,
          onChange: (v) => {
            const protocol = (v ?? []).filter((x) => x.startsWith('p:')).map((x) => x.slice(2))
            const version = (v ?? []).filter((x) => x.startsWith('v:')).map((x) => x.slice(2))
            onSearchChange({
              protocol: protocol.length ? (protocol as WorkloadListSearch['protocol']) : undefined,
              version: version.length ? version : undefined,
            })
          },
        },
        value: (r) => r.protocol,
        render: (r) => ({
          primary: r.protocol,
          secondary: `stroppy ${r.stroppy_version}`,
          title: `${r.protocol}\nstroppy ${r.stroppy_version}`,
        }),
      }),
      col.stack<Workload>({
        id: 'segments',
        header: t('library.columns.segments'),
        minWidth: 240,
        sortKey: 'segments',
        filter: {
          kind: 'text',
          value: search.script,
          placeholder: t('library.columns.script'),
          onChange: (v) => onSearchChange({ script: v }),
        },
        value: (r) => r.segments.length,
        render: (r) => {
          const segs = segmentViews(r.segments)
          const total = totalDuration(segs)
          return {
            primary: scriptsLine(segs),
            secondary: [
              segmentsCount(segs, t),
              vusText(segs, t),
              total !== undefined ? formatDuration(total) : undefined,
            ]
              .filter(Boolean)
              .join(' · '),
            title: segmentsTitle(segs),
          }
        },
      }),
      col.stack<Workload>({
        id: 'runner',
        header: t('library.columns.runner'),
        width: 170,
        defaultHidden: true,
        value: (r) => r.requirements?.runner?.cpu,
        render: (r) => {
          const run = r.requirements?.runner
          if (!run) return { primary: undefined }
          const primary = [
            run.cpu !== undefined ? `${run.cpu} CPU` : undefined,
            run.memory_gb ? `${run.memory_gb} GB` : undefined,
          ]
            .filter(Boolean)
            .join(' · ')
          return {
            primary: primary || undefined,
            secondary: run.reason,
            title: [primary, run.reason].filter(Boolean).join('\n'),
          }
        },
      }),
      col.stack<Workload>({
        id: 'usages',
        header: t('library.columns.usage'),
        width: 160,
        sortKey: 'usages',
        value: (r) => r.usages?.length ?? 0,
        render: (r) => {
          const u = usageView(r.usages, t)
          return { primary: u.primary, title: u.title ?? u.primary }
        },
      }),
      col.stack<Workload>({
        id: 'changed',
        header: t('library.columns.changed'),
        width: 170,
        sortOptions: [
          { key: 'updated_at', label: t('library.sort.updated') },
          { key: 'created_at', label: t('library.sort.created') },
          { key: 'author', label: t('library.sort.author') },
        ],
        filter: {
          kind: 'checklist',
          options: authors.map((a) => ({
            value: a.id,
            label: a.display_name ?? a.id,
            count: rows.filter((r) => r.author.id === a.id).length,
          })),
          value: search.author,
          single: true,
          onChange: (v) => onSearchChange({ author: v }),
        },
        value: (r) => r.updated_at,
        render: (r) => ({
          primary: <RelativeTime value={r.updated_at} />,
          secondary: <UserLabel user={r.author} />,
          title: r.author.display_name,
        }),
      }),
      col.actions<Workload>({ title: (r) => r.name, actions: rowActions }),
    ],
    [t, slug, onSearchChange, search, rows, authors, stroppy.data]
  )
  const prefs = useTablePrefs('workloads', columns)

  const pill = (key: string, label: string, patch: Partial<WorkloadListSearch>): ActivePill => ({
    key,
    label,
    onRemove: () => onSearchChange(patch),
  })
  const pills: ActivePill[] = [
    ...(search.name
      ? [pill('name', `${t('library.columns.name')}: ${search.name}`, { name: undefined })]
      : []),
    ...(search.protocol ?? []).map((v) =>
      pill(`p:${v}`, `${t('library.columns.protocol')}: ${v}`, {
        protocol: search.protocol?.filter((x) => x !== v),
      })
    ),
    ...(search.version ?? []).map((v) =>
      pill(`v:${v}`, `stroppy ${v}`, { version: search.version?.filter((x) => x !== v) })
    ),
    ...(search.script
      ? [pill('script', `${t('library.columns.script')}: ${search.script}`, { script: undefined })]
      : []),
    ...(search.tags
      ? [pill('tags', `${t('library.columns.tags')}: ${search.tags}`, { tags: undefined })]
      : []),
    ...(search.author ?? []).map((v) =>
      pill(
        `author:${v}`,
        `${t('library.columns.author')}: ${authors.find((a) => a.id === v)?.display_name ?? v}`,
        { author: search.author?.filter((x) => x !== v) }
      )
    ),
  ]
  const clearAll = () =>
    onSearchChange(
      Object.fromEntries(FILTER_KEYS.map((k) => [k, undefined])) as Partial<WorkloadListSearch>
    )
  const filtered = pills.length > 0 || !!search.q || !!search.favorites

  return (
    <Page width="wide" fill>
      <PageHeader
        title={t('library.workloads.title')}
        icon="bolt"
        subtitle={t('library.workloads.subtitle')}
        actions={
          <>
            <Button
              variant="secondary"
              icon="import"
              onClick={() => setImportOpen(true)}
              disabled={!canEdit}
            >
              {t('common.actions.import')}
            </Button>
            <Button
              icon="plus"
              disabled={!canEdit}
              onClick={() =>
                void navigate({ to: '/t/$slug/library/workloads/new', params: { slug } })
              }
            >
              {t('library.workloads.new')}
            </Button>
          </>
        }
      />
      <DataTableToolbar
        search={search.q}
        onSearch={(v) => onSearchChange({ q: v || undefined })}
        searchPlaceholder={t('library.workloads.search')}
        toggles={[
          {
            key: 'fav',
            label: t('library.filters.favorites'),
            value: !!search.favorites,
            onChange: (v) => onSearchChange({ favorites: v || undefined }),
          },
        ]}
        pills={pills}
        onClearAll={pills.length ? clearAll : undefined}
        count={rows.length}
        size={search.size}
        onSizeChange={(size) => onSearchChange({ size })}
        refresh={{
          interval: search.refresh,
          onIntervalChange: (refresh) => onSearchChange({ refresh }),
          onRefresh: () => void list.refetch(),
          isFetching: list.isFetching,
          paused: auto.paused,
        }}
        settings={<TableSettings prefs={prefs} />}
      />
      <DataTable<Workload>
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
            sort: (s?.field as WorkloadListSearch['sort']) ?? 'updated_at',
            order: s?.order ?? 'desc',
          })
        }
        onOverlayChange={auto.setPaused}
        rowHref={(r) => `/t/${slug}/library/workloads/${r.id}`}
        filtered={filtered}
        onClearFilters={clearAll}
        empty={{
          message: t('library.workloads.empty.title'),
          button: (
            <Stack gap={1} justifyContent="center">
              <Button
                icon="plus"
                disabled={!canEdit}
                onClick={() =>
                  void navigate({ to: '/t/$slug/library/workloads/new', params: { slug } })
                }
              >
                {t('library.workloads.new')}
              </Button>
              <Button
                variant="secondary"
                icon="import"
                disabled={!canEdit}
                onClick={() => setImportOpen(true)}
              >
                {t('common.actions.import')}
              </Button>
              <SeedExamplesButton slug={slug} kind="workload" disabled={!canEdit} />
            </Stack>
          ),
        }}
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
      <ImportModal kind="workload" isOpen={importOpen} onClose={() => setImportOpen(false)} />
      <ConfirmModal
        isOpen={!!deleting}
        title={t('common.confirm.deleteTitle', { name: deleting?.name ?? '' })}
        body={t('common.confirm.deleteBody')}
        confirmText={t('common.confirm.yesDelete')}
        confirmButtonVariant="destructive"
        dismissText={t('common.actions.cancel')}
        disabled={remove.isPending}
        onConfirm={() => {
          if (deleting) remove.mutate(deleting)
        }}
        onDismiss={() => setDeleting(undefined)}
      />
    </Page>
  )
}
