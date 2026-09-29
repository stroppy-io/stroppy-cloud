import { keys } from '@api/queries/keys'
import { type ShareListQuery, shareMutations, shareQueries } from '@api/queries/results'
import type { Share } from '@api/types'
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
import { Button, ConfirmModal, Stack, useStyles2 } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import {
  keepPreviousData,
  useInfiniteQuery,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { ShareEditDrawer } from './ShareEditDrawer'

const SHARE_KINDS = ['run', 'suite_run', 'comparison'] as const

export const sharesSearchSchema = z.object({
  q: z.string().optional().catch(undefined),
  kind: z.enum(SHARE_KINDS).optional().catch(undefined),
  active: z.enum(['true', 'false']).optional().catch(undefined),
  sort: z
    .enum(['title', 'target', 'scope', 'expires_at', 'views', 'created_at'])
    .default('created_at')
    .catch('created_at'),
  order: orderSchema.default('desc').catch('desc'),
  size: sizeSchema,
  refresh: refreshSchema('off'),
})
export type SharesSearch = z.infer<typeof sharesSearchSchema>
export const SHARES_DEFAULTS = {
  sort: 'created_at',
  order: 'desc',
  size: 50,
  refresh: 'off',
} as const

export function toShareQuery(s: SharesSearch): ShareListQuery {
  return {
    target_kind: s.kind,
    active: s.active === undefined ? undefined : s.active === 'true',
    sort: s.sort,
    order: s.order,
    limit: s.size,
  }
}

// The shares endpoint has no text search: `q` narrows the loaded page client-side.
function matchesLocal(s: Share, q: string | undefined): boolean {
  const needle = q?.trim().toLowerCase()
  if (!needle) return true
  const hay = [s.title, s.url, s.target.name, s.created_by?.display_name]
    .filter(Boolean)
    .join(' ')
    .toLowerCase()
  return needle.split(/\s+/).every((w) => hay.includes(w))
}

const FILTER_KEYS = ['q', 'kind', 'active'] as const

const getStyles = (theme: GrafanaTheme2) => ({
  muted: css({ color: theme.colors.text.secondary }),
})

function shareState(s: Share): 'active' | 'revoked' | 'expired' {
  if (!s.active) return 'revoked'
  if (s.expires_at && new Date(s.expires_at).getTime() < Date.now()) return 'expired'
  return 'active'
}

function publicUrl(s: Share): string {
  return new URL(s.url, window.location.origin).toString()
}

function copyLink(s: Share, message: string) {
  void navigator.clipboard.writeText(publicUrl(s))
  toast.success(message)
}

// Same-origin path of the public page, so a row click opens it inside the app.
function publicPath(s: Share): string | undefined {
  const u = new URL(s.url, window.location.origin)
  return u.origin === window.location.origin ? `${u.pathname}${u.search}` : undefined
}

type Dialog = { kind: 'none' } | { kind: 'revoke' | 'delete'; share: Share }

export function SharesPage({
  search,
  onSearchChange,
}: {
  search: SharesSearch
  onSearchChange: (next: Partial<SharesSearch>) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug, can } = useTenant()
  const qc = useQueryClient()
  const query = useMemo(() => toShareQuery(search), [search])
  const auto = useAutoRefresh(search.refresh)
  const list = useInfiniteQuery({
    ...shareQueries.list(slug, query),
    placeholderData: keepPreviousData,
    refetchInterval: auto.refetchInterval,
  })
  const loaded = useMemo(() => list.data?.pages.flatMap((p) => p.data) ?? [], [list.data])
  const rows = useMemo(() => loaded.filter((s) => matchesLocal(s, search.q)), [loaded, search.q])
  const [editing, setEditing] = useState<Share | undefined>()
  const [dialog, setDialog] = useState<Dialog>({ kind: 'none' })
  const invalidate = () => qc.invalidateQueries({ queryKey: [...keys.t(slug), 'shares'] })

  const rebuild = useMutation({
    mutationFn: (id: string) => shareMutations.rebuild(slug, id),
    onSuccess: () => {
      void invalidate()
      toast.success(t('results.shares.actions.rebuilt'))
    },
    onError: (e: Error) => toast.error(e),
  })
  const revoke = useMutation({
    mutationFn: (id: string) => shareMutations.revoke(slug, id),
    onSuccess: () => {
      void invalidate()
      toast.success(t('results.shares.actions.revoked'))
      setDialog({ kind: 'none' })
    },
    onError: (e: Error) => {
      toast.error(e)
      setDialog({ kind: 'none' })
    },
  })
  const manage = can('share')

  const rowActions = (s: Share): RowAction[] => {
    const st = shareState(s)
    const alive = st === 'active'
    const noPerm = t('runs.actions.noPermission')
    return [
      {
        key: 'open',
        label: t('results.shares.actions.open'),
        icon: 'external-link-alt',
        onClick: () => window.open(s.url, '_blank', 'noopener'),
      },
      {
        key: 'copy',
        label: t('results.shares.actions.copy'),
        icon: 'link',
        onClick: () => copyLink(s, t('results.shares.actions.copied')),
      },
      {
        key: 'copyId',
        label: t('runs.actions.copyId'),
        icon: 'copy',
        onClick: () =>
          void navigator.clipboard
            .writeText(s.id)
            .then(() => toast.success(t('common.actions.copied')))
            .catch((e) => toast.error(e)),
      },
      {
        key: 'edit',
        group: true,
        label: t('results.shares.actions.edit'),
        icon: 'pen',
        disabled: !manage || !s.active,
        disabledReason: !manage ? noPerm : t('results.shares.actions.alreadyRevoked'),
        onClick: () => setEditing(s),
      },
      {
        key: 'rebuild',
        label: t('results.shares.actions.rebuild'),
        icon: 'sync',
        disabled: !manage || !alive || rebuild.isPending,
        disabledReason: !manage ? noPerm : t('results.shares.actions.notActive'),
        onClick: () => rebuild.mutate(s.id),
      },
      {
        key: 'revoke',
        group: true,
        label: t('results.shares.actions.revoke'),
        icon: 'times-circle',
        destructive: true,
        disabled: !manage || !s.active,
        disabledReason: !manage ? noPerm : t('results.shares.actions.alreadyRevoked'),
        onClick: () => setDialog({ kind: 'revoke', share: s }),
      },
      {
        key: 'delete',
        label: t('common.actions.delete'),
        icon: 'trash-alt',
        destructive: true,
        disabled: !manage,
        disabledReason: noPerm,
        // The API has no hard delete: revoking is what removes the public link.
        description: t('results.shares.actions.deleteHint'),
        onClick: () => setDialog({ kind: 'delete', share: s }),
      },
    ]
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure; search/rows drive the recompute
  const columns = useMemo<DataTableColumn<Share>[]>(
    () => [
      {
        // Share states read in the share's own words (active / revoked / expired, feminine).
        ...col.custom<Share>({
          id: 'state',
          title: t('results.shares.columns.state'),
          width: WIDTH.icon,
          align: 'center',
          tight: true,
          filter: {
            kind: 'checklist',
            options: [
              {
                value: 'true',
                label: t('results.shares.filters.activeOnly'),
                count: loaded.filter((r) => r.active).length,
              },
              {
                value: 'false',
                label: t('results.shares.filters.inactiveOnly'),
                count: loaded.filter((r) => !r.active).length,
              },
            ],
            value: search.active ? [search.active] : undefined,
            single: true,
            onChange: (v) =>
              onSearchChange({
                active: v?.length !== 1 ? undefined : (v[0] as SharesSearch['active']),
              }),
          },
          value: (s) => shareState(s),
          cell: (s) => {
            const st = shareState(s)
            return <StatusBadge status={st} label={t(`results.shares.state.${st}`)} iconOnly />
          },
        }),
        sticky: 'left',
        hideable: false,
      },
      col.identity<Share>({
        id: 'title',
        header: t('results.shares.columns.title'),
        sortOptions: [
          { key: 'title', label: t('results.shares.sort.title') },
          { key: 'target', label: t('results.shares.sort.target') },
        ],
        filter: {
          kind: 'checklist',
          options: SHARE_KINDS.map((k) => ({
            value: k,
            label: t(`results.shares.kind.${k}`),
            count: loaded.filter((r) => r.target.kind === k).length,
          })),
          value: search.kind ? [search.kind] : undefined,
          single: true,
          onChange: (v) =>
            onSearchChange({
              kind: v?.length !== 1 ? undefined : (v[0] as SharesSearch['kind']),
            }),
        },
        render: (s) => {
          const tg = s.target
          const kind = t(`results.shares.kind.${tg.kind}`)
          const name =
            tg.kind === 'comparison'
              ? t('results.shares.runsCount', { count: tg.run_ids?.length ?? 0 })
              : (tg.name ?? tg.id)
          const target =
            tg.kind === 'run' ? (
              <AppLink to="/t/$slug/runs/$id" params={{ slug, id: tg.id }} plain>
                {name}
              </AppLink>
            ) : tg.kind === 'suite_run' ? (
              <AppLink to="/t/$slug/suite-runs/$id" params={{ slug, id: tg.id }} plain>
                {name}
              </AppLink>
            ) : (
              <AppLink
                to="/t/$slug/compare"
                params={{ slug }}
                search={{ runs: tg.run_ids ?? [] } as never}
                plain
              >
                {name}
              </AppLink>
            )
          return {
            title: s.title ?? t('results.shares.untitled'),
            subtitle: (
              <span title={`${kind}: ${name}\n${publicUrl(s)}`}>
                {kind} · {target}
              </span>
            ),
          }
        },
      }),
      col.stack<Share>({
        id: 'scope',
        header: t('results.shares.columns.scope'),
        width: 220,
        sortKey: 'scope',
        value: (s) => s.scope,
        render: (s) => {
          const label = t(`results.shares.scope.${s.scope}`)
          const hint = t(`results.shares.scopeHint.${s.scope}`)
          return { primary: label, secondary: hint, title: `${label}\n${hint}` }
        },
      }),
      col.number<Share>({
        id: 'views',
        header: t('results.shares.columns.views'),
        width: 130,
        sortKey: 'views',
        unit: 'count',
        value: (s) => s.view_count ?? 0,
      }),
      col.stack<Share>({
        id: 'term',
        header: t('results.shares.columns.term'),
        width: 220,
        sortOptions: [
          { key: 'expires_at', label: t('results.shares.sort.expires') },
          { key: 'created_at', label: t('results.shares.sort.created') },
        ],
        value: (s) => s.expires_at ?? undefined,
        render: (s) => {
          const author = s.created_by?.display_name
          return {
            primary: s.revoked_at ? (
              <>
                {t('results.shares.revokedAt')} <RelativeTime value={s.revoked_at} />
              </>
            ) : s.expires_at ? (
              <>
                {t(
                  shareState(s) === 'expired'
                    ? 'results.shares.expiredAt'
                    : 'results.shares.expiresAt'
                )}{' '}
                <RelativeTime value={s.expires_at} />
              </>
            ) : (
              <span className={styles.muted}>{t('results.shares.noExpiry')}</span>
            ),
            secondary: (
              <>
                {t('results.shares.createdAt')} <RelativeTime value={s.created_at} />
                {author ? ` · ${author}` : ''}
              </>
            ),
            title: author,
          }
        },
      }),
      col.actions<Share>({
        title: (s) => s.title ?? t('results.shares.untitled'),
        actions: rowActions,
      }),
    ],
    [t, styles, slug, manage, rebuild, search, onSearchChange, loaded]
  )
  const prefs = useTablePrefs('shares', columns)

  const pill = (key: string, label: string, patch: Partial<SharesSearch>): ActivePill => ({
    key,
    label,
    onRemove: () => onSearchChange(patch),
  })
  const pills: ActivePill[] = [
    ...(search.kind
      ? [
          pill(
            'kind',
            `${t('results.shares.filters.kind')}: ${t(`results.shares.kind.${search.kind}`)}`,
            { kind: undefined }
          ),
        ]
      : []),
    ...(search.active
      ? [
          pill(
            'active',
            `${t('results.shares.filters.active')}: ${search.active === 'true' ? t('results.shares.filters.activeOnly') : t('results.shares.filters.inactiveOnly')}`,
            { active: undefined }
          ),
        ]
      : []),
  ]
  const clearAll = () =>
    onSearchChange(
      Object.fromEntries(FILTER_KEYS.map((k) => [k, undefined])) as Partial<SharesSearch>
    )
  const filtered = pills.length > 0 || !!search.q

  return (
    <Page width="wide" fill>
      <PageHeader
        title={t('results.shares.title')}
        subtitle={t('results.shares.subtitle')}
        icon="share-alt"
      />
      <DataTableToolbar
        search={search.q}
        onSearch={(v) => onSearchChange({ q: v || undefined })}
        searchPlaceholder={t('results.shares.search')}
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
      <DataTable<Share>
        fill
        columns={prefs.visible}
        density={prefs.density}
        data={rows}
        getRowId={(s) => s.id}
        loading={list.isPending}
        error={list.isError ? list.error : undefined}
        onRetry={() => void list.refetch()}
        sort={{ field: search.sort, order: search.order }}
        onSortChange={(s) =>
          onSearchChange({
            sort: (s?.field as SharesSearch['sort']) ?? 'created_at',
            order: s?.order ?? 'desc',
          })
        }
        onOverlayChange={auto.setPaused}
        rowHref={publicPath}
        filtered={filtered}
        onClearFilters={clearAll}
        empty={{ message: t('results.shares.empty.title') }}
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
      {editing && (
        <ShareEditDrawer slug={slug} share={editing} onClose={() => setEditing(undefined)} />
      )}
      <ConfirmModal
        isOpen={dialog.kind !== 'none'}
        title={
          dialog.kind === 'delete'
            ? t('common.confirm.deleteTitle', { name: dialog.share.title ?? dialog.share.url })
            : t('results.shares.revokeConfirm.title')
        }
        body={
          dialog.kind === 'none'
            ? ''
            : t('results.shares.revokeConfirm.body', { url: publicUrl(dialog.share) })
        }
        confirmText={
          dialog.kind === 'delete'
            ? t('common.confirm.yesDelete')
            : t('results.shares.actions.revoke')
        }
        confirmButtonVariant="destructive"
        dismissText={t('common.actions.cancel')}
        disabled={revoke.isPending}
        onConfirm={() => {
          if (dialog.kind !== 'none') revoke.mutate(dialog.share.id)
        }}
        onDismiss={() => setDialog({ kind: 'none' })}
      />
    </Page>
  )
}
