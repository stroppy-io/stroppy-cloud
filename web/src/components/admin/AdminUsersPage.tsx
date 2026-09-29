import { adminMutations, adminQueries } from '@api/queries/admin'
import type { AdminUser } from '@api/types'
import { PageFill } from '@app/Page'
import { toast } from '@app/Toaster'
import { Dash } from '@components/DataTable/cells'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { DataTableToolbar } from '@components/DataTable/Toolbar'
import { RelativeTime } from '@components/RelativeTime'
import { Badge, Button, Stack } from '@grafana/ui'
import { formatDateTime, relativeTime } from '@helpers/time'
import { useCopy } from '@hooks/useCopy'
import { useMe } from '@hooks/useMe'
import {
  keepPreviousData,
  useInfiniteQuery,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

export const adminUsersSearchSchema = z.object({
  search: z.string().optional().catch(undefined),
  admins: z.boolean().optional().catch(undefined),
})
export type AdminUsersSearch = z.infer<typeof adminUsersSearchSchema>

export function AdminUsersPage({
  search,
  onSearchChange,
}: {
  search: AdminUsersSearch
  onSearchChange: (next: Partial<AdminUsersSearch>) => void
}) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const me = useMe()
  const copy = useCopy()
  const list = useInfiniteQuery({
    ...adminQueries.users({ search: search.search, platform_admin: search.admins }),
    placeholderData: keepPreviousData,
  })
  const rows = useMemo(() => list.data?.pages.flatMap((p) => p.data) ?? [], [list.data])
  const patch = useMutation({
    mutationFn: ({ id, admin }: { id: string; admin: boolean }) =>
      adminMutations.patchUser(id, { is_platform_admin: admin }),
    onSuccess: async (u) => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['admin', 'users'] }),
        qc.invalidateQueries({ queryKey: ['me'] }),
      ])
      toast.success(
        u.is_platform_admin
          ? t('admin.users.granted', { name: u.display_name })
          : t('admin.users.revoked', { name: u.display_name })
      )
    },
    onError: (e) => toast.error(e),
  })
  const rowActions = (u: AdminUser): RowAction[] => {
    const fromConfig = u.admin_source === 'config'
    const self = u.id === me.id
    return [
      {
        key: 'admin',
        label: u.is_platform_admin ? t('admin.users.revoke') : t('admin.users.grant'),
        icon: 'shield',
        disabled: fromConfig || self || patch.isPending,
        disabledReason: fromConfig ? t('admin.users.fromConfig') : t('admin.users.self'),
        onClick: () => patch.mutate({ id: u.id, admin: !u.is_platform_admin }),
      },
      {
        key: 'copyEmail',
        group: true,
        label: t('admin.table.copyEmail'),
        icon: 'envelope',
        onClick: () => copy(u.email),
      },
      {
        key: 'copyId',
        label: t('admin.table.copyId'),
        icon: 'copy',
        onClick: () => copy(u.id),
      },
    ]
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure over mutation state
  const columns = useMemo<DataTableColumn<AdminUser>[]>(
    () => [
      col.identity<AdminUser>({
        id: 'name',
        header: t('common.fields.name'),
        render: (u) => ({
          title: u.display_name,
          subtitle: u.email,
          badges: (
            <>
              {u.id === me.id && <Badge text={t('common.misc.you')} color="blue" />}
              {u.admin_source === 'config' && (
                <Badge text={t('admin.users.config')} color="darkgrey" icon="cog" />
              )}
            </>
          ),
        }),
      }),
      col.bool<AdminUser>({
        id: 'admin',
        header: t('admin.users.platformAdmin'),
        width: 176,
        value: (u) => u.is_platform_admin,
      }),
      col.text<AdminUser>({
        id: 'owned',
        header: t('admin.users.ownedTenant'),
        width: 220,
        value: (u) => u.owned_tenant?.name,
      }),
      col.number<AdminUser>({
        id: 'memberships',
        header: t('admin.users.memberships'),
        width: 112,
        value: (u) => u.memberships,
        format: (v) => String(v),
      }),
      col.stack<AdminUser>({
        id: 'activity',
        header: t('admin.table.activity'),
        width: 180,
        value: (u) => u.last_seen_at ?? u.created_at,
        render: (u) => ({
          primary: u.last_seen_at ? <RelativeTime value={u.last_seen_at} /> : <Dash />,
          secondary: t('admin.table.createdAgo', { time: relativeTime(u.created_at) }),
          title: [
            u.last_seen_at
              ? t('admin.table.lastSeenAt', { time: formatDateTime(u.last_seen_at) })
              : undefined,
            t('admin.table.createdAt', { time: formatDateTime(u.created_at) }),
          ]
            .filter(Boolean)
            .join('\n'),
        }),
      }),
      col.actions<AdminUser>({ title: (u) => u.display_name, actions: rowActions }),
    ],
    [t, me.id, patch.isPending]
  )
  const clearAll = () => onSearchChange({ search: undefined, admins: undefined })
  return (
    <PageFill>
      <DataTableToolbar
        search={search.search}
        onSearch={(v) => onSearchChange({ search: v || undefined })}
        searchPlaceholder={t('admin.users.search')}
        toggles={[
          {
            key: 'admins',
            label: t('admin.users.onlyAdmins'),
            value: !!search.admins,
            onChange: (v) => onSearchChange({ admins: v || undefined }),
          },
        ]}
        count={rows.length}
      />
      <DataTable<AdminUser>
        fill
        columns={columns}
        data={rows}
        getRowId={(u) => u.id}
        loading={list.isPending}
        error={list.isError ? list.error : undefined}
        onRetry={() => void list.refetch()}
        filtered={!!search.search || !!search.admins}
        onClearFilters={clearAll}
        empty={{ message: t('admin.users.empty') }}
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
    </PageFill>
  )
}
