import { adminQueries } from '@api/queries/admin'
import { PageHeader, type TabDef } from '@app/PageHeader'
import { Badge } from '@grafana/ui'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

export function AdminHeader() {
  const { t } = useTranslation()
  const status = useQuery(adminQueries.status()).data
  const degraded = Object.values(status?.components ?? {}).filter((c) => c.status !== 'ok').length
  const behind = status?.pipelines.namespaces.filter((n) => n.status !== 'synced').length ?? 0
  const running = (status?.runs.running ?? 0) + (status?.runs.pending ?? 0)
  const tabs: TabDef[] = [
    {
      id: 'index',
      label: t('admin.tabs.status'),
      icon: 'signal',
      to: '/admin',
      counter: degraded + behind || undefined,
    },
    { id: 'tenants', label: t('admin.tabs.tenants'), icon: 'building', to: '/admin/tenants' },
    { id: 'users', label: t('admin.tabs.users'), icon: 'users-alt', to: '/admin/users' },
    {
      id: 'runs',
      label: t('admin.tabs.runs'),
      icon: 'play',
      to: '/admin/runs',
      counter: running || undefined,
    },
    { id: 'settings', label: t('admin.tabs.settings'), icon: 'cog', to: '/admin/settings' },
    { id: 'audit', label: t('admin.tabs.audit'), icon: 'history', to: '/admin/audit' },
  ]
  return (
    <PageHeader
      title={t('admin.title')}
      subtitle={status ? `v${status.version} · ${status.commit}` : undefined}
      icon="shield"
      badge={
        degraded > 0 ? (
          <Badge
            text={t('admin.status.degradedCount', { count: degraded })}
            color="orange"
            icon="exclamation-triangle"
          />
        ) : (
          <Badge text={t('common.status.ok')} color="green" icon="check" />
        )
      }
      tabs={tabs}
    />
  )
}
