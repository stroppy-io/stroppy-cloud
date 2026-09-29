import {
  type AdminTenant,
  type AdminTenantsQuery,
  adminMutations,
  adminQueries,
} from '@api/queries/admin'
import { AppLink } from '@app/AppLink'
import { PageFill } from '@app/Page'
import { toast } from '@app/Toaster'
import { ConfirmAction } from '@components/ConfirmAction'
import { Dash } from '@components/DataTable/cells'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import { orderSchema } from '@components/DataTable/list-search'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { type ActivePill, DataTableToolbar } from '@components/DataTable/Toolbar'
import { KeyValueList } from '@components/KeyValueList'
import { RelativeTime } from '@components/RelativeTime'
import { StatusBadge } from '@components/StatusBadge'
import { Badge, Button, ConfirmModal, Drawer, Stack, Text } from '@grafana/ui'
import { formatDuration } from '@helpers/format'
import { formatDateTime, relativeTime } from '@helpers/time'
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
import { AssignOwnerModal } from './AssignOwnerModal'
import { SuspendModal } from './SuspendModal'
import { TenantLimitsDrawer } from './TenantLimitsDrawer'

// Server sort keys of adminListTenants (default created_at).
const TENANT_SORT_KEYS = [
  'name',
  'slug',
  'status',
  'members',
  'runs',
  'created_at',
  'last_activity',
] as const satisfies readonly NonNullable<AdminTenantsQuery['sort']>[]

export const adminTenantsSearchSchema = z.object({
  search: z.string().optional().catch(undefined),
  status: z.enum(['active', 'orphaned', 'suspended']).optional().catch(undefined),
  sort: z.enum(TENANT_SORT_KEYS).default('created_at').catch('created_at'),
  order: orderSchema.default('desc').catch('desc'),
})
export type AdminTenantsSearch = z.infer<typeof adminTenantsSearchSchema>
export const ADMIN_TENANTS_DEFAULTS = { sort: 'created_at', order: 'desc' } as const

type Dialog =
  | { kind: 'detail'; tenant: AdminTenant }
  | { kind: 'limits'; tenant: AdminTenant }
  | { kind: 'owner'; tenant: AdminTenant }
  | { kind: 'suspend'; tenant: AdminTenant }
  | { kind: 'delete'; tenant: AdminTenant }

const TENANT_STATUSES = ['active', 'suspended', 'orphaned'] as const

