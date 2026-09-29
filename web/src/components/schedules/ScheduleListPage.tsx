import { type ScheduleListQuery, scheduleMutations, scheduleQueries } from '@api/queries/suites'
import type { Schedule } from '@api/types'
import { AppLink } from '@app/AppLink'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { col, WIDTH } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import { orderSchema, refreshSchema, sizeSchema } from '@components/DataTable/list-search'
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
import { describeCron } from '@helpers/cron'
import { zoneOffsetLabel } from '@helpers/timezones'
import { useTenant } from '@hooks/useTenant'
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

export const scheduleListSearchSchema = z.object({
  q: z.string().optional().catch(undefined),
  name: z.string().optional().catch(undefined),
  kind: z.enum(['test', 'suite']).optional().catch(undefined),
  target: z.string().optional().catch(undefined),
  enabled: z.boolean().optional().catch(undefined),
  sort: z
    .enum([
      'name',
      'enabled',
      'target',
      'cron',
      'next_run_at',
      'last_run_at',
      'created_at',
      'updated_at',
      'author',
    ])
    .default('next_run_at')
    .catch('next_run_at'),
  order: orderSchema.default('asc').catch('asc'),
  size: sizeSchema,
  refresh: refreshSchema('5s'),
})
export type ScheduleListSearch = z.infer<typeof scheduleListSearchSchema>
export const SCHEDULE_LIST_DEFAULTS = {
  sort: 'next_run_at',
  order: 'asc',
  size: 50,
  refresh: '5s',
} as const

export function toScheduleListQuery(s: ScheduleListSearch): ScheduleListQuery {
  return {
    target_kind: s.kind,
    enabled: s.enabled,
    sort: s.sort,
    order: s.order,
    limit: s.size,
  }
}

// The schedules endpoint has no text search / target id params: `q`, `name` and `target` narrow
// the loaded page client-side (the list is bounded).
function matchesLocal(s: Schedule, search: ScheduleListSearch): boolean {
  if (search.target && s.target.id !== search.target) return false
  const q = [search.q, search.name].filter(Boolean).join(' ').toLowerCase()
  if (!q) return true
  const hay = [s.name, s.target.name, s.cron, s.timezone].filter(Boolean).join(' ').toLowerCase()
  return q.split(/\s+/).every((w) => hay.includes(w))
}

const FILTER_KEYS = ['q', 'name', 'kind', 'target', 'enabled'] as const

const getStyles = (theme: GrafanaTheme2) => ({
  inline: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(0.75) }),
  muted: css({ color: theme.colors.text.secondary }),
  err: css({ color: theme.colors.error.text }),
  paused: css({ color: theme.colors.text.secondary, display: 'inline-flex' }),
})

type ScheduleState = 'active' | 'paused' | 'error'

// active / paused / the last firing failed (the schedule keeps running, but needs a look).
export function scheduleState(s: Schedule): ScheduleState {
  if (!s.enabled) return 'paused'
  if (s.last_run?.error || s.last_run?.status === 'failed') return 'error'
  return 'active'
}

// Status icon of a schedule with its own labels (the run-status vocabulary has no «paused»).
function ScheduleStatusIcon({ schedule }: { schedule: Schedule }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const st = scheduleState(schedule)
  const label = t(`schedules.state.${st}`)
  if (st === 'paused')
    return (
      <Tooltip content={label}>
        <span className={styles.paused} role="img" aria-label={label}>
          <Icon name="pause" />
        </span>
      </Tooltip>
    )
  return <StatusBadge status={st === 'error' ? 'failed' : 'active'} label={label} iconOnly />
}

// Next fire time as wall clock in the schedule's zone: «вт, 29 сент., 02:00».
function formatNext(iso: string, tz: string, lang: string): string {
  try {
    return new Intl.DateTimeFormat(lang, {
      timeZone: tz,
      weekday: 'short',
      day: 'numeric',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
      hourCycle: 'h23',
    }).format(new Date(iso))
  } catch {
    return new Date(iso).toLocaleString(lang)
  }
}

