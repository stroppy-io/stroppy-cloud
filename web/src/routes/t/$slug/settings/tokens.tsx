import { tenantQueries } from '@api/queries/tenants'
import { TokensPage } from '@components/settings/TokensPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/settings/tokens')({
  staticData: { crumb: 'settings.tabs.tokens' },
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(tenantQueries.tokens(params.slug)),
  component: TokensPage,
})
