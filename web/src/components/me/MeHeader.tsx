import { meQueries } from '@api/queries/me'
import { PageHeader, type TabDef } from '@app/PageHeader'
import { Badge } from '@grafana/ui'
import { useMe } from '@hooks/useMe'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

// Header + tab strip for `/me/*`, rendered by the `/me` layout route.
export function MeHeader() {
  const { t } = useTranslation()
  const me = useMe()
  const invites = useQuery(meQueries.invites()).data?.data.filter((i) => i.status === 'pending')
  const tabs: TabDef[] = [
    { id: 'profile', label: t('me.tabs.profile'), icon: 'user', to: '/me/profile' },
    { id: 'tokens', label: t('me.tabs.tokens'), icon: 'key-skeleton-alt', to: '/me/tokens' },
    {
      id: 'invites',
      label: t('me.tabs.invites'),
      icon: 'envelope',
      to: '/me/invites',
      counter: invites?.length || undefined,
    },
  ]
  return (
    <PageHeader
      title={me.display_name}
      subtitle={me.email}
      icon="user"
      badge={
        me.is_platform_admin ? (
          <Badge text={t('me.platformAdmin')} color="purple" icon="shield" />
        ) : undefined
      }
      tabs={tabs}
    />
  )
}
