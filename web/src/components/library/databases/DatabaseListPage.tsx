import { catalogQueries } from '@api/queries/catalog'
import { keys } from '@api/queries/keys'
import { type DatabaseListQuery, databaseQueries, libraryMutations } from '@api/queries/library'
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
import { RelativeTime } from '@components/RelativeTime'
import { Button, ConfirmModal, Stack } from '@grafana/ui'
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
  requirementsLine,
  TagChips,
  usageView,
} from '../shared/cells'
import { ImportModal } from '../shared/ImportModal'

type Database = Schemas['Database']

export const databaseListSearchSchema = z.object({
  q: z.string().optional().catch(undefined),
  name: z.string().optional().catch(undefined),
  kind: csvSchema,
  tags: z.string().optional().catch(undefined),
  author: csvSchema,
  favorites: z.boolean().optional().catch(undefined),
  sort: z
    .enum(['name', 'created_at', 'updated_at', 'kind', 'version', 'author'])
    .default('updated_at')
    .catch('updated_at'),
  order: orderSchema.default('desc').catch('desc'),
  size: sizeSchema,
  refresh: refreshSchema('off'),
})
export type DatabaseListSearch = z.infer<typeof databaseListSearchSchema>
export const DATABASE_LIST_DEFAULTS = {
  sort: 'updated_at',
  order: 'desc',
  size: 50,
  refresh: 'off',
} as const

export function toDatabaseListQuery(s: DatabaseListSearch): DatabaseListQuery {
  return {
    search: joinSearch(s.q, s.name),
    kind: s.kind as DatabaseListQuery['kind'],
    tags: s.tags,
    author: authorParam(s.author),
    favorites: s.favorites,
    sort: s.sort,
    order: s.order,
    limit: s.size,
  }
}

const FILTER_KEYS = ['q', 'name', 'kind', 'tags', 'author', 'favorites'] as const