export function AdminTenantsPage({
  search,
  onSearchChange,
}: {
  search: AdminTenantsSearch
  onSearchChange: (next: Partial<AdminTenantsSearch>) => void
}) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const copy = useCopy()
  const list = useInfiniteQuery({
    ...adminQueries.tenants({
      search: search.search,
      status: search.status,
      sort: search.sort,
      order: search.order,
    }),
    placeholderData: keepPreviousData,
  })
  const rows = useMemo(() => list.data?.pages.flatMap((p) => p.data) ?? [], [list.data])
  const [dialog, setDialog] = useState<Dialog>()
  const invalidate = () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: ['admin', 'tenants'] }),
      qc.invalidateQueries({ queryKey: adminQueries.status().queryKey }),
      qc.invalidateQueries({ queryKey: ['me'] }),
    ])
  const fail = (e: unknown) => toast.error(e)

  const resume = useMutation({
    mutationFn: (slug: string) => adminMutations.resumeTenant(slug),
    onSuccess: async (tn) => {
      await invalidate()
      toast.success(t('admin.tenants.resumed', { name: tn.name }))
    },
    onError: fail,
  })
  const remove = useMutation({
    mutationFn: (slug: string) => adminMutations.deleteTenant(slug),
    onSuccess: async () => {
      await invalidate()
      toast.success(t('admin.tenants.deleted'))
      setDialog(undefined)
    },
    onError: fail,
  })

  const rowActions = (tn: AdminTenant): RowAction[] => {
    const suspended = tn.status === 'suspended'
    const live = (tn.counters?.runs_running ?? 0) > 0
    return [
      {
        key: 'detail',
        label: t('admin.tenants.details'),
        icon: 'eye',
        onClick: () => setDialog({ kind: 'detail', tenant: tn }),
      },
      {
        key: 'open',
        label: t('admin.tenants.open'),
        icon: 'external-link-alt',
        onClick: () => void navigate({ to: '/t/$slug', params: { slug: tn.slug } }),
      },
      {
        key: 'limits',
        group: true,
        label: t('admin.tenants.editLimits'),
        icon: 'sliders-v-alt',
        onClick: () => setDialog({ kind: 'limits', tenant: tn }),
      },
      {
        key: 'owner',
        label: t('admin.tenants.assignOwner'),
        icon: 'user',
        onClick: () => setDialog({ kind: 'owner', tenant: tn }),
      },
      {
        key: 'resume',
        label: t('admin.tenants.resume'),
        icon: 'play',
        disabled: !suspended || resume.isPending,
        disabledReason: t('admin.tenants.notSuspended'),
        onClick: () => resume.mutate(tn.slug),
      },
      {
        key: 'copySlug',
        group: true,
        label: t('admin.table.copySlug'),
        icon: 'copy',
        onClick: () => copy(tn.slug),
      },
      {
        key: 'copyId',
        label: t('admin.table.copyId'),
        icon: 'copy',
        onClick: () => copy(tn.id),
      },
      {
        key: 'suspend',
        group: true,
        label: t('admin.tenants.suspend'),
        icon: 'pause',
        destructive: true,
        disabled: suspended,
        disabledReason: t('admin.tenants.alreadySuspended'),
        onClick: () => setDialog({ kind: 'suspend', tenant: tn }),
      },
      {
        key: 'delete',
        label: t('common.actions.delete'),
        icon: 'trash-alt',
        destructive: true,
        disabled: live,
        disabledReason: t('admin.tenants.deleteBlocked'),
        onClick: () => setDialog({ kind: 'delete', tenant: tn }),
      },
    ]
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure over mutation state
  const columns = useMemo<DataTableColumn<AdminTenant>[]>(
    () => [
      col.status<AdminTenant>({
        id: 'status',
        title: t('common.fields.status'),
        status: (x) => x.status,
        sortKey: 'status',
        filter: {
          kind: 'checklist',
          single: true,
          options: TENANT_STATUSES.map((s) => ({ value: s, label: t(`common.status.${s}`) })),
          value: search.status ? [search.status] : undefined,
          onChange: (v) => onSearchChange({ status: v?.[0] as AdminTenantsSearch['status'] }),
        },
      }),
      col.identity<AdminTenant>({
        id: 'name',
        header: t('common.fields.name'),
        sortOptions: [
          { key: 'name', label: t('admin.sort.name') },
          { key: 'slug', label: t('admin.sort.slug') },
        ],
        render: (x) => ({
          title: x.name,
          subtitle: [x.slug, x.status === 'suspended' ? x.suspended_reason : x.description]
            .filter(Boolean)
            .join(' · '),
          badges:
            x.limits?.source === 'tenant_override' ? (
              <Badge text={t('admin.tenants.limitsOverride')} color="purple" icon="sliders-v-alt" />
            ) : undefined,
        }),
      }),
      col.text<AdminTenant>({
        id: 'owner',
        header: t('admin.tenants.owner'),
        width: 200,
        value: (x) => (x.status === 'orphaned' ? t('admin.tenants.noOwner') : x.owner.display_name),
      }),
      col.number<AdminTenant>({
        id: 'members',
        header: t('admin.tenants.members'),
        width: 128,
        sortKey: 'members',
        value: (x) => x.counters?.members ?? x.member_count,
        format: (v) => String(v),
      }),
      col.stack<AdminTenant>({
        id: 'runs',
        header: t('admin.tenants.runs'),
        width: 128,
        align: 'right',
        sortKey: 'runs',
        value: (x) => x.counters?.runs_total,
        render: (x) => {
          const c = x.counters
          return {
            primary: String(c?.runs_total ?? 0),
            secondary: c?.runs_running ? t('admin.tenants.live', { count: c.runs_running }) : '',
          }
        },
      }),
      col.stack<AdminTenant>({
        id: 'activity',
        header: t('admin.table.activity'),
        width: 180,
        sortOptions: [
          { key: 'last_activity', label: t('admin.sort.lastActivity') },
          { key: 'created_at', label: t('admin.sort.created') },
        ],
        value: (x) => x.last_activity_at ?? x.created_at,
        render: (x) => ({
          primary: x.last_activity_at ? <RelativeTime value={x.last_activity_at} /> : <Dash />,
          secondary: t('admin.table.createdAgo', { time: relativeTime(x.created_at) }),
          title: [
            x.last_activity_at
              ? t('admin.table.lastActivityAt', { time: formatDateTime(x.last_activity_at) })
              : undefined,
            t('admin.table.createdAt', { time: formatDateTime(x.created_at) }),
          ]
            .filter(Boolean)
            .join('\n'),
        }),
      }),
      col.actions<AdminTenant>({ title: (x) => x.name, actions: rowActions }),
    ],
    [t, search.status, onSearchChange, resume.isPending]
  )

  const pills: ActivePill[] = [
    ...(search.status
      ? [
          {
            key: 'status',
            label: `${t('common.fields.status')}: ${t(`common.status.${search.status}`)}`,
            onRemove: () => onSearchChange({ status: undefined }),
          },
        ]
      : []),
  ]
  const clearAll = () => onSearchChange({ status: undefined, search: undefined })

  return (
    <PageFill>
      <DataTableToolbar
        search={search.search}
        onSearch={(v) => onSearchChange({ search: v || undefined })}
        searchPlaceholder={t('admin.tenants.search')}
        pills={pills}
        onClearAll={pills.length ? () => onSearchChange({ status: undefined }) : undefined}
        count={rows.length}
      />
      <DataTable<AdminTenant>
        fill
        columns={columns}
        data={rows}
        getRowId={(x) => x.id}
        loading={list.isPending}
        error={list.isError ? list.error : undefined}
        onRetry={() => void list.refetch()}
        sort={{ field: search.sort, order: search.order }}
        onSortChange={(s) =>
          onSearchChange({
            sort: (s?.field as AdminTenantsSearch['sort']) ?? 'created_at',
            order: s?.order ?? 'desc',
          })
        }
        onRowClick={(tn) => setDialog({ kind: 'detail', tenant: tn })}
        filtered={pills.length > 0 || !!search.search}
        onClearFilters={clearAll}
        empty={{ message: t('admin.tenants.empty') }}
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

      {dialog?.kind === 'detail' && (
        <TenantDetailDrawer
          tenant={rows.find((x) => x.id === dialog.tenant.id) ?? dialog.tenant}
          onClose={() => setDialog(undefined)}
          onLimits={() => setDialog({ kind: 'limits', tenant: dialog.tenant })}
          onOwner={() => setDialog({ kind: 'owner', tenant: dialog.tenant })}
          onSuspend={() => setDialog({ kind: 'suspend', tenant: dialog.tenant })}
          onResume={() => resume.mutate(dialog.tenant.slug)}
          onDelete={() => remove.mutateAsync(dialog.tenant.slug).then(() => setDialog(undefined))}
        />
      )}
      {dialog?.kind === 'limits' && (
        <TenantLimitsDrawer
          tenant={dialog.tenant}
          onClose={() => setDialog(undefined)}
          onSaved={async () => {
            await invalidate()
            setDialog(undefined)
          }}
        />
      )}
      {dialog?.kind === 'owner' && (
        <AssignOwnerModal
          tenant={dialog.tenant}
          onClose={() => setDialog(undefined)}
          onSaved={async () => {
            await invalidate()
            setDialog(undefined)
          }}
        />
      )}
      {dialog?.kind === 'suspend' && (
        <SuspendModal
          tenant={dialog.tenant}
          onClose={() => setDialog(undefined)}
          onSaved={async () => {
            await invalidate()
            setDialog(undefined)
          }}
        />
      )}
      {dialog?.kind === 'delete' && (
        <ConfirmModal
          isOpen
          title={t('admin.tenants.deleteTitle', { name: dialog.tenant.name })}
          body={
            <Stack direction="column" gap={1}>
              <span>{t('admin.tenants.deleteBody')}</span>
              <span>{t('settings.general.deleteConfirm', { slug: dialog.tenant.slug })}</span>
            </Stack>
          }
          confirmationText={dialog.tenant.slug}
          confirmText={t('common.actions.delete')}
          confirmButtonVariant="destructive"
          dismissText={t('common.actions.cancel')}
          disabled={remove.isPending}
          onConfirm={() => remove.mutate(dialog.tenant.slug)}
          onDismiss={() => setDialog(undefined)}
        />
      )}
    </PageFill>
  )
}

