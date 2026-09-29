import { settingsQueries } from '@api/queries/settings'
import { ProvidersPage } from '@components/settings/ProvidersPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/settings/providers')({
  staticData: { crumb: 'settings.tabs.providers' },
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(settingsQueries.providers(params.slug)),
  component: ProvidersPage,
})
