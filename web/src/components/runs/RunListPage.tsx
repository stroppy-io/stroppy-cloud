import { keys } from '@api/queries/keys'
import { type RunListQuery, runMutations, runQueries } from '@api/queries/runs'
import type { Run } from '@api/types'
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
  secondsToDuration,
  sizeSchema,
} from '@components/DataTable/list-search'
import { useTablePrefs } from '@components/DataTable/prefs'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { TableSettings } from '@components/DataTable/TableSettings'
import { type ActivePill, DataTableToolbar } from '@components/DataTable/Toolbar'
import { useAutoRefresh } from '@components/DataTable/useAutoRefresh'
import { KeepExtendModal } from '@components/runs/detail/KeepExtendModal'
import { type RerunBody, RerunDrawer } from '@components/runs/detail/RerunDrawer'
import { SaveAsTestModal } from '@components/runs/detail/SaveAsTestModal'
import {
  databaseView,
  providerView,
  RunMetricsCell,
  RunTimeCell,
  TriggerMark,
  workloadView,
} from '@components/runs/list/RunCells'
import { ShareModal } from '@components/runs/ShareModal'
import { css } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import { Badge, Button, ConfirmModal, Icon, Stack, Tooltip, useStyles2 } from '@grafana/ui'
import { isTerminal } from '@helpers/run-status'
import { useTenant } from '@hooks/useTenant'
import { useTopic } from '@hooks/useTopic'
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

export const RUN_SORT_KEYS = [
  'default',
  'qps',
  'p50',
  'errors',
  'started_at',
  'finished_at',
  'duration',
  'tps',
  'p99',
  'status',
  'name',
  'db_kind',
  'workload',
  'topology',
  'provider',
  'trigger',
  'author',
  'created_at',
  'updated_at',
] as const

// Search params of /t/$slug/runs — the URL is the only source of truth for filters.
export const runListSearchSchema = z.object({
  q: z.string().optional().catch(undefined),
  name: z.string().optional().catch(undefined),
  status: z
    .array(z.enum(['pending', 'running', 'cancelling', 'completed', 'failed', 'cancelled']))
    .optional()
    .catch(undefined),
  kind: csvSchema,
  provider: csvSchema,
  trigger: z
    .array(z.enum(['manual', 'suite', 'api', 'schedule']))
    .optional()
    .catch(undefined),
  author: csvSchema,
  labels: z.string().optional().catch(undefined),
  test: z.string().optional().catch(undefined),
  suiteRun: z.string().optional().catch(undefined),
  favorites: z.boolean().optional().catch(undefined),
  kept: z.boolean().optional().catch(undefined),
  startedFrom: z.string().optional().catch(undefined),
  startedTo: z.string().optional().catch(undefined),
  finishedFrom: z.string().optional().catch(undefined),
  finishedTo: z.string().optional().catch(undefined),
  durMin: z.coerce.number().optional().catch(undefined),
  durMax: z.coerce.number().optional().catch(undefined),
  sort: z.enum(RUN_SORT_KEYS).default('default').catch('default'),
  order: orderSchema.default('desc').catch('desc'),
  size: sizeSchema,
  refresh: refreshSchema('5s'),
})
export type RunListSearch = z.infer<typeof runListSearchSchema>
export const RUN_LIST_DEFAULTS = {
  sort: 'default',
  order: 'desc',
  size: 50,
  refresh: '5s',
} as const

export function toRunListQuery(s: RunListSearch): RunListQuery {
  return {
    search: joinSearch(s.q, s.name),
    status: s.status,
    kind: s.kind as RunListQuery['kind'],
    provider_profile: s.provider,
    trigger: s.trigger,
    author: authorParam(s.author),
    labels: s.labels,
    test_id: s.test,
    suite_run_id: s.suiteRun,
    favorites: s.favorites,
    stand_kept: s.kept,
    started_after: s.startedFrom,
    started_before: s.startedTo,
    finished_after: s.finishedFrom,
    finished_before: s.finishedTo,
    duration_min: secondsToDuration(s.durMin),
    duration_max: secondsToDuration(s.durMax),
    sort: s.sort,
    order: s.order,
    limit: s.size,
  }
}

