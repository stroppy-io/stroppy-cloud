import { adminMutations, adminQueries } from '@api/queries/admin'
import type { AdminStatus } from '@api/types'
import { SectionTitle } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { KeyValueList } from '@components/KeyValueList'
import { RelativeTime } from '@components/RelativeTime'
import { StatusBadge } from '@components/StatusBadge'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Badge, Button, Stack, Text, Tooltip, useStyles2 } from '@grafana/ui'
import { formatDuration } from '@helpers/format'
import { formatDateTime } from '@helpers/time'
import { useCopy } from '@hooks/useCopy'
import { useMutation, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

type Namespace = AdminStatus['pipelines']['namespaces'][number]

const getStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
    gap: theme.spacing(2),
  }),
  panel: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(2),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1.5),
  }),
  chips: css({ display: 'flex', gap: theme.spacing(1), flexWrap: 'wrap' }),
  chip: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(0.75, 1.5),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
  }),
  counter: css({
    fontSize: theme.typography.h2.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: 1,
  }),
  counters: css({ display: 'flex', gap: theme.spacing(3), flexWrap: 'wrap' }),
  mono: css({ fontFamily: theme.typography.fontFamilyMonospace }),
})

export function AdminStatusPage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const qc = useQueryClient()
  const navigate = useNavigate()
  const copy = useCopy()
  const status = useSuspenseQuery(adminQueries.status()).data
  const resync = useMutation({
    mutationFn: (slug?: string) => adminMutations.resync(slug),
    onSuccess: async (r) => {
      await qc.invalidateQueries({ queryKey: adminQueries.status().queryKey })
      toast.success(t('admin.status.resynced', { count: r.namespaces?.length ?? 0 }))
    },
    onError: (e) => toast.error(e),
  })
  const uptime = status.started_at
    ? formatDuration((Date.now() - new Date(status.started_at).getTime()) / 1000)
    : '—'
  const behind = status.pipelines.namespaces.filter((n) => n.status !== 'synced')

  const nsActions = (n: Namespace): RowAction[] => [
    {
      key: 'resync',
      label: t('admin.status.resync'),
      icon: 'sync',
      disabled: !n.tenant_slug || resync.isPending,
      disabledReason: t('admin.status.noTenant'),
      onClick: () => resync.mutate(n.tenant_slug),
    },
    {
      key: 'tenant',
      label: t('admin.status.findTenant'),
      icon: 'building',
      disabled: !n.tenant_slug,
      disabledReason: t('admin.status.noTenant'),
      onClick: () =>
        void navigate({ to: '/admin/tenants', search: { search: n.tenant_slug } as never }),
    },
    {
      key: 'copy',
      group: true,
      label: t('admin.status.copyNamespace'),
      icon: 'copy',
      onClick: () => copy(n.namespace),
    },
  ]

  // biome-ignore lint/correctness/useExhaustiveDependencies: nsActions is a per-render closure over resync state
  const columns = useMemo<DataTableColumn<Namespace>[]>(
    () => [
      col.status<Namespace>({
        id: 'status',
        title: t('common.fields.status'),
        status: (n) => n.status,
      }),
      col.identity<Namespace>({
        id: 'ns',
        header: t('admin.status.namespace'),
        render: (n) => ({
          title: n.namespace,
          subtitle: n.tenant_slug,
          link: n.tenant_slug
            ? { to: '/admin/tenants', search: { search: n.tenant_slug } }
            : undefined,
        }),
      }),
      col.stack<Namespace>({
        id: 'rev',
        header: t('admin.status.revision'),
        width: 200,
        value: (n) => n.revision,
        render: (n) => {
          const ok = !!n.revision && n.revision === status.pipelines.expected_revision
          const secondary = n.revision
            ? ok
              ? t('admin.status.matches')
              : t('admin.status.expected', { revision: status.pipelines.expected_revision })
            : undefined
          return {
            primary: n.revision ? (
              <span className={styles.mono}>
                <Text color={ok ? 'primary' : 'warning'}>{n.revision}</Text>
              </span>
            ) : undefined,
            secondary,
            title: [n.revision, secondary].filter(Boolean).join('\n'),
          }
        },
      }),
      col.time<Namespace>({
        id: 'pushed',
        header: t('admin.status.pushedAt'),
        value: (n) => n.pushed_at,
      }),
      col.text<Namespace>({
        id: 'error',
        header: t('admin.status.error'),
        value: (n) => n.error,
      }),
      col.actions<Namespace>({ title: (n) => n.namespace, actions: nsActions }),
    ],
    [t, styles, status.pipelines.expected_revision, resync.isPending]
  )

  return (
    <Stack direction="column" gap={3}>
      <div className={styles.grid}>
        <section className={styles.panel}>
          <KeyValueList
            title={t('admin.status.server')}
            items={[
              { label: t('common.fields.version'), value: status.version },
              {
                label: t('admin.status.commit'),
                value: <span className={styles.mono}>{status.commit}</span>,
              },
              {
                label: t('admin.status.started'),
                value: status.started_at ? (
                  <span title={formatDateTime(status.started_at)}>
                    <RelativeTime value={status.started_at} /> ·{' '}
                    {t('admin.status.uptime', { uptime })}
                  </span>
                ) : (
                  '—'
                ),
              },
            ]}
          />
        </section>
        <section className={styles.panel}>
          <Text element="h3" variant="h5">
            {t('admin.status.components')}
          </Text>
          <div className={styles.chips}>
            {Object.entries(status.components).map(([name, c]) => (
              <Tooltip key={name} content={c.detail ?? c.status}>
                <span className={styles.chip}>
                  <StatusBadge status={c.status} iconOnly />
                  <Text weight="medium">{name}</Text>
                  {c.version && (
                    <Text color="secondary" variant="bodySmall">
                      {c.version}
                    </Text>
                  )}
                </span>
              </Tooltip>
            ))}
          </div>
          {Object.values(status.components).some((c) => c.status !== 'ok') && (
            <Text color="secondary" variant="bodySmall">
              {Object.entries(status.components)
                .filter(([, c]) => c.status !== 'ok')
                .map(([n, c]) => `${n}: ${c.detail ?? c.status}`)
                .join(' · ')}
            </Text>
          )}
        </section>
        <section className={styles.panel}>
          <Text element="h3" variant="h5">
            {t('admin.status.counters')}
          </Text>
          <div className={styles.counters}>
            <Counter label={t('common.status.running')} value={status.runs.running ?? 0} />
            <Counter label={t('common.status.pending')} value={status.runs.pending ?? 0} />
            <Counter label={t('admin.status.keptStands')} value={status.runs.kept_stands ?? 0} />
            <Counter label={t('admin.status.tenantsActive')} value={status.tenants?.active ?? 0} />
            <Counter label={t('common.status.suspended')} value={status.tenants?.suspended ?? 0} />
            <Counter label={t('common.status.orphaned')} value={status.tenants?.orphaned ?? 0} />
          </div>
        </section>
      </div>

      <Stack direction="column" gap={1}>
        <SectionTitle
          right={
            <Stack gap={1} alignItems="center">
              <Text color="secondary" variant="bodySmall">
                {t('admin.status.expectedRevision')}{' '}
                <span className={styles.mono}>{status.pipelines.expected_revision}</span>
              </Text>
              {behind.length > 0 && (
                <Badge
                  text={t('admin.status.behindCount', { count: behind.length })}
                  color="orange"
                />
              )}
              <Button
                size="sm"
                icon="sync"
                variant="secondary"
                disabled={resync.isPending}
                onClick={() => resync.mutate(undefined)}
              >
                {t('admin.status.resyncAll')}
              </Button>
            </Stack>
          }
        >
          {t('admin.status.pipelines')}
        </SectionTitle>
        <DataTable<Namespace>
          columns={columns}
          data={status.pipelines.namespaces}
          getRowId={(n) => n.namespace}
          clientSort
          empty={{ message: t('admin.status.noNamespaces') }}
        />
      </Stack>
    </Stack>
  )
}

function Counter({ label, value }: { label: string; value: number }) {
  const styles = useStyles2(getStyles)
  return (
    <div>
      <div className={styles.counter}>{value}</div>
      <Text color="secondary" variant="bodySmall">
        {label}
      </Text>
    </div>
  )
}
