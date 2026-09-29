import { settingsQueries } from '@api/queries/settings'
import { WebhooksPage } from '@components/settings/WebhooksPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/settings/webhooks')({
  staticData: { crumb: 'settings.tabs.webhooks' },
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(settingsQueries.webhooks(params.slug)),
  component: WebhooksPage,
})
