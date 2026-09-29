import { PageHeader, type TabDef } from '@app/PageHeader'
import { Badge } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useTranslation } from 'react-i18next'

// GitHub-style settings header with the tab strip; rendered by the `/t/$slug/settings` layout.
export function SettingsHeader() {
  const { t } = useTranslation()
  const { slug, tenant, role } = useTenant()
  const params = { slug }
  const tabs: TabDef[] = [
    {
      id: 'index',
      label: t('settings.tabs.general'),
      icon: 'cog',
      to: '/t/$slug/settings',
      params,
    },
    {
      id: 'members',
      label: t('settings.tabs.members'),
      icon: 'users-alt',
      to: '/t/$slug/settings/members',
      params,
    },
    {
      id: 'providers',
      label: t('settings.tabs.providers'),
      icon: 'cloud',
      to: '/t/$slug/settings/providers',
      params,
    },
    {
      id: 'quotas',
      label: t('settings.tabs.quotas'),
      icon: 'graph-bar',
      to: '/t/$slug/settings/quotas',
      params,
    },
    {
      id: 'webhooks',
      label: t('settings.tabs.webhooks'),
      icon: 'plug',
      to: '/t/$slug/settings/webhooks',
      params,
    },
    {
      id: 'tokens',
      label: t('settings.tabs.tokens'),
      icon: 'key-skeleton-alt',
      to: '/t/$slug/settings/tokens',
      params,
    },
    {
      id: 'audit',
      label: t('settings.tabs.audit'),
      icon: 'history',
      to: '/t/$slug/settings/audit',
      params,
    },
    {
      id: 'limits',
      label: t('settings.tabs.limits'),
      icon: 'sliders-v-alt',
      to: '/t/$slug/settings/limits',
      params,
    },
  ]
  return (
    <PageHeader
      title={t('settings.title')}
      subtitle={tenant ? `${tenant.name} · ${slug}` : slug}
      icon="cog"
      badge={role ? <Badge text={t(`common.role.${role}`)} color="blue" icon="user" /> : undefined}
      breadcrumbs={[{ label: tenant?.name ?? slug, to: '/t/$slug', params }]}
      tabs={tabs}
    />
  )
}
