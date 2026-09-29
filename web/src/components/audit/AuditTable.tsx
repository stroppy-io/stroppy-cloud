import type { AuditEntry } from '@api/types'
import { PageFill } from '@app/Page'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import { useTablePrefs } from '@components/DataTable/prefs'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { TableSettings } from '@components/DataTable/TableSettings'
import { type ActivePill, DataTableToolbar } from '@components/DataTable/Toolbar'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Modal, Stack, useStyles2 } from '@grafana/ui'
import { formatDateTime } from '@helpers/time'
import { useCopy } from '@hooks/useCopy'
import { useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

// URL state shared by the tenant and the admin audit screens.
export const auditSearchSchema = z.object({
  action: z.string().optional().catch(undefined),
  actor: z.string().optional().catch(undefined),
  since: z.enum(['1h', '24h', '7d', '30d']).optional().catch(undefined),
})
export type AuditSearch = z.infer<typeof auditSearchSchema>

export function sinceToIso(since: AuditSearch['since']): string | undefined {
  if (!since) return undefined
  const ms = { '1h': 3600_000, '24h': 86400_000, '7d': 7 * 86400_000, '30d': 30 * 86400_000 }[since]
  return new Date(Date.now() - ms).toISOString()
}

const getStyles = (theme: GrafanaTheme2) => ({
  details: css({
    margin: 0,
    marginTop: theme.spacing(0.5),
    padding: theme.spacing(1),
    background: theme.colors.background.secondary,
    borderRadius: theme.shape.radius.default,
    fontSize: theme.typography.bodySmall.fontSize,
    fontFamily: theme.typography.fontFamilyMonospace,
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-word',
    maxHeight: 420,
    overflow: 'auto',
  }),
})

const SINCE = ['1h', '24h', '7d', '30d'] as const

function hasDetails(e: AuditEntry): boolean {
  return !!e.details && Object.keys(e.details).length > 0
}

function targetLabel(e: AuditEntry): string {
  return [e.target.kind, e.target.name ?? e.target.id].filter(Boolean).join(' · ')
}

function targetLink(
  e: AuditEntry,
  slug: string | undefined
):
  | {
      to:
        | '/t/$slug/runs/$id'
        | '/t/$slug/library/tests/$id'
        | '/t/$slug/library/databases/$id'
        | '/t/$slug/library/workloads/$id'
      params: { slug: string; id: string }
    }
  | undefined {
  if (!slug || !e.target.id) return undefined
  const params = { slug, id: e.target.id }
  switch (e.target.kind) {
    case 'run':
      return { to: '/t/$slug/runs/$id', params }
    case 'test':
      return { to: '/t/$slug/library/tests/$id', params }
    case 'database':
      return { to: '/t/$slug/library/databases/$id', params }
    case 'workload':
      return { to: '/t/$slug/library/workloads/$id', params }
    default:
      return undefined
  }
}

// Audit log of a tenant or of the whole platform (admin: + tenant column and filter). The table is
// the screen's main content: it fills the section and scrolls its rows (web/AGENTS.md §18).
export function AuditTable({
  rows,
  loading,
  error,
  onRetry,
  hasMore,
  fetchingMore,
  onLoadMore,
  search,
  onSearchChange,
  showTenant,
  tenantSlug,
  tenantOptions,
  tenantFilter,
  onTenantFilter,
}: {
  rows: AuditEntry[]
  loading: boolean
  error?: unknown
  onRetry?: () => void
  hasMore: boolean
  fetchingMore: boolean
  onLoadMore: () => void
  search: AuditSearch
  onSearchChange: (next: Partial<AuditSearch>) => void
  // Admin view: extra tenant column + filter.
  showTenant?: boolean
  tenantSlug?: string
  tenantOptions?: { label: string; value: string }[]
  tenantFilter?: string
  onTenantFilter?: (v: string | undefined) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const navigate = useNavigate()
  const copy = useCopy()
  const [details, setDetails] = useState<AuditEntry>()

  const rowActions = (e: AuditEntry): RowAction[] => {
    const link = targetLink(e, showTenant ? undefined : tenantSlug)
    return [
      {
        key: 'details',
        label: t('settings.audit.showDetails'),
        icon: 'brackets-curly',
        disabled: !hasDetails(e),
        disabledReason: t('settings.audit.noDetails'),
        onClick: () => setDetails(e),
      },
      {
        key: 'target',
        label: t('settings.audit.openTarget'),
        description: targetLabel(e) || undefined,
        icon: 'external-link-alt',
        disabled: !link,
        disabledReason: t('settings.audit.noTargetLink'),
        onClick: () => link && void navigate(link),
      },
      {
        key: 'byActor',
        group: true,
        label: t('settings.audit.filterActor'),
        icon: 'user',
        disabled: !e.actor.id,
        disabledReason: t('settings.audit.noActorId'),
        onClick: () => onSearchChange({ actor: e.actor.id }),
      },
      {
        key: 'byAction',
        label: t('settings.audit.filterAction'),
        icon: 'filter',
        onClick: () => onSearchChange({ action: e.action }),
      },
      {
        key: 'copyTarget',
        group: true,
        label: t('settings.audit.copyTargetId'),
        icon: 'copy',
        disabled: !e.target.id,
        disabledReason: t('settings.audit.noTargetId'),
        onClick: () => e.target.id && copy(e.target.id),
      },
      {
        key: 'copyRequest',
        label: t('settings.audit.copyRequestId'),
        icon: 'copy',
        disabled: !e.request_id,
        disabledReason: t('settings.audit.noRequestId'),
        onClick: () => e.request_id && copy(e.request_id),
      },
    ]
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure; search + tenant filter drive the recompute
  const columns = useMemo<DataTableColumn<AuditEntry>[]>(() => {
    const cols: DataTableColumn<AuditEntry>[] = [
      col.identity<AuditEntry>({
        id: 'action',
        header: t('settings.audit.columns.action'),
        minWidth: 240,
        filter: {
          kind: 'text',
          value: search.action,
          placeholder: t('settings.audit.actionPlaceholder'),
          onChange: (v) => onSearchChange({ action: v }),
        },
        render: (e) => ({
          title: e.action,
          subtitle: targetLabel(e),
        }),
      }),
      col.stack<AuditEntry>({
        id: 'actor',
        header: t('settings.audit.columns.actor'),
        width: 220,
        filter: {
          kind: 'text',
          value: search.actor,
          placeholder: t('settings.audit.actorPlaceholder'),
          onChange: (v) => onSearchChange({ actor: v }),
        },
        render: (e) => {
          const kind = t(`settings.audit.actorKind.${e.actor.kind}`)
          const name = e.actor.display_name ?? e.actor.id ?? kind
          return {
            primary: name,
            secondary: kind,
            title: [name, kind, e.actor.id].filter(Boolean).join('\n'),
          }
        },
      }),
    ]
    if (showTenant)
      cols.push(
        col.text<AuditEntry>({
          id: 'tenant',
          header: t('common.misc.tenant'),
          width: 180,
          value: (e) => e.tenant?.name ?? e.tenant?.id,
          filter: onTenantFilter
            ? {
                kind: 'checklist',
                single: true,
                options: (tenantOptions ?? []).map((o) => ({ value: o.value, label: o.label })),
                value: tenantFilter ? [tenantFilter] : undefined,
                onChange: (v) => onTenantFilter(v?.[0]),
              }
            : undefined,
        })
      )
    cols.push(
      col.text<AuditEntry>({
        id: 'details',
        header: t('settings.audit.columns.details'),
        minWidth: 220,
        value: (e) => (hasDetails(e) ? JSON.stringify(e.details) : undefined),
      }),
      col.text<AuditEntry>({
        id: 'request',
        header: t('settings.audit.requestId'),
        width: 180,
        defaultHidden: true,
        value: (e) => e.request_id,
      }),
      col.time<AuditEntry>({
        id: 'at',
        header: t('settings.audit.columns.time'),
        width: 150,
        filter: {
          kind: 'checklist',
          single: true,
          options: SINCE.map((v) => ({ value: v, label: t(`settings.audit.sinceOptions.${v}`) })),
          value: search.since ? [search.since] : undefined,
          onChange: (v) => onSearchChange({ since: v?.[0] as AuditSearch['since'] }),
        },
        value: (e) => e.at,
      }),
      col.actions<AuditEntry>({ title: (e) => e.action, actions: rowActions })
    )
    return cols
  }, [
    t,
    search,
    onSearchChange,
    showTenant,
    tenantSlug,
    tenantOptions,
    tenantFilter,
    onTenantFilter,
  ])
  const prefs = useTablePrefs(showTenant ? 'admin-audit' : 'audit', columns)

  const pills: ActivePill[] = [
    ...(search.action
      ? [
          {
            key: 'action',
            label: `${t('settings.audit.columns.action')}: ${search.action}`,
            onRemove: () => onSearchChange({ action: undefined }),
          },
        ]
      : []),
    ...(search.actor
      ? [
          {
            key: 'actor',
            label: `${t('settings.audit.columns.actor')}: ${search.actor}`,
            onRemove: () => onSearchChange({ actor: undefined }),
          },
        ]
      : []),
    ...(search.since
      ? [
          {
            key: 'since',
            label: `${t('settings.audit.since')}: ${t(`settings.audit.sinceOptions.${search.since}`)}`,
            onRemove: () => onSearchChange({ since: undefined }),
          },
        ]
      : []),
    ...(tenantFilter && onTenantFilter
      ? [
          {
            key: 'tenant',
            label: `${t('common.misc.tenant')}: ${tenantOptions?.find((o) => o.value === tenantFilter)?.label ?? tenantFilter}`,
            onRemove: () => onTenantFilter(undefined),
          },
        ]
      : []),
  ]
  const clearAll = () => {
    onSearchChange({ action: undefined, actor: undefined, since: undefined })
    onTenantFilter?.(undefined)
  }

  return (
    <PageFill>
      <DataTableToolbar
        pills={pills}
        onClearAll={pills.length ? clearAll : undefined}
        count={rows.length}
        settings={<TableSettings prefs={prefs} />}
      />
      <DataTable<AuditEntry>
        fill
        columns={prefs.visible}
        density={prefs.density}
        data={rows}
        getRowId={(e) => e.id}
        loading={loading}
        error={error}
        onRetry={onRetry}
        onRowClick={(e) => hasDetails(e) && setDetails(e)}
        filtered={pills.length > 0}
        onClearFilters={clearAll}
        empty={{ message: t('settings.audit.empty') }}
        footer={
          hasMore ? (
            <Stack justifyContent="space-between" alignItems="center">
              <span>{t('common.misc.showing', { count: rows.length })}</span>
              <Button size="sm" variant="secondary" onClick={onLoadMore} disabled={fetchingMore}>
                {t('common.actions.loadMore')}
              </Button>
            </Stack>
          ) : undefined
        }
      />
      <Modal
        isOpen={!!details}
        title={details ? `${details.action} · ${formatDateTime(details.at)}` : ''}
        onDismiss={() => setDetails(undefined)}
      >
        <pre className={styles.details}>{JSON.stringify(details?.details ?? {}, null, 2)}</pre>
      </Modal>
    </PageFill>
  )
}