const FILTER_KEYS = [
  'q',
  'name',
  'status',
  'kind',
  'provider',
  'trigger',
  'author',
  'labels',
  'test',
  'suiteRun',
  'favorites',
  'kept',
  'startedFrom',
  'startedTo',
  'finishedFrom',
  'finishedTo',
  'durMin',
  'durMax',
] as const

const TRIGGER_ICON: Record<Run['trigger'], IconName> = {
  manual: 'user',
  suite: 'layer-group',
  api: 'brackets-curly',
  schedule: 'clock-nine',
}

const getStyles = (theme: GrafanaTheme2) => ({
  star: css({ color: theme.colors.warning.text }),
  labels: css({
    display: 'inline-flex',
    verticalAlign: 'middle',
    marginRight: theme.spacing(0.75),
  }),
})

type Dialog =
  | { kind: 'none' }
  | { kind: 'clone' | 'saveAsTest' | 'keep' | 'share' | 'delete'; run: Run }

export function RunListPage({
  search,
  onSearchChange,
}: {
  search: RunListSearch
  onSearchChange: (next: Partial<RunListSearch>) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug, can } = useTenant()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const query = useMemo(() => toRunListQuery(search), [search])
  const auto = useAutoRefresh(search.refresh)
  const list = useInfiniteQuery({
    ...runQueries.list(slug, query),
    placeholderData: keepPreviousData,
    refetchInterval: auto.refetchInterval,
  })
  const facets = useQuery(runQueries.facets(slug, query))
  const [selected, setSelected] = useState<Record<string, boolean>>({})
  const [dialog, setDialog] = useState<Dialog>({ kind: 'none' })
  const invalidate = () => qc.invalidateQueries({ queryKey: keys.t(slug) })
  const canRun = can('run')
  const canShare = can('share')

  // Live updates: merge run deltas into every cached page.
  useTopic<{ data: Run[] }, typeof list.data>({
    topic: `tenant.runs/${slug}`,
    queryKey: runQueries.list(slug, query).queryKey,
    batchMs: 250,
    merge: (prev, payload) => {
      if (!prev) return prev as never
      const byId = new Map(payload.data.map((r) => [r.id, r]))
      return {
        ...prev,
        pages: prev.pages.map((p) => ({ ...p, data: p.data.map((r) => byId.get(r.id) ?? r) })),
      }
    },
  })

  const cancel = useMutation({
    mutationFn: (run: Run) => runMutations.cancel(slug, run.id),
    onSuccess: () => {
      toast.success(t('runs.toasts.cancelRequested'))
      void invalidate()
    },
    onError: (e) => toast.error(e),
  })
  const rerun = useMutation({
    mutationFn: ({ run, body }: { run: Run; body: RerunBody }) =>
      runMutations.rerun(slug, run.id, body),
    onSuccess: (created) => {
      toast.success(t('runs.toasts.rerunStarted', { name: created.name }))
      void invalidate()
      setDialog({ kind: 'none' })
      void navigate({ to: '/t/$slug/runs/$id', params: { slug, id: created.id } })
    },
    onError: (e) => toast.error(e),
  })
  const saveAsTest = useMutation({
    mutationFn: ({
      run,
      body,
    }: {
      run: Run
      body: { name: string; save_database_as?: string; save_workload_as?: string }
    }) => runMutations.saveAsTest(slug, run.id, body),
    onSuccess: (test) => {
      toast.success(t('runs.toasts.savedAsTest', { name: test.name }), {
        action: {
          label: t('common.actions.open'),
          onClick: () =>
            void navigate({ to: '/t/$slug/library/tests/$id', params: { slug, id: test.id } }),
        },
      })
      void invalidate()
      setDialog({ kind: 'none' })
    },
    onError: (e) => toast.error(e),
  })
  const keepExtend = useMutation({
    mutationFn: ({ run, duration }: { run: Run; duration: string }) =>
      runMutations.keepExtend(slug, run.id, duration),
    onSuccess: () => {
      toast.success(t('runs.toasts.keepExtended'))
      void invalidate()
      setDialog({ kind: 'none' })
    },
    onError: (e) => toast.error(e),
  })
  const keepRelease = useMutation({
    mutationFn: (run: Run) => runMutations.keepRelease(slug, run.id),
    onSuccess: () => {
      toast.success(t('runs.toasts.keepReleased'))
      void invalidate()
    },
    onError: (e) => toast.error(e),
  })
  const favorite = useMutation({
    mutationFn: (run: Run) => runMutations.favorite(slug, 'run', run.id, !run.is_favorite),
    onSuccess: () => void invalidate(),
    onError: (e) => toast.error(e),
  })
  const remove = useMutation({
    mutationFn: (run: Run) => runMutations.remove(slug, run.id),
    onSuccess: (_r, run) => {
      toast.success(t('runs.toasts.deleted', { name: run.name }))
      void invalidate()
      setDialog({ kind: 'none' })
    },
    onError: (e) => toast.error(e),
  })

  const rows = useMemo(() => list.data?.pages.flatMap((p) => p.data) ?? [], [list.data])
  const facet = (field: string) => facets.data?.data.find((f) => f.field === field)?.values ?? []
  const runningCount = facet('status').find((v) => v.value === 'running')?.count ?? 0
  const authorName = (id: string) =>
    facet('author').find((v) => v.value === id)?.label ??
    rows.find((r) => r.author.id === id)?.author.display_name ??
    id

  const copy = (text: string) =>
    void navigator.clipboard
      .writeText(text)
      .then(() => toast.success(t('common.actions.copied')))
      .catch((e) => toast.error(e))

  // Compare with the previous run of the same test (the newest one created before this run).
  const comparePrevious = async (r: Run) => {
    const testId = r.test_ref.id
    if (!testId) return
    try {
      const page = await qc.fetchInfiniteQuery(
        runQueries.list(slug, { test_id: testId, sort: 'created_at', order: 'desc', limit: 50 })
      )
      const prev = page.pages[0]?.data.find(
        (x) => x.id !== r.id && new Date(x.created_at) < new Date(r.created_at)
      )
      if (!prev) {
        toast.info(t('runs.actions.comparePrevNone'))
        return
      }
      void navigate({
        to: '/t/$slug/compare',
        params: { slug },
        search: { runs: [prev.id, r.id] } as never,
      })
    } catch (e) {
      toast.error(e)
    }
  }

  const rowActions = (r: Run): RowAction[] => {
    const terminal = isTerminal(r.status)
    const active = r.status === 'running' || r.status === 'pending' || r.status === 'cancelling'
    const canCancel = can('cancel-any-run', r.author.id)
    const noPerm = t('runs.actions.noPermission')
    const open = (tab?: 'logs' | 'metrics') =>
      void navigate({
        to: tab ? `/t/$slug/runs/$id/${tab}` : '/t/$slug/runs/$id',
        params: { slug, id: r.id },
      } as never)
    const rerunLabel =
      r.status === 'completed' ? t('runs.actions.rerunAgain') : t('runs.actions.rerun')
    return [
      { key: 'open', label: t('common.actions.open'), icon: 'eye', onClick: () => open() },
      {
        key: 'logs',
        label: t('runs.tabs.logs'),
        icon: 'document-info',
        onClick: () => open('logs'),
      },
      {
        key: 'metrics',
        label: t('runs.tabs.metrics'),
        icon: 'chart-line',
        onClick: () => open('metrics'),
      },
      {
        key: 'test',
        label: t('runs.actions.openTest'),
        description: r.test_ref.name,
        icon: 'vial',
        disabled: !r.test_ref.id,
        disabledReason: t('runs.actions.noTest'),
        onClick: () =>
          void navigate({
            to: '/t/$slug/library/tests/$id',
            params: { slug, id: r.test_ref.id as string },
          }),
      },
      {
        key: 'comparePrev',
        label: t('runs.actions.comparePrev'),
        icon: 'columns',
        disabled: !r.test_ref.id,
        disabledReason: t('runs.actions.noTest'),
        onClick: () => void comparePrevious(r),
      },
      {
        key: 'share',
        group: true,
        label: t('common.actions.share'),
        icon: 'share-alt',
        disabled: !canShare,
        disabledReason: noPerm,
        onClick: () => setDialog({ kind: 'share', run: r }),
      },
      {
        key: 'copyLink',
        label: t('runs.actions.copyLink'),
        icon: 'link',
        onClick: () => copy(`${window.location.origin}/t/${slug}/runs/${r.id}`),
      },
      {
        key: 'copyId',
        label: t('runs.actions.copyId'),
        icon: 'copy',
        onClick: () => copy(r.id),
      },
      {
        key: 'favorite',
        label: r.is_favorite ? t('common.actions.unfavorite') : t('common.actions.favorite'),
        icon: r.is_favorite ? 'favorite' : 'star',
        onClick: () => favorite.mutate(r),
      },
      {
        key: 'clone',
        group: true,
        label: t('runs.actions.clone'),
        icon: 'pen',
        disabled: !canRun,
        disabledReason: noPerm,
        onClick: () => setDialog({ kind: 'clone', run: r }),
      },
      {
        key: 'rerun',
        label: rerunLabel,
        icon: 'repeat',
        disabled: !canRun || !terminal || rerun.isPending,
        disabledReason: !canRun ? noPerm : t('runs.actions.rerunNotTerminal'),
        onClick: () => rerun.mutate({ run: r, body: {} }),
      },
      {
        key: 'saveAsTest',
        label: t('runs.actions.saveAsTest'),
        icon: 'save',
        disabled: !canRun,
        disabledReason: noPerm,
        onClick: () => setDialog({ kind: 'saveAsTest', run: r }),
      },
      {
        key: 'cancel',
        group: true,
        label: t('runs.actions.cancelRun'),
        icon: 'times',
        disabled: !canCancel || !active || cancel.isPending,
        disabledReason: !canCancel ? noPerm : t('runs.actions.cancelNotActive'),
        onClick: () => cancel.mutate(r),
      },
      {
        key: 'keepExtend',
        label: t('runs.keep.extend'),
        icon: 'lock',
        disabled: !canRun,
        disabledReason: noPerm,
        onClick: () => setDialog({ kind: 'keep', run: r }),
      },
      {
        key: 'keepRelease',
        label: t('runs.keep.release'),
        icon: 'unlock',
        disabled: !canRun || !r.stand_kept || keepRelease.isPending,
        disabledReason: !canRun ? noPerm : t('runs.keep.notKeptShort'),
        onClick: () => keepRelease.mutate(r),
      },
      {
        key: 'delete',
        group: true,
        label: t('common.actions.delete'),
        icon: 'trash-alt',
        destructive: true,
        disabled: !canCancel || !terminal,
        disabledReason: !canCancel ? noPerm : t('runs.actions.deleteNotTerminal'),
        onClick: () => setDialog({ kind: 'delete', run: r }),
      },
    ]
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: facet/rowActions are per-render closures; facets.data + search drive the recompute
  const columns = useMemo<DataTableColumn<Run>[]>(
    () => [
      col.status<Run>({
        id: 'status',
        title: t('runs.columns.status'),
        status: (r) => r.status,
        sortKey: 'status',
        filter: {
          kind: 'checklist',
          options: facet('status').map((v) => ({
            value: v.value,
            label: t(`common.status.${v.value}`),
            count: v.count,
          })),
          value: search.status,
          onChange: (v) => onSearchChange({ status: v as RunListSearch['status'] }),
        },
      }),
      col.identity<Run>({
        id: 'name',
        header: t('runs.columns.name'),
        sortKey: 'name',
        filter: {
          kind: 'text',
          value: search.name,
          placeholder: t('runs.columns.name'),
          onChange: (v) => onSearchChange({ name: v }),
        },
        render: (r) => {
          const running = r.status === 'running' || r.status === 'cancelling'
          const labels = Object.entries(r.labels ?? {}).map(([k, v]) => (v ? `${k}=${v}` : k))
          return {
            title: r.name,
            link: { to: '/t/$slug/runs/$id', params: { slug, id: r.id } },
            lead:
              r.trigger !== 'manual' ? (
                <TriggerMark trigger={r.trigger} icon={TRIGGER_ICON[r.trigger]} />
              ) : undefined,
            subtitle: (
              <>
                {labels.length > 0 && (
                  <span className={styles.labels}>
                    <TagsCell items={labels} max={2} />
                  </span>
                )}
                {running
                  ? [
                      t(`common.phase.${r.phase}`),
                      r.summary?.segment
                        ? t('runs.segment', { name: r.summary.segment })
                        : undefined,
                      t('runs.progress', { pct: r.summary?.progress_pct ?? 0 }),
                    ]
                      .filter(Boolean)
                      .join(' · ')
                  : r.status_reason || r.test_ref.name}
              </>
            ),
            progress: running ? (r.summary?.progress_pct ?? 0) : undefined,
            badges: (
              <>
                {r.is_favorite && (
                  <Tooltip content={t('runs.list.favorite')}>
                    <Icon name="favorite" className={styles.star} />
                  </Tooltip>
                )}
                {r.stand_kept && <Badge text="keep" color="purple" icon="lock" />}
              </>
            ),
          }
        },
      }),
      col.stack<Run>({
        id: 'database',
        header: t('runs.columns.database'),
        minWidth: 200,
        sortOptions: [
          { key: 'db_kind', label: t('runs.sort.engine') },
          { key: 'topology', label: t('runs.sort.topology') },
        ],
        filter: {
          kind: 'checklist',
          options: facet('kind').map((v) => ({
            value: v.value,
            label: v.label ?? v.value,
            count: v.count,
          })),
          value: search.kind,
          onChange: (v) => onSearchChange({ kind: v }),
        },
        value: (r) => r.summary?.db_kind,
        render: (r) => databaseView(r, t),
      }),
      col.stack<Run>({
        id: 'workload',
        header: t('runs.columns.workload'),
        minWidth: 240,
        sortKey: 'workload',
        value: (r) => workloadView(r, t).primary,
        render: (r) => workloadView(r, t),
      }),
      col.custom<Run>({
        id: 'metrics',
        header: t('runs.columns.metrics'),
        width: 290,
        sortOptions: [
          { key: 'qps', label: 'QPS' },
          { key: 'p99', label: 'p99' },
          { key: 'p50', label: 'p50' },
          { key: 'errors', label: t('runs.sort.errors') },
        ],
        cell: (r) => <RunMetricsCell run={r} />,
      }),
      col.custom<Run>({
        id: 'time',
        header: t('runs.columns.time'),
        width: 180,
        sortOptions: [
          { key: 'started_at', label: t('runs.sort.started') },
          { key: 'finished_at', label: t('runs.sort.finished') },
          { key: 'duration', label: t('runs.sort.duration') },
        ],
        filter: {
          kind: 'date',
          from: search.startedFrom,
          to: search.startedTo,
          onChange: (from, to) => onSearchChange({ startedFrom: from, startedTo: to }),
        },
        cell: (r) => <RunTimeCell run={r} />,
      }),
      col.stack<Run>({
        id: 'provider',
        header: t('runs.columns.provider'),
        width: 200,
        defaultHidden: true,
        sortKey: 'provider',
        filter: {
          kind: 'checklist',
          options: facet('provider_profile').map((v) => ({
            value: v.value,
            label: v.label ?? v.value,
            count: v.count,
          })),
          value: search.provider,
          onChange: (v) => onSearchChange({ provider: v }),
        },
        value: (r) => r.summary?.provider_profile?.name,
        render: (r) => providerView(r),
      }),
      col.text<Run>({
        id: 'author',
        header: t('runs.columns.author'),
        width: 160,
        defaultHidden: true,
        sortKey: 'author',
        value: (r) => r.author.display_name,
        filter: {
          kind: 'checklist',
          options: facet('author').map((v) => ({
            value: v.value,
            label: v.label ?? v.value,
            count: v.count,
          })),
          value: search.author,
          single: true,
          onChange: (v) => onSearchChange({ author: v }),
        },
      }),
      col.actions<Run>({ title: (r) => r.name, actions: rowActions }),
    ],
    [t, slug, styles, search, onSearchChange, facets.data]
  )
  const prefs = useTablePrefs('runs', columns)

  const pill = (key: string, label: string, patch: Partial<RunListSearch>): ActivePill => ({
    key,
    label,
    onRemove: () => onSearchChange(patch),
  })
  const pills: ActivePill[] = [
    ...(search.name
      ? [pill('name', `${t('runs.columns.name')}: ${search.name}`, { name: undefined })]
      : []),
    ...(search.status ?? []).map((v) =>
      pill(`status:${v}`, `${t('runs.filters.status')}: ${t(`common.status.${v}`)}`, {
        status: search.status?.filter((x) => x !== v),
      })
    ),
    ...(search.kind ?? []).map((v) =>
      pill(`kind:${v}`, `${t('runs.filters.kind')}: ${v}`, {
        kind: search.kind?.filter((x) => x !== v),
      })
    ),
    ...(search.provider ?? []).map((v) =>
      pill(
        `prov:${v}`,
        `${t('runs.filters.provider')}: ${facet('provider_profile').find((f) => f.value === v)?.label ?? v}`,
        { provider: search.provider?.filter((x) => x !== v) }
      )
    ),
    ...(search.trigger ?? []).map((v) =>
      pill(`trig:${v}`, `${t('runs.filters.trigger')}: ${t(`common.trigger.${v}`)}`, {
        trigger: search.trigger?.filter((x) => x !== v),
      })
    ),
    ...(search.author ?? []).map((v) =>
      pill(`author:${v}`, `${t('runs.filters.author')}: ${authorName(v)}`, {
        author: search.author?.filter((x) => x !== v),
      })
    ),
    ...(search.labels
      ? [pill('labels', `${t('runs.filters.labels')}: ${search.labels}`, { labels: undefined })]
      : []),
    ...(search.test ? [pill('test', `test: ${search.test}`, { test: undefined })] : []),
    ...(search.suiteRun
      ? [
          pill('sr', `${t('runs.overview.trigger.suiteRun')}: ${search.suiteRun}`, {
            suiteRun: undefined,
          }),
        ]
      : []),
    ...(search.startedFrom || search.startedTo
      ? [
          pill(
            'started',
            `${t('runs.columns.started')}: ${search.startedFrom ? new Date(search.startedFrom).toLocaleDateString() : '…'} – ${search.startedTo ? new Date(search.startedTo).toLocaleDateString() : '…'}`,
            { startedFrom: undefined, startedTo: undefined }
          ),
        ]
      : []),
    ...(search.finishedFrom || search.finishedTo
      ? [
          pill(
            'finished',
            `${t('runs.columns.finished')}: ${search.finishedFrom ? new Date(search.finishedFrom).toLocaleDateString() : '…'} – ${search.finishedTo ? new Date(search.finishedTo).toLocaleDateString() : '…'}`,
            { finishedFrom: undefined, finishedTo: undefined }
          ),
        ]
      : []),
    ...(search.durMin !== undefined || search.durMax !== undefined
      ? [
          pill(
            'dur',
            `${t('runs.columns.duration')}: ${search.durMin ?? '…'} – ${search.durMax ?? '…'} ${t('common.table.seconds')}`,
            { durMin: undefined, durMax: undefined }
          ),
        ]
      : []),
  ]
  const clearAll = () =>
    onSearchChange(
      Object.fromEntries(FILTER_KEYS.map((k) => [k, undefined])) as Partial<RunListSearch>
    )
  const filtered = pills.length > 0 || !!search.q || !!search.favorites || !!search.kept

  return (
    <Page width="wide" fill>
      <PageHeader
        title={t('runs.title')}
        icon="play"
        badge={
          runningCount > 0 ? (
            <Badge text={t('runs.live', { count: runningCount })} color="blue" icon="sync" />
          ) : undefined
        }
        actions={
          <Button
            icon="plus"
            onClick={() => void navigate({ to: '/t/$slug/library/tests', params: { slug } })}
          >
            {t('runs.new')}
          </Button>
        }
      />
      <DataTableToolbar
        search={search.q}
        onSearch={(v) => onSearchChange({ q: v || undefined })}
        searchPlaceholder={t('runs.search')}
        toggles={[
          {
            key: 'fav',
            label: t('runs.filters.favorites'),
            value: !!search.favorites,
            onChange: (v) => onSearchChange({ favorites: v || undefined }),
          },
          {
            key: 'kept',
            label: t('runs.filters.kept'),
            value: !!search.kept,
            onChange: (v) => onSearchChange({ kept: v || undefined }),
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
      <DataTable<Run>
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
            sort: (s?.field as RunListSearch['sort']) ?? 'created_at',
            order: s?.order ?? 'desc',
          })
        }
        onOverlayChange={auto.setPaused}
        selected={selected}
        onSelectedChange={setSelected}
        bulkActions={(ids) => (
          <Button
            size="sm"
            variant="secondary"
            icon="columns"
            disabled={ids.length < 2 || ids.length > 16}
            tooltip={ids.length < 2 ? t('runs.selected.compareHint') : undefined}
            onClick={() =>
              void navigate({
                to: '/t/$slug/compare',
                params: { slug },
                search: { runs: ids } as never,
              })
            }
          >
            {t('runs.selected.compare', { count: ids.length })}
          </Button>
        )}
        rowHref={(r) => `/t/${slug}/runs/${r.id}`}
        filtered={filtered}
        onClearFilters={clearAll}
        empty={{
          message: t('runs.empty.title'),
          button: (
            <Button
              icon="vial"
              onClick={() => void navigate({ to: '/t/$slug/library/tests', params: { slug } })}
            >
              {t('runs.empty.cta')}
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

      {dialog.kind === 'clone' && (
        <RerunDrawer
          run={dialog.run}
          mode="clone"
          onClose={() => setDialog({ kind: 'none' })}
          onSubmit={(body) => rerun.mutateAsync({ run: dialog.run, body })}
        />
      )}
      {dialog.kind === 'saveAsTest' && (
        <SaveAsTestModal
          run={dialog.run}
          isOpen
          onClose={() => setDialog({ kind: 'none' })}
          onSubmit={(body) => saveAsTest.mutateAsync({ run: dialog.run, body })}
        />
      )}
      {dialog.kind === 'keep' && (
        <KeepExtendModal
          run={dialog.run}
          isOpen
          onClose={() => setDialog({ kind: 'none' })}
          onSubmit={(duration) => keepExtend.mutateAsync({ run: dialog.run, duration })}
        />
      )}
      {dialog.kind === 'share' && (
        <ShareModal
          isOpen
          onClose={() => setDialog({ kind: 'none' })}
          defaultTitle={dialog.run.name}
          onCreate={async (input) => {
            const share = await runMutations.share(slug, dialog.run.id, {
              scope: input.scope,
              ttl: input.ttl,
              title: input.title || undefined,
            })
            void invalidate()
            return share
          }}
        />
      )}
      <ConfirmModal
        isOpen={dialog.kind === 'delete'}
        title={t('common.confirm.deleteTitle', {
          name: dialog.kind === 'delete' ? dialog.run.name : '',
        })}
        body={t('runs.actions.deleteBody')}
        confirmText={t('common.confirm.yesDelete')}
        confirmButtonVariant="destructive"
        dismissText={t('common.actions.cancel')}
        disabled={remove.isPending}
        onConfirm={() => {
          if (dialog.kind === 'delete') remove.mutate(dialog.run)
        }}
        onDismiss={() => setDialog({ kind: 'none' })}
      />
    </Page>
  )
}
