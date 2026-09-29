import { settingsQueries } from '@api/queries/settings'
import { QuotasPage } from '@components/settings/QuotasPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/settings/quotas')({
  staticData: { crumb: 'settings.tabs.quotas' },
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(settingsQueries.providers(params.slug)),
  component: QuotasPage,
})
