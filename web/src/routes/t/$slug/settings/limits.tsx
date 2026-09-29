import { settingsQueries } from '@api/queries/settings'
import { LimitsPage } from '@components/settings/LimitsPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/settings/limits')({
  staticData: { crumb: 'settings.tabs.limits' },
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(settingsQueries.limits(params.slug)),
  component: LimitsPage,
})