export function DatabaseListPage({
  search,
  onSearchChange,
}: {
  search: DatabaseListSearch
  onSearchChange: (next: Partial<DatabaseListSearch>) => void
}) {
  const { t } = useTranslation()
  const { slug, can } = useTenant()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [importOpen, setImportOpen] = useState(false)
  const [deleting, setDeleting] = useState<Database | undefined>()
  const query = useMemo(() => toDatabaseListQuery(search), [search])
  const auto = useAutoRefresh(search.refresh)
  const list = useInfiniteQuery({
    ...databaseQueries.list(slug, query),
    placeholderData: keepPreviousData,
    refetchInterval: auto.refetchInterval,
  })
  const catalog = useQuery(catalogQueries.databases())
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
    mutationFn: (db: Database) => libraryMutations.cloneDatabase(slug, db.id),
    onSuccess: (created) => {
      void invalidate()
      toast.success(t('library.actions.cloned', { name: created.name }))
      void navigate({ to: '/t/$slug/library/databases/$id', params: { slug, id: created.id } })
    },
    onError,
  })
  const exportDoc = useMutation({
    mutationFn: async ({ db, format }: { db: Database; format: 'json' | 'yaml' }) => {
      const doc = await qc.fetchQuery(databaseQueries.export(slug, db.id))
      const name = safeFilename(db.name)
      if (format === 'json') downloadText(`${name}.json`, JSON.stringify(doc, null, 2))
      else downloadText(`${name}.yaml`, `${toYaml(doc)}\n`, 'application/yaml')
    },
    onError,
  })
  const favorite = useMutation({
    mutationFn: (db: Database) => runMutations.favorite(slug, 'database', db.id, !db.is_favorite),
    onSuccess: () => void invalidate(),
    onError,
  })
  const remove = useMutation({
    mutationFn: (db: Database) => libraryMutations.deleteDatabase(slug, db.id),
    onSuccess: (_r, db) => {
      void invalidate()
      toast.success(t('library.actions.deleted', { name: db.name }))
      setDeleting(undefined)
    },
    onError: (e) => {
      onError(e)
      setDeleting(undefined)
    },
  })

  const kindTitle = (kind: string) => catalog.data?.data.find((c) => c.kind === kind)?.title ?? kind

  const rowActions = (r: Database): RowAction[] => {
    const noPerm = t('runs.actions.noPermission')
    const usages = r.usages?.length ?? 0
    return [
      {
        key: 'open',
        label: t('common.actions.open'),
        icon: 'eye',
        onClick: () =>
          void navigate({ to: '/t/$slug/library/databases/$id', params: { slug, id: r.id } }),
      },
      {
        key: 'edit',
        label: t('common.actions.edit'),
        icon: 'edit',
        disabled: !canEdit,
        disabledReason: noPerm,
        onClick: () =>
          void navigate({
            to: '/t/$slug/library/databases/$id',
            params: { slug, id: r.id },
            search: { tab: 'params' } as never,
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
            search: { database: r.id } as never,
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
          copyToClipboard(`${window.location.origin}/t/${slug}/library/databases/${r.id}`, t),
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
        onClick: () => exportDoc.mutate({ db: r, format: 'json' }),
      },
      {
        key: 'exportYaml',
        label: t('library.actions.exportYaml'),
        icon: 'file-alt',
        onClick: () => exportDoc.mutate({ db: r, format: 'yaml' }),
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

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions/kindTitle are per-render closures; search/rows/catalog drive the recompute
  const columns = useMemo<DataTableColumn<Database>[]>(
    () => [
      col.identity<Database>({
        id: 'name',
        header: t('library.columns.dbShort'),
        sortKey: 'name',
        filter: {
          kind: 'text',
          value: search.name,
          placeholder: t('library.columns.name'),
          onChange: (v) => onSearchChange({ name: v }),
        },
        render: (r) => ({
          title: r.name,
          link: { to: '/t/$slug/library/databases/$id', params: { slug, id: r.id } },
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
      col.stack<Database>({
        id: 'engine',
        header: t('library.columns.engine'),
        width: 210,
        sortOptions: [
          { key: 'kind', label: t('library.sort.engine') },
          { key: 'version', label: t('library.sort.version') },
        ],
        filter: {
          kind: 'checklist',
          options: (catalog.data?.data ?? []).map((c) => ({
            value: c.kind,
            label: c.title,
            count: rows.filter((r) => r.kind === c.kind).length,
          })),
          value: search.kind,
          onChange: (v) => onSearchChange({ kind: v }),
        },
        value: (r) => r.kind,
        render: (r) => {
          const primary = `${kindTitle(r.kind)} ${r.version}`
          return {
            primary,
            secondary: r.image,
            title: [primary, r.image].filter(Boolean).join('\n'),
          }
        },
      }),
      col.stack<Database>({
        id: 'topology',
        header: t('library.columns.topology'),
        minWidth: 240,
        value: (r) => r.topology_preview?.label,
        render: (r) => {
          const tp = r.topology_preview
          const primary = tp
            ? [
                tp.label,
                tp.node_count ? t('library.topology.nodes', { count: tp.node_count }) : undefined,
              ]
                .filter(Boolean)
                .join(' · ')
            : undefined
          return {
            primary,
            secondary: requirementsLine(r.requirements),
            title: [primary, requirementsLine(r.requirements, true)].filter(Boolean).join('\n'),
          }
        },
      }),
      col.stack<Database>({
        id: 'usages',
        header: t('library.columns.usage'),
        width: 160,
        value: (r) => r.usages?.length ?? 0,
        render: (r) => {
          const u = usageView(r.usages, t)
          return { primary: u.primary, title: u.title ?? u.primary }
        },
      }),
      col.stack<Database>({
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
          secondary: r.author.display_name,
          title: r.author.display_name,
        }),
      }),
      col.actions<Database>({ title: (r) => r.name, actions: rowActions }),
    ],
    [t, slug, catalog.data, onSearchChange, search, rows, authors]
  )
  const prefs = useTablePrefs('databases', columns)

  const pill = (key: string, label: string, patch: Partial<DatabaseListSearch>): ActivePill => ({
    key,
    label,
    onRemove: () => onSearchChange(patch),
  })
  const pills: ActivePill[] = [
    ...(search.name
      ? [pill('name', `${t('library.columns.name')}: ${search.name}`, { name: undefined })]
      : []),
    ...(search.kind ?? []).map((v) =>
      pill(`kind:${v}`, `${t('library.sort.engine')}: ${kindTitle(v)}`, {
        kind: search.kind?.filter((x) => x !== v),
      })
    ),
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
      Object.fromEntries(FILTER_KEYS.map((k) => [k, undefined])) as Partial<DatabaseListSearch>
    )
  const filtered = pills.length > 0 || !!search.q || !!search.favorites

  return (
    <Page width="wide" fill>
      <PageHeader
        title={t('library.databases.title')}
        icon="database"
        subtitle={t('library.databases.subtitle')}
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
                void navigate({ to: '/t/$slug/library/databases/new', params: { slug } })
              }
            >
              {t('library.databases.new')}
            </Button>
          </>
        }
      />
      <DataTableToolbar
        search={search.q}
        onSearch={(v) => onSearchChange({ q: v || undefined })}
        searchPlaceholder={t('library.databases.search')}
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
      <DataTable<Database>
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
            sort: (s?.field as DatabaseListSearch['sort']) ?? 'updated_at',
            order: s?.order ?? 'desc',
          })
        }
        onOverlayChange={auto.setPaused}
        rowHref={(r) => `/t/${slug}/library/databases/${r.id}`}
        filtered={filtered}
        onClearFilters={clearAll}
        empty={{
          message: t('library.databases.empty.title'),
          button: (
            <Stack gap={1} justifyContent="center">
              <Button
                icon="plus"
                disabled={!canEdit}
                onClick={() =>
                  void navigate({ to: '/t/$slug/library/databases/new', params: { slug } })
                }
              >
                {t('library.databases.new')}
              </Button>
              <Button
                variant="secondary"
                icon="import"
                disabled={!canEdit}
                onClick={() => setImportOpen(true)}
              >
                {t('common.actions.import')}
              </Button>
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
      <ImportModal kind="database" isOpen={importOpen} onClose={() => setImportOpen(false)} />
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