function TenantDetailDrawer({
  tenant: tn,
  onClose,
  onLimits,
  onOwner,
  onSuspend,
  onResume,
  onDelete,
}: {
  tenant: AdminTenant
  onClose: () => void
  onLimits: () => void
  onOwner: () => void
  onSuspend: () => void
  onResume: () => void
  onDelete: () => Promise<unknown>
}) {
  const { t } = useTranslation()
  const live = (tn.counters?.runs_running ?? 0) > 0
  return (
    <Drawer title={tn.name} subtitle={tn.slug} size="md" onClose={onClose}>
      <Stack direction="column" gap={3}>
        <Stack gap={1} alignItems="center" wrap="wrap">
          <StatusBadge status={tn.status} />
          {tn.suspended_reason && (
            <Text color="secondary" variant="bodySmall">
              {tn.suspended_reason}
            </Text>
          )}
          <AppLink to="/t/$slug" params={{ slug: tn.slug }}>
            {t('admin.tenants.open')}
          </AppLink>
        </Stack>
        <KeyValueList
          title={t('admin.tenants.about')}
          items={[
            {
              label: t('admin.tenants.owner'),
              value: tn.owner.display_name ?? t('admin.tenants.noOwner'),
            },
            { label: t('common.fields.description'), value: tn.description || '—' },
            { label: t('settings.general.publicName'), value: tn.public_name || '—' },
            {
              label: t('admin.tenants.namespace'),
              value: tn.graphene_namespace ?? `st-${tn.slug}`,
            },
            { label: t('common.fields.created'), value: <RelativeTime value={tn.created_at} /> },
            {
              label: t('admin.tenants.lastActivity'),
              value: <RelativeTime value={tn.last_activity_at} />,
            },
          ]}
        />
        <KeyValueList
          title={t('admin.tenants.counters')}
          items={[
            {
              label: t('admin.tenants.members'),
              value: tn.counters?.members ?? tn.member_count ?? 0,
            },
            { label: t('admin.tenants.runs'), value: tn.counters?.runs_total ?? 0 },
            { label: t('common.status.running'), value: tn.counters?.runs_running ?? 0 },
            { label: t('admin.status.keptStands'), value: tn.counters?.kept_stands ?? 0 },
            { label: t('settings.tabs.providers'), value: tn.counters?.providers ?? 0 },
          ]}
        />
        {tn.limits && (
          <KeyValueList
            title={`${t('settings.limits.title')} · ${tn.limits.source === 'tenant_override' ? t('settings.limits.sourceOverride') : t('settings.limits.sourceDefault')}`}
            items={[
              {
                label: t('settings.limits.maxConcurrentRuns'),
                value: tn.limits.max_concurrent_runs,
              },
              {
                label: t('settings.limits.maxMachinesPerRun'),
                value: tn.limits.max_machines_per_run,
              },
              { label: t('settings.limits.maxSize'), value: tn.limits.max_size },
              { label: t('settings.limits.maxKeep'), value: formatDuration(tn.limits.max_keep) },
              {
                label: t('settings.limits.retentionMax'),
                value: t('settings.limits.days', { count: tn.limits.run_retention_max_days }),
              },
            ]}
          />
        )}
        <Stack gap={1} wrap="wrap">
          <Button variant="secondary" icon="sliders-v-alt" onClick={onLimits}>
            {t('admin.tenants.editLimits')}
          </Button>
          <Button variant="secondary" icon="user" onClick={onOwner}>
            {t('admin.tenants.assignOwner')}
          </Button>
          {tn.status === 'suspended' ? (
            <Button variant="secondary" icon="play" onClick={onResume}>
              {t('admin.tenants.resume')}
            </Button>
          ) : (
            <Button variant="secondary" icon="pause" onClick={onSuspend}>
              {t('admin.tenants.suspend')}
            </Button>
          )}
          <ConfirmAction
            title={t('admin.tenants.deleteTitle', { name: tn.name })}
            body={
              <Stack direction="column" gap={1}>
                <span>{t('admin.tenants.deleteBody')}</span>
                {live && <Text color="warning">{t('admin.tenants.deleteBlocked')}</Text>}
                <span>{t('settings.general.deleteConfirm', { slug: tn.slug })}</span>
              </Stack>
            }
            confirmationText={tn.slug}
            confirmText={t('common.actions.delete')}
            onConfirm={onDelete}
          >
            {(open) => (
              <Button variant="destructive" icon="trash-alt" onClick={open}>
                {t('common.actions.delete')}
              </Button>
            )}
          </ConfirmAction>
        </Stack>
      </Stack>
    </Drawer>
  )
}
