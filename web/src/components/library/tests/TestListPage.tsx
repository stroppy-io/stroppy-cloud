import { catalogQueries } from '@api/queries/catalog'
import { keys } from '@api/queries/keys'
import { libraryMutations, type TestListQuery, testQueries } from '@api/queries/library'
import { runMutations } from '@api/queries/runs'
import { providerQueries } from '@api/queries/settings'
import type { Schemas } from '@api/types'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import type { CellLink } from '@components/DataTable/cells'
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
import { UserLabel } from '@components/UserAvatar'
import { Badge, Button, ConfirmModal, Stack } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import {
  keepPreviousData,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import type { TFunction } from 'i18next'
import { useCallback, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import {
  copyToClipboard,
  FavoriteMark,
  StaleMark,
  scriptsLine,
  segmentsCount,
  segmentsTitle,
  segmentViews,
  TagChips,
  vusText,
} from '../shared/cells'
import { ImportModal } from '../shared/ImportModal'
import { LaunchDrawer } from './LaunchDrawer'
import { TestLastRunCell } from './TestCells'

type Test = Schemas['Test']

const TEST_STATUSES = ['ready', 'needs_attention', 'draft'] as const

export const testListSearchSchema = z.object({
  q: z.string().optional().catch(undefined),
  name: z.string().optional().catch(undefined),
  status: z.array(z.enum(TEST_STATUSES)).optional().catch(undefined),
  kind: csvSchema,
  tags: z.string().optional().catch(undefined),
  author: csvSchema,
  favorites: z.boolean().optional().catch(undefined),
  sort: z
    .enum([
      'name',
      'created_at',
      'updated_at',
      'kind',
      'database',
      'workload',
      'provider',
      'status',
      'author',
      'last_run_at',
    ])
    .default('updated_at')
    .catch('updated_at'),
  order: orderSchema.default('desc').catch('desc'),
  size: sizeSchema,
  refresh: refreshSchema('off'),
})
export type TestListSearch = z.infer<typeof testListSearchSchema>
export const TEST_LIST_DEFAULTS = {
  sort: 'updated_at',
  order: 'desc',
  size: 50,
  refresh: 'off',
} as const

export function toTestListQuery(s: TestListSearch): TestListQuery {
  return {
    search: joinSearch(s.q, s.name),
    status: s.status,
    kind: s.kind as TestListQuery['kind'],
    tags: s.tags,
    author: authorParam(s.author),
    favorites: s.favorites,
    sort: s.sort,
    order: s.order,
    limit: s.size,
  }
}

const FILTER_KEYS = ['q', 'name', 'status', 'kind', 'tags', 'author', 'favorites'] as const

// «База»: engine + version over topology · N nodes. The library record name (when the test
// references one) is in the tooltip.
function databaseView(r: Test, t: TFunction) {
  const db = r.resolved?.database
  const inline = r.database && 'inline' in r.database ? r.database.inline : undefined
  const kind = r.summary?.db_kind ?? db?.kind ?? inline?.kind
  if (!kind) return { primary: undefined }
  const version = r.summary?.db_version ?? db?.version ?? inline?.version
  const primary = `${kind}${version ? ` ${version}` : ''}`
  const nodes = r.summary?.node_count ?? db?.topology_preview?.node_count
  const topo = [
    r.summary?.topology_label ?? db?.topology_preview?.label,
    nodes ? t('library.topology.nodes', { count: nodes }) : undefined,
  ]
    .filter(Boolean)
    .join(' · ')
  const stale = !!r.validation?.stale?.database
  const refName = r.database && 'ref' in r.database ? db?.name : undefined
  return {
    primary,
    secondary:
      topo || stale ? (
        <>
          {topo}
          {topo && stale ? ' · ' : ''}
          {stale && <StaleMark />}
        </>
      ) : undefined,
    title: [refName, primary, topo, stale ? t('library.tests.staleDatabase') : undefined]
      .filter(Boolean)
      .join('\n'),
  }
}

// «Нагрузка»: main script (+N) over protocol · stroppy X · N segments · VU; every segment in
// the tooltip.
function workloadView(r: Test, t: TFunction) {
  const wl =
    r.resolved?.workload ?? (r.workload && 'inline' in r.workload ? r.workload.inline : undefined)
  const segs = segmentViews(wl?.segments)
  const name = r.workload && 'ref' in r.workload ? r.resolved?.workload?.name : undefined
  const primary = scriptsLine(segs) ?? name
  if (!primary) return { primary: undefined }
  const protocol = r.summary?.protocol ?? wl?.protocol
  const stroppy = r.summary?.stroppy_version ?? wl?.stroppy_version
  const line = [
    protocol,
    stroppy ? `stroppy ${stroppy}` : undefined,
    segmentsCount(segs, t),
    vusText(segs, t),
  ]
    .filter(Boolean)
    .join(' · ')
  const stale = !!r.validation?.stale?.workload
  return {
    primary,
    secondary:
      line || stale ? (
        <>
          {line}
          {line && stale ? ' · ' : ''}
          {stale && <StaleMark />}
        </>
      ) : undefined,
    title: [name, line, segmentsTitle(segs), stale ? t('library.tests.staleWorkload') : undefined]
      .filter(Boolean)
      .join('\n'),
  }
}

type Dialog = { kind: 'none' } | { kind: 'launch' | 'delete'; test: Test }

export function TestListPage({
  search,
  onSearchChange,
}: {
  search: TestListSearch
  onSearchChange: (next: Partial<TestListSearch>) => void
}) {
  const { t } = useTranslation()
  const { slug, can } = useTenant()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [importOpen, setImportOpen] = useState(false)
  const [dialog, setDialog] = useState<Dialog>({ kind: 'none' })
  const query = useMemo(() => toTestListQuery(search), [search])
  const auto = useAutoRefresh(search.refresh)
  const list = useInfiniteQuery({
    ...testQueries.list(slug, query),
    placeholderData: keepPreviousData,
    refetchInterval: auto.refetchInterval,
  })
  const catalog = useQuery(catalogQueries.databases())
  const providers = useQuery(providerQueries.list(slug))
  const rows = useMemo(() => list.data?.pages.flatMap((p) => p.data) ?? [], [list.data])
  const authors = useMemo(
    () => [...new Map(rows.map((r) => [r.author.id, r.author])).values()],
    [rows]
  )
  const canEdit = can('edit-library')
  const canRun = can('run')
  const providerName = useCallback(
    (id: string | null | undefined) => providers.data?.data.find((p) => p.id === id)?.name,
    [providers.data]
  )
  const invalidate = () => qc.invalidateQueries({ queryKey: keys.t(slug) })
  const onError = (e: unknown) => toast.error(e)

  const clone = useMutation({
    mutationFn: (test: Test) => libraryMutations.cloneTest(slug, test.id),
    onSuccess: (created) => {
      void invalidate()
      toast.success(t('library.actions.cloned', { name: created.name }))
      void navigate({ to: '/t/$slug/library/tests/$id', params: { slug, id: created.id } })
    },
    onError,
  })
  const revalidate = useMutation({
    mutationFn: (test: Test) => libraryMutations.revalidateTest(slug, test.id),
    onSuccess: (updated) => {
      void invalidate()
      toast.success(
        updated.validation?.fits === false
          ? t('library.tests.revalidatedIssues')
          : t('library.tests.revalidated')
      )
    },
    onError,
  })
  const favorite = useMutation({
    mutationFn: (test: Test) => runMutations.favorite(slug, 'test', test.id, !test.is_favorite),
    onSuccess: () => void invalidate(),
    onError,
  })
  const remove = useMutation({
    mutationFn: (test: Test) => libraryMutations.deleteTest(slug, test.id),
    onSuccess: (_r, test) => {
      void invalidate()
      toast.success(t('library.actions.deleted', { name: test.name }))
      setDialog({ kind: 'none' })
    },
    onError: (e) => {
      onError(e)
      setDialog({ kind: 'none' })
    },
  })

  const openWizard = (test: Test, step = 1) =>
    void navigate({
      to: '/t/$slug/library/tests/new',
      params: { slug },
      search: { id: test.id, step } as never,
    })

  const rowActions = (r: Test): RowAction[] => {
    const noPerm = t('runs.actions.noPermission')
    const draft = r.status === 'draft'
    const usages = r.usages?.length ?? 0
    const runs = r.summary?.run_count ?? 0
    const openDetail = (tab?: 'runs') =>
      void navigate({
        to: '/t/$slug/library/tests/$id',
        params: { slug, id: r.id },
        search: (tab ? { tab } : {}) as never,
      })
    return [
      {
        key: 'open',
        label: t('common.actions.open'),
        icon: 'eye',
        onClick: () => (draft ? openWizard(r) : openDetail()),
      },
      {
        key: 'launch',
        label: t('common.actions.launch'),
        icon: 'play',
        disabled: !canRun || draft,
        disabledReason: !canRun ? noPerm : t('library.tests.draftCannotLaunch'),
        onClick: () => setDialog({ kind: 'launch', test: r }),
      },
      {
        key: 'runs',
        label: t('library.actions.testRuns'),
        description: runs ? t('library.tests.runCount', { count: runs }) : undefined,
        icon: 'history',
        disabled: runs === 0,
        disabledReason: t('library.actions.noRunsYet'),
        onClick: () => openDetail('runs'),
      },
      {
        key: 'edit',
        group: true,
        label: t('common.actions.edit'),
        icon: 'edit',
        disabled: !canEdit,
        disabledReason: noPerm,
        onClick: () => openWizard(r),
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
        key: 'validate',
        label: draft ? t('library.tests.finalize') : t('library.tests.revalidate'),
        description: draft ? t('library.tests.finalizeHint') : undefined,
        icon: 'check-circle',
        disabled: !canEdit || revalidate.isPending,
        disabledReason: noPerm,
        onClick: () => (draft ? openWizard(r, 4) : revalidate.mutate(r)),
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
          copyToClipboard(
            draft
              ? `${window.location.origin}/t/${slug}/library/tests/new?id=${r.id}`
              : `${window.location.origin}/t/${slug}/library/tests/${r.id}`,
            t
          ),
      },
      {
        key: 'copyId',
        label: t('library.actions.copyId'),
        icon: 'clipboard-alt',
        onClick: () => copyToClipboard(r.id, t),
      },
      {
        key: 'delete',
        group: true,
        label: t('common.actions.delete'),
        icon: 'trash-alt',
        destructive: true,
        disabled: !canEdit || usages > 0,
        disabledReason: !canEdit ? noPerm : t('library.actions.inUseShort', { count: usages }),
        onClick: () => setDialog({ kind: 'delete', test: r }),
      },
    ]
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure; search/rows/catalog/providers drive the recompute
  const columns = useMemo<DataTableColumn<Test>[]>(
    () => [
      col.status<Test>({
        id: 'status',
        title: t('common.fields.status'),
        status: (r) => r.status,
        sortKey: 'status',
        filter: {
          kind: 'checklist',
          options: TEST_STATUSES.map((s) => ({
            value: s,
            label: t(`common.status.${s}`),
            count: rows.filter((r) => r.status === s).length,
          })),
          value: search.status,
          onChange: (v) => onSearchChange({ status: v as TestListSearch['status'] }),
        },
      }),
      col.identity<Test>({
        id: 'name',
        header: t('library.columns.test'),
        sortKey: 'name',
        filter: {
          kind: 'text',
          value: search.name,
          placeholder: t('library.columns.name'),
          onChange: (v) => onSearchChange({ name: v }),
        },
        render: (r) => {
          const second =
            r.status === 'draft'
              ? t('library.tests.draftHint')
              : r.status === 'needs_attention'
                ? (r.validation?.issues?.[0]?.message ?? t('common.status.needs_attention'))
                : r.description
          return {
            title: r.name,
            link: (r.status === 'draft'
              ? { to: '/t/$slug/library/tests/new', params: { slug }, search: { id: r.id } }
              : { to: '/t/$slug/library/tests/$id', params: { slug, id: r.id } }) as CellLink,
            subtitle:
              r.tags && Object.keys(r.tags).length ? (
                <>
                  <TagChips tags={r.tags} onPick={(tag) => onSearchChange({ tags: tag })} />
                  <span title={second}>{second}</span>
                </>
              ) : (
                second
              ),
            badges: r.is_favorite ? <FavoriteMark /> : undefined,
          }
        },
      }),
      col.stack<Test>({
        id: 'database',
        header: t('library.columns.dbShort'),
        minWidth: 200,
        sortOptions: [
          { key: 'kind', label: t('library.sort.engine') },
          { key: 'database', label: t('library.sort.database') },
        ],
        filter: {
          kind: 'checklist',
          options: (catalog.data?.data ?? []).map((c) => ({
            value: c.kind,
            label: c.title,
            count: rows.filter((r) => r.summary?.db_kind === c.kind).length,
          })),
          value: search.kind,
          onChange: (v) => onSearchChange({ kind: v }),
        },
        value: (r) => r.summary?.db_kind,
        render: (r) => databaseView(r, t),
      }),
      col.stack<Test>({
        id: 'workload',
        header: t('library.columns.workload'),
        minWidth: 240,
        sortKey: 'workload',
        value: (r) => workloadView(r, t).primary,
        render: (r) => workloadView(r, t),
      }),
      col.custom<Test>({
        id: 'last_run',
        header: t('library.columns.lastRun'),
        width: 190,
        sortKey: 'last_run_at',
        value: (r) => r.summary?.last_run?.started_at ?? undefined,
        cell: (r) => <TestLastRunCell test={r} />,
      }),
      col.stack<Test>({
        id: 'provider',
        header: t('library.columns.provider'),
        width: 200,
        defaultHidden: true,
        sortKey: 'provider',
        value: (r) => providerName(r.provider_profile_id),
        render: (r) => {
          const sizes = Object.entries(r.sizes ?? {})
          const primary = r.provider_profile_id
            ? (providerName(r.provider_profile_id) ?? r.provider_profile_id)
            : t('library.tests.pickAtLaunch')
          const secondary = sizes.map(([role, s]) => `${role}:${s.size}`).join(' · ')
          const full = sizes
            .map(
              ([role, s]) =>
                `${role}: ${s.size}${s.disk?.gb ? ` · ${s.disk.gb} GB ${s.disk.type ?? ''}` : ''}`
            )
            .join('\n')
          return {
            primary,
            secondary: secondary || undefined,
            title: [primary, full].filter(Boolean).join('\n'),
          }
        },
      }),
      col.time<Test>({
        id: 'updated_at',
        header: t('library.columns.changed'),
        width: 150,
        defaultHidden: true,
        sortKey: 'updated_at',
        value: (r) => r.updated_at,
      }),
      col.stack<Test>({
        id: 'author',
        header: t('library.columns.author'),
        width: 160,
        defaultHidden: true,
        sortKey: 'author',
        value: (r) => r.author.display_name,
        render: (r) => ({ primary: <UserLabel user={r.author} /> }),
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
      }),
      col.actions<Test>({ title: (r) => r.name, actions: rowActions }),
    ],
    [t, slug, search, onSearchChange, providerName, rows, authors, catalog.data]
  )
  const prefs = useTablePrefs('tests', columns)

  const pill = (key: string, label: string, patch: Partial<TestListSearch>): ActivePill => ({
    key,
    label,
    onRemove: () => onSearchChange(patch),
  })
  const pills: ActivePill[] = [
    ...(search.name
      ? [pill('name', `${t('library.columns.name')}: ${search.name}`, { name: undefined })]
      : []),
    ...(search.status ?? []).map((v) =>
      pill(`s:${v}`, `${t('common.fields.status')}: ${t(`common.status.${v}`)}`, {
        status: search.status?.filter((x) => x !== v),
      })
    ),
    ...(search.kind ?? []).map((v) =>
      pill(`k:${v}`, `${t('library.sort.engine')}: ${v}`, {
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
      Object.fromEntries(FILTER_KEYS.map((k) => [k, undefined])) as Partial<TestListSearch>
    )
  const filtered = pills.length > 0 || !!search.q || !!search.favorites
  const attention = rows.filter((r) => r.status === 'needs_attention').length
  const newTest = () => void navigate({ to: '/t/$slug/library/tests/new', params: { slug } })

  return (
    <Page width="wide" fill>
      <PageHeader
        title={t('library.tests.title')}
        icon="vial"
        subtitle={t('library.tests.subtitle')}
        badge={
          attention > 0 ? (
            <Badge
              color="orange"
              icon="exclamation-triangle"
              text={t('library.tests.attention', { count: attention })}
            />
          ) : undefined
        }
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
            <Button icon="plus" disabled={!canEdit && !canRun} onClick={newTest}>
              {t('library.tests.new')}
            </Button>
          </>
        }
      />
      <DataTableToolbar
        search={search.q}
        onSearch={(v) => onSearchChange({ q: v || undefined })}
        searchPlaceholder={t('library.tests.search')}
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
      <DataTable<Test>
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
            sort: (s?.field as TestListSearch['sort']) ?? 'updated_at',
            order: s?.order ?? 'desc',
          })
        }
        onOverlayChange={auto.setPaused}
        rowHref={(r) =>
          r.status === 'draft'
            ? `/t/${slug}/library/tests/new?id=${r.id}`
            : `/t/${slug}/library/tests/${r.id}`
        }
        filtered={filtered}
        onClearFilters={clearAll}
        empty={{
          message: t('library.tests.empty.title'),
          button: (
            <Button icon="plus" disabled={!canEdit && !canRun} onClick={newTest}>
              {t('library.tests.new')}
            </Button>
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
      <ImportModal kind="test" isOpen={importOpen} onClose={() => setImportOpen(false)} />
      {dialog.kind === 'launch' && (
        <LaunchDrawer test={dialog.test} isOpen onClose={() => setDialog({ kind: 'none' })} />
      )}
      <ConfirmModal
        isOpen={dialog.kind === 'delete'}
        title={t('common.confirm.deleteTitle', {
          name: dialog.kind === 'delete' ? dialog.test.name : '',
        })}
        body={t('common.confirm.deleteBody')}
        confirmText={t('common.confirm.yesDelete')}
        confirmButtonVariant="destructive"
        dismissText={t('common.actions.cancel')}
        disabled={remove.isPending}
        onConfirm={() => {
          if (dialog.kind === 'delete') remove.mutate(dialog.test)
        }}
        onDismiss={() => setDialog({ kind: 'none' })}
      />
    </Page>
  )
}
