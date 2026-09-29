import { settingsQueries } from '@api/queries/settings'
import { tenantQueries } from '@api/queries/tenants'
import { GeneralSettingsPage } from '@components/settings/GeneralSettingsPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/settings/')({
  staticData: { crumb: 'settings.tabs.general' },
  loader: ({ context, params }) =>
    Promise.all([
      context.queryClient.ensureQueryData(settingsQueries.settings(params.slug)),
      context.queryClient.ensureQueryData(tenantQueries.members(params.slug)),
    ]),
  component: GeneralSettingsPage,
})
