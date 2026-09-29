import { runMutations } from '@api/queries/runs'
import { type SuiteListQuery, suiteMutations, suiteQueries } from '@api/queries/suites'
import type { Schemas } from '@api/types'
import { AppLink } from '@app/AppLink'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { TagsCell } from '@components/DataTable/cells'
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
import { StatusBadge } from '@components/StatusBadge'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, ConfirmModal, Icon, Stack, Tooltip, useStyles2 } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import {
  keepPreviousData,
  useInfiniteQuery,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import type { TFunction } from 'i18next'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { SuiteImportDialog } from './SuiteImportDialog'
import { SuiteLaunchDialog } from './SuiteLaunchDialog'

type Suite = Schemas['Suite']

export const suiteListSearchSchema = z.object({
  q: z.string().optional().catch(undefined),
  name: z.string().optional().catch(undefined),
  tags: z.string().optional().catch(undefined),
  author: csvSchema,
  favorites: z.boolean().optional().catch(undefined),
  sort: z
    .enum(['name', 'created_at', 'updated_at', 'last_run_at', 'cell_count', 'test_count', 'author'])
    .default('updated_at')
    .catch('updated_at'),
  order: orderSchema.default('desc').catch('desc'),
  size: sizeSchema,
  refresh: refreshSchema('off'),
})
export type SuiteListSearch = z.infer<typeof suiteListSearchSchema>
export const SUITE_LIST_DEFAULTS = {
  sort: 'updated_at',
  order: 'desc',
  size: 50,
  refresh: 'off',
} as const

export function toSuiteListQuery(s: SuiteListSearch): SuiteListQuery {
  return {
    search: joinSearch(s.q, s.name),
    tags: s.tags,
    author: authorParam(s.author),
    favorites: s.favorites,
    sort: s.sort,
    order: s.order,
    limit: s.size,
  }
}

const FILTER_KEYS = ['q', 'name', 'tags', 'author', 'favorites'] as const

const getStyles = (theme: GrafanaTheme2) => ({
  star: css({ color: theme.colors.warning.text }),
  tags: css({ display: 'inline-flex', verticalAlign: 'middle', marginRight: theme.spacing(0.75) }),
  inline: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(0.75) }),
  muted: css({ color: theme.colors.text.secondary }),
})

function tagList(tags: Suite['tags']): string[] {
  return Object.entries(tags ?? {}).map(([k, v]) => (v ? `${k}=${v}` : k))
}

// «Состав»: N tests over «M of K cells · axes: …».
function compositionView(s: Suite, t: TFunction) {
  const total = s.summary?.cell_count ?? 0
  const enabled = s.summary?.enabled_cell_count ?? 0
  const a = s.axes
  const axes = [
    a?.provider_profiles?.length ? t('suites.cells.axis.provider') : undefined,
    a?.sizes?.length ? t('suites.cells.axis.sizes') : undefined,
    a?.database_versions?.length ? t('suites.cells.axis.version') : undefined,
    a?.workload_variants?.length ? t('suites.cells.axis.variant') : undefined,
  ].filter(Boolean) as string[]
  const primary = t('suites.tests', { count: s.tests?.length ?? 0 })
  const secondary = [
    t('suites.list.cells', { count: total, enabled }),
    axes.length ? t('suites.list.axes', { axes: axes.join(', ').toLowerCase() }) : undefined,
  ]
    .filter(Boolean)
    .join(' · ')
  return { primary, secondary, title: `${primary}\n${secondary}` }
}