export function ScheduleListPage({
  search,
  onSearchChange,
}: {
  search: ScheduleListSearch
  onSearchChange: (next: Partial<ScheduleListSearch>) => void
}) {
  const { t, i18n } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug, can } = useTenant()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [deleting, setDeleting] = useState<Schedule | undefined>()
  const query = useMemo(() => toScheduleListQuery(search), [search])
  const auto = useAutoRefresh(search.refresh)
  const list = useInfiniteQuery({
    ...scheduleQueries.list(slug, query),
    placeholderData: keepPreviousData,
    refetchInterval: auto.refetchInterval,
  })
  const loaded = useMemo(() => list.data?.pages.flatMap((p) => p.data) ?? [], [list.data])
  const rows = useMemo(() => loaded.filter((s) => matchesLocal(s, search)), [loaded, search])
  const invalidate = () => qc.invalidateQueries({ queryKey: ['t', slug, 'schedules'] })
  const onError = (e: unknown) => toast.error(e)
  const toggle = useMutation({
    mutationFn: (s: Schedule) =>
      s.enabled ? scheduleMutations.pause(slug, s.id) : scheduleMutations.resume(slug, s.id),
    onSuccess: (s) => {
      toast.success(s.enabled ? t('schedules.toasts.resumed') : t('schedules.toasts.paused'), {
        description: s.name,
      })
      void invalidate()
    },
    onError,
  })
  const runNow = useMutation({
    mutationFn: (s: Schedule) => scheduleMutations.runNow(slug, s.id),
    onSuccess: (ref) => {
      toast.success(t('schedules.toasts.ran', { name: ref.name ?? ref.id }), {
        action: {
          label: t('schedules.toasts.ranOpen'),
          onClick: () =>
            ref.kind === 'run'
              ? void navigate({ to: '/t/$slug/runs/$id', params: { slug, id: ref.id } })
              : void navigate({ to: '/t/$slug/suite-runs/$id', params: { slug, id: ref.id } }),
        },
      })
      void invalidate()
      void qc.invalidateQueries({ queryKey: ['t', slug, 'runs'] })
      void qc.invalidateQueries({ queryKey: ['t', slug, 'suite-runs'] })
    },
    onError,
  })
  const remove = useMutation({
    mutationFn: (s: Schedule) => scheduleMutations.remove(slug, s.id),
    onSuccess: () => {
      toast.success(t('schedules.toasts.deleted'))
      void invalidate()
      setDeleting(undefined)
    },
    onError: (e) => {
      onError(e)
      setDeleting(undefined)
    },
  })
  const canEdit = can('edit-library')
  const canRun = can('run')

  const copy = (text: string) =>
    void navigator.clipboard
      .writeText(text)
      .then(() => toast.success(t('common.actions.copied')))
      .catch((e) => toast.error(e))

  const openTarget = (s: Schedule) =>
    s.target.kind === 'test'
      ? void navigate({ to: '/t/$slug/library/tests/$id', params: { slug, id: s.target.id } })
      : void navigate({ to: '/t/$slug/suites/$id', params: { slug, id: s.target.id } })

  const rowActions = (s: Schedule): RowAction[] => {
    const noPerm = t('runs.actions.noPermission')
    const open = () => void navigate({ to: '/t/$slug/schedules/$id', params: { slug, id: s.id } })
    return [
      { key: 'open', label: t('common.actions.open'), icon: 'eye', onClick: open },
      {
        key: 'runNow',
        label: t('schedules.actions.runNow'),
        icon: 'play',
        disabled: !canRun || runNow.isPending,
        disabledReason: noPerm,
        onClick: () => runNow.mutate(s),
      },
      {
        key: 'toggle',
        label: s.enabled ? t('schedules.actions.pause') : t('schedules.actions.resume'),
        icon: s.enabled ? 'pause' : 'play',
        disabled: !canEdit || toggle.isPending,
        disabledReason: noPerm,
        onClick: () => toggle.mutate(s),
      },
      {
        key: 'edit',
        group: true,
        label: t('schedules.actions.edit'),
        icon: 'edit',
        disabled: !canEdit,
        disabledReason: noPerm,
        onClick: open,
      },
      {
        key: 'target',
        label:
          s.target.kind === 'test'
            ? t('schedules.actions.openTest')
            : t('schedules.actions.openSuite'),
        description: s.target.name,
        icon: s.target.kind === 'test' ? 'vial' : 'layer-group',
        onClick: () => openTarget(s),
      },
      {
        key: 'copyLink',
        group: true,
        label: t('runs.actions.copyLink'),
        icon: 'link',
        onClick: () => copy(`${window.location.origin}/t/${slug}/schedules/${s.id}`),
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
        label: t('schedules.actions.delete'),
        icon: 'trash-alt',
        destructive: true,
        disabled: !canEdit,
        disabledReason: noPerm,
        onClick: () => setDeleting(s),
      },
    ]
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure; search/loaded drive the recompute
  const columns = useMemo<DataTableColumn<Schedule>[]>(
    () => [
      {
        // The kit's `col.status` speaks the run-status vocabulary; a schedule needs «paused».
        ...col.custom<Schedule>({
          id: 'status',
          title: t('schedules.columns.status'),
          width: WIDTH.icon,
          align: 'center',
          tight: true,
          sortKey: 'enabled',
          filter: {
            kind: 'checklist',
            options: [
              {
                value: 'on',
                label: t('schedules.filters.enabled'),
                count: loaded.filter((r) => r.enabled).length,
              },
              {
                value: 'off',
                label: t('schedules.filters.paused'),
                count: loaded.filter((r) => !r.enabled).length,
              },
            ],
            value: search.enabled === undefined ? undefined : search.enabled ? ['on'] : ['off'],
            onChange: (v) =>
              onSearchChange({ enabled: v?.length !== 1 ? undefined : v[0] === 'on' }),
          },
          value: (s) => scheduleState(s),
          cell: (s) => <ScheduleStatusIcon schedule={s} />,
        }),
        sticky: 'left',
        hideable: false,
      },
      col.identity<Schedule>({
        id: 'name',
        header: t('schedules.columns.name'),
        sortOptions: [
          { key: 'name', label: t('schedules.sort.name') },
          { key: 'target', label: t('schedules.sort.target') },
        ],
        filter: {
          kind: 'checklist',
          options: (['test', 'suite'] as const).map((k) => ({
            value: k,
            label:
              k === 'test' ? t('schedules.filters.kindTest') : t('schedules.filters.kindSuite'),
            count: loaded.filter((r) => r.target.kind === k).length,
          })),
          value: search.kind ? [search.kind] : undefined,
          single: true,
          onChange: (v) =>
            onSearchChange({
              kind: v?.length !== 1 ? undefined : (v[0] as ScheduleListSearch['kind']),
            }),
        },
        render: (s) => {
          const isTest = s.target.kind === 'test'
          const name = s.target.name ?? t('schedules.target.missing')
          return {
            title: s.name,
            link: { to: '/t/$slug/schedules/$id', params: { slug, id: s.id } },
            subtitle: (
              <span title={`${t(`schedules.target.${s.target.kind}`)}: ${name}`}>
                <Icon name={isTest ? 'vial' : 'layer-group'} size="xs" />{' '}
                {isTest ? (
                  <AppLink to="/t/$slug/library/tests/$id" params={{ slug, id: s.target.id }} plain>
                    {name}
                  </AppLink>
                ) : (
                  <AppLink to="/t/$slug/suites/$id" params={{ slug, id: s.target.id }} plain>
                    {name}
                  </AppLink>
                )}
              </span>
            ),
          }
        },
      }),
      col.stack<Schedule>({
        id: 'when',
        header: t('schedules.columns.when'),
        minWidth: 220,
        sortKey: 'cron',
        value: (s) => s.cron,
        render: (s) => {
          const human = describeCron(s.cron, (k, o) => String(t(k, o as never)), i18n.language)
          const tz = `${s.timezone} ${zoneOffsetLabel(s.timezone)}`.trim()
          return { primary: human, secondary: tz, title: `${human}\n${s.cron} · ${tz}` }
        },
      }),
      col.stack<Schedule>({
        id: 'next',
        header: t('schedules.columns.nextRun'),
        width: 200,
        sortKey: 'next_run_at',
        value: (s) => s.next_run_at ?? undefined,
        render: (s) => {
          if (!s.enabled)
            return { primary: <span className={styles.muted}>{t('schedules.paused')}</span> }
          if (!s.next_run_at) return { primary: undefined }
          const when = formatNext(s.next_run_at, s.timezone, i18n.language)
          return {
            primary: when,
            secondary: <RelativeTime value={s.next_run_at} />,
            title: `${when} (${s.timezone})`,
          }
        },
      }),
      col.stack<Schedule>({
        id: 'last',
        header: t('schedules.columns.lastRun'),
        width: 220,
        sortKey: 'last_run_at',
        value: (s) => s.last_run?.at,
        render: (s) => {
          const lr = s.last_run
          if (!lr)
            return { primary: <span className={styles.muted}>{t('schedules.neverRan')}</span> }
          const name = lr.name ?? lr.id
          return {
            primary: (
              <span className={styles.inline}>
                <StatusBadge status={lr.status} iconOnly />
                <RelativeTime value={lr.at} />
              </span>
            ),
            secondary: lr.error ? (
              <span className={styles.err}>{lr.error}</span>
            ) : lr.kind === 'run' ? (
              <AppLink to="/t/$slug/runs/$id" params={{ slug, id: lr.id }} plain>
                {name}
              </AppLink>
            ) : (
              <AppLink to="/t/$slug/suite-runs/$id" params={{ slug, id: lr.id }} plain>
                {name}
              </AppLink>
            ),
            title: lr.error ? `${name}\n${lr.error}` : name,
          }
        },
      }),
      col.text<Schedule>({
        id: 'author',
        header: t('schedules.columns.author'),
        width: 160,
        defaultHidden: true,
        sortKey: 'author',
        value: (s) => s.author?.display_name,
      }),
      col.actions<Schedule>({ title: (s) => s.name, actions: rowActions }),
    ],
    [t, i18n.language, slug, styles, onSearchChange, search, loaded]
  )
  const prefs = useTablePrefs('schedules', columns)

  const pill = (key: string, label: string, patch: Partial<ScheduleListSearch>): ActivePill => ({
    key,
    label,
    onRemove: () => onSearchChange(patch),
  })
  const pills: ActivePill[] = [
    ...(search.name
      ? [pill('name', `${t('schedules.columns.name')}: ${search.name}`, { name: undefined })]
      : []),
    ...(search.kind
      ? [
          pill(
            'kind',
            `${t('schedules.filters.kind')}: ${search.kind === 'test' ? t('schedules.filters.kindTest') : t('schedules.filters.kindSuite')}`,
            { kind: undefined }
          ),
        ]
      : []),
    ...(search.target
      ? [
          pill(
            'target',
            `${t('schedules.columns.target')}: ${loaded.find((s) => s.target.id === search.target)?.target.name ?? search.target}`,
            { target: undefined }
          ),
        ]
      : []),
    ...(search.enabled !== undefined
      ? [
          pill(
            'enabled',
            `${t('schedules.filters.state')}: ${search.enabled ? t('schedules.filters.enabled') : t('schedules.filters.paused')}`,
            { enabled: undefined }
          ),
        ]
      : []),
  ]
  const clearAll = () =>
    onSearchChange(
      Object.fromEntries(FILTER_KEYS.map((k) => [k, undefined])) as Partial<ScheduleListSearch>
    )
  const filtered = pills.length > 0 || !!search.q

  return (
    <Page width="wide" fill>
      <PageHeader
        title={t('schedules.title')}
        icon="clock-nine"
        actions={
          <Button
            icon="plus"
            disabled={!canEdit}
            onClick={() => void navigate({ to: '/t/$slug/schedules/new', params: { slug } })}
          >
            {t('schedules.new')}
          </Button>
        }
      />
      <DataTableToolbar
        search={search.q}
        onSearch={(v) => onSearchChange({ q: v || undefined })}
        searchPlaceholder={t('schedules.search')}
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
      <DataTable<Schedule>
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
            sort: (s?.field as ScheduleListSearch['sort']) ?? 'next_run_at',
            order: s?.order ?? 'asc',
          })
        }
        onOverlayChange={auto.setPaused}
        rowHref={(r) => `/t/${slug}/schedules/${r.id}`}
        filtered={filtered}
        onClearFilters={clearAll}
        empty={{
          message: t('schedules.empty.title'),
          button: (
            <Button
              icon="plus"
              disabled={!canEdit}
              onClick={() => void navigate({ to: '/t/$slug/schedules/new', params: { slug } })}
            >
              {t('schedules.empty.cta')}
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
      <ConfirmModal
        isOpen={!!deleting}
        title={t('common.confirm.deleteTitle', { name: deleting?.name ?? '' })}
        body={t('schedules.confirm.deleteBody')}
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