// Downloads an export document as a .json file.
export function downloadJson(name: string, doc: unknown): void {
  const blob = new Blob([JSON.stringify(doc, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${name.replace(/[^a-z0-9-_]+/gi, '-').toLowerCase()}.suite.json`
  a.click()
  URL.revokeObjectURL(url)
}

export function SuiteListPage({
  search,
  onSearchChange,
}: {
  search: SuiteListSearch
  onSearchChange: (next: Partial<SuiteListSearch>) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug, can } = useTenant()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const query = useMemo(() => toSuiteListQuery(search), [search])
  const auto = useAutoRefresh(search.refresh)
  const list = useInfiniteQuery({
    ...suiteQueries.list(slug, query),
    placeholderData: keepPreviousData,
    refetchInterval: auto.refetchInterval,
  })
  const rows = useMemo(() => list.data?.pages.flatMap((p) => p.data) ?? [], [list.data])
  // The suites endpoint has no facets: author options come from the loaded page.
  const authors = useMemo(
    () => [...new Map(rows.map((r) => [r.author.id, r.author])).values()],
    [rows]
  )
  const [launching, setLaunching] = useState<Suite | undefined>()
  const [deleting, setDeleting] = useState<Suite | undefined>()
  const [importing, setImporting] = useState(false)
  const invalidate = () => qc.invalidateQueries({ queryKey: ['t', slug, 'suites'] })
  const onError = (e: unknown) => toast.error(e)
  const canEdit = can('edit-library')
  const canRun = can('run')

  const clone = useMutation({
    mutationFn: (s: Suite) => suiteMutations.clone(slug, s.id),
    onSuccess: (s) => {
      toast.success(t('suites.toasts.cloned'), { description: s.name })
      void invalidate()
      void navigate({ to: '/t/$slug/suites/$id', params: { slug, id: s.id } })
    },
    onError,
  })
  const exportM = useMutation({
    mutationFn: (s: Suite) => suiteMutations.export(slug, s.id),
    onSuccess: (doc, s) => {
      downloadJson(s.name, doc)
      toast.success(t('suites.toasts.exported'))
    },
    onError,
  })
  const favorite = useMutation({
    mutationFn: (s: Suite) => runMutations.favorite(slug, 'suite', s.id, !s.is_favorite),
    onSuccess: () => void invalidate(),
    onError,
  })
  const remove = useMutation({
    mutationFn: (s: Suite) => suiteMutations.remove(slug, s.id),
    onSuccess: () => {
      toast.success(t('suites.toasts.deleted'))
      void invalidate()
      setDeleting(undefined)
    },
    onError: (e) => {
      onError(e)
      setDeleting(undefined)
    },
  })

  const copy = (text: string) =>
    void navigator.clipboard
      .writeText(text)
      .then(() => toast.success(t('common.actions.copied')))
      .catch((e) => toast.error(e))

  const rowActions = (s: Suite): RowAction[] => {
    const noPerm = t('runs.actions.noPermission')
    const enabledCells = s.summary?.enabled_cell_count ?? 0
    return [
      {
        key: 'open',
        label: t('common.actions.open'),
        icon: 'eye',
        onClick: () => void navigate({ to: '/t/$slug/suites/$id', params: { slug, id: s.id } }),
      },
      {
        key: 'launch',
        label: t('suites.actions.launch'),
        icon: 'play',
        disabled: !canRun || enabledCells === 0,
        disabledReason: !canRun ? noPerm : t('suites.actions.noEnabledCells'),
        onClick: () => setLaunching(s),
      },
      {
        key: 'schedule',
        label: t('suites.actions.schedule'),
        icon: 'clock-nine',
        disabled: !canEdit,
        disabledReason: noPerm,
        onClick: () =>
          void navigate({
            to: '/t/$slug/schedules/new',
            params: { slug },
            search: { kind: 'suite', target: s.id } as never,
          }),
      },
      {
        key: 'edit',
        group: true,
        label: t('common.actions.edit'),
        icon: 'edit',
        disabled: !canEdit,
        disabledReason: noPerm,
        onClick: () =>
          void navigate({
            to: '/t/$slug/suites/$id',
            params: { slug, id: s.id },
            search: { tab: 'axes' } as never,
          }),
      },
      {
        key: 'duplicate',
        label: t('suites.actions.clone'),
        icon: 'copy',
        disabled: !canEdit || clone.isPending,
        disabledReason: noPerm,
        onClick: () => clone.mutate(s),
      },
      {
        key: 'export',
        label: t('suites.actions.export'),
        icon: 'download-alt',
        onClick: () => exportM.mutate(s),
      },
      {
        key: 'favorite',
        group: true,
        label: s.is_favorite ? t('common.actions.unfavorite') : t('common.actions.favorite'),
        icon: s.is_favorite ? 'favorite' : 'star',
        disabled: favorite.isPending,
        onClick: () => favorite.mutate(s),
      },
      {
        key: 'copyLink',
        label: t('runs.actions.copyLink'),
        icon: 'link',
        onClick: () => copy(`${window.location.origin}/t/${slug}/suites/${s.id}`),
      },
      {
        key: 'copyId',
        label: t('runs.actions.copyId'),
        icon: 'copy',
        onClick: () => copy(s.id),
      },
      {
        key: 'delete',
        group: true,
        label: t('suites.actions.delete'),
        icon: 'trash-alt',
        destructive: true,
        disabled: !canEdit,
        disabledReason: noPerm,
        onClick: () => setDeleting(s),
      },
    ]
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure; search/rows drive the recompute
  const columns = useMemo<DataTableColumn<Suite>[]>(
    () => [
      col.identity<Suite>({
        id: 'name',
        header: t('suites.columns.name'),
        sortKey: 'name',
        filter: {
          kind: 'text',
          value: search.name,
          placeholder: t('suites.columns.name'),
          onChange: (v) => onSearchChange({ name: v }),
        },
        render: (s) => {
          const tags = tagList(s.tags)
          return {
            title: s.name,
            link: { to: '/t/$slug/suites/$id', params: { slug, id: s.id } },
            subtitle:
              tags.length || s.description ? (
                <>
                  {tags.length > 0 && (
                    <span className={styles.tags}>
                      <TagsCell items={tags} max={2} />
                    </span>
                  )}
                  <span title={s.description}>{s.description}</span>
                </>
              ) : undefined,
            badges: s.is_favorite ? (
              <Tooltip content={t('suites.list.favorite')}>
                <Icon name="favorite" className={styles.star} />
              </Tooltip>
            ) : undefined,
          }
        },
      }),
      col.stack<Suite>({
        id: 'composition',
        header: t('suites.columns.composition'),
        minWidth: 240,
        sortOptions: [
          { key: 'test_count', label: t('suites.columns.tests') },
          { key: 'cell_count', label: t('suites.columns.cells') },
        ],
        value: (s) => s.tests?.length ?? 0,
        render: (s) => compositionView(s, t),
      }),
      col.stack<Suite>({
        id: 'last_run',
        header: t('suites.columns.lastRun'),
        width: 230,
        sortKey: 'last_run_at',
        value: (s) => s.summary?.last_run?.started_at ?? undefined,
        render: (s) => {
          const lr = s.summary?.last_run
          if (!lr) return { primary: <span className={styles.muted}>{t('suites.noRuns')}</span> }
          const name = lr.name ?? lr.id
          const runs = t('suites.runs', { count: s.summary?.run_count ?? 0 })
          return {
            primary: (
              <span className={styles.inline}>
                <StatusBadge status={lr.status} iconOnly />
                {lr.started_at ? <RelativeTime value={lr.started_at} /> : t('runs.list.queued')}
              </span>
            ),
            secondary: (
              <>
                <AppLink to="/t/$slug/suite-runs/$id" params={{ slug, id: lr.id }} plain>
                  {name}
                </AppLink>
                {` · ${runs}`}
              </>
            ),
            title: `${name}\n${runs}`,
          }
        },
      }),
      col.stack<Suite>({
        id: 'schedules',
        header: t('suites.columns.schedules'),
        width: 200,
        render: (s) => {
          const sch = s.summary?.schedules ?? []
          if (!sch.length) return { primary: undefined }
          const first = sch[0]
          return {
            primary: (
              <>
                <AppLink to="/t/$slug/schedules/$id" params={{ slug, id: first.id }} plain>
                  {first.name ?? first.id}
                </AppLink>
                {sch.length > 1 ? ` +${sch.length - 1}` : ''}
              </>
            ),
            secondary: t('suites.list.schedules', { count: sch.length }),
            title: sch.map((x) => x.name ?? x.id).join('\n'),
          }
        },
      }),
      col.stack<Suite>({
        id: 'updated',
        header: t('suites.columns.updated'),
        width: 190,
        defaultHidden: true,
        sortOptions: [
          { key: 'updated_at', label: t('suites.sort.updated') },
          { key: 'created_at', label: t('suites.sort.created') },
          { key: 'author', label: t('suites.columns.author') },
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
        value: (s) => s.updated_at,
        render: (s) => ({
          primary: <RelativeTime value={s.updated_at} />,
          secondary: s.author.display_name,
          title: s.author.display_name,
        }),
      }),
      col.tags<Suite>({
        id: 'tags',
        header: t('suites.columns.tags'),
        defaultHidden: true,
        items: (s) => tagList(s.tags),
        filter: {
          kind: 'text',
          value: search.tags,
          placeholder: 'key=value',
          onChange: (v) => onSearchChange({ tags: v }),
        },
      }),
      col.actions<Suite>({ title: (s) => s.name, actions: rowActions }),
    ],
    [t, slug, styles, onSearchChange, search, rows, authors]
  )
  const prefs = useTablePrefs('suites', columns)

  const pill = (key: string, label: string, patch: Partial<SuiteListSearch>): ActivePill => ({
    key,
    label,
    onRemove: () => onSearchChange(patch),
  })
  const pills: ActivePill[] = [
    ...(search.name
      ? [pill('name', `${t('suites.columns.name')}: ${search.name}`, { name: undefined })]
      : []),
    ...(search.tags
      ? [pill('tags', `${t('suites.filters.tags')}: ${search.tags}`, { tags: undefined })]
      : []),
    ...(search.author ?? []).map((v) =>
      pill(
        `author:${v}`,
        `${t('suites.columns.author')}: ${authors.find((a) => a.id === v)?.display_name ?? v}`,
        { author: search.author?.filter((x) => x !== v) }
      )
    ),
  ]
  const clearAll = () =>
    onSearchChange(
      Object.fromEntries(FILTER_KEYS.map((k) => [k, undefined])) as Partial<SuiteListSearch>
    )
  const filtered = pills.length > 0 || !!search.q || !!search.favorites

  return (
    <Page width="wide" fill>
      <PageHeader
        title={t('suites.title')}
        icon="layer-group"
        actions={
          <>
            <Button
              variant="secondary"
              icon="import"
              onClick={() => setImporting(true)}
              disabled={!canEdit}
            >
              {t('suites.actions.import')}
            </Button>
            <Button
              icon="plus"
              disabled={!canEdit}
              onClick={() => void navigate({ to: '/t/$slug/suites/new', params: { slug } })}
            >
              {t('suites.new')}
            </Button>
          </>
        }
      />
      <DataTableToolbar
        search={search.q}
        onSearch={(v) => onSearchChange({ q: v || undefined })}
        searchPlaceholder={t('suites.search')}
        toggles={[
          {
            key: 'fav',
            label: t('suites.filters.favorites'),
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
      <DataTable<Suite>
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
            sort: (s?.field as SuiteListSearch['sort']) ?? 'updated_at',
            order: s?.order ?? 'desc',
          })
        }
        onOverlayChange={auto.setPaused}
        rowHref={(r) => `/t/${slug}/suites/${r.id}`}
        filtered={filtered}
        onClearFilters={clearAll}
        empty={{
          message: t('suites.empty.title'),
          button: (
            <Stack gap={1}>
              <Button
                icon="plus"
                disabled={!canEdit}
                onClick={() => void navigate({ to: '/t/$slug/suites/new', params: { slug } })}
              >
                {t('suites.empty.cta')}
              </Button>
              <Button
                variant="secondary"
                icon="import"
                disabled={!canEdit}
                onClick={() => setImporting(true)}
              >
                {t('suites.empty.import')}
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
      {launching && <SuiteLaunchDialog suite={launching} onClose={() => setLaunching(undefined)} />}
      {importing && <SuiteImportDialog onClose={() => setImporting(false)} />}
      <ConfirmModal
        isOpen={!!deleting}
        title={t('common.confirm.deleteTitle', { name: deleting?.name ?? '' })}
        body={t('suites.confirm.deleteBody')}
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
