import { meQueries } from '@api/queries/me'
import { MyTokensPage } from '@components/me/MyTokensPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/me/tokens')({
  staticData: { crumb: 'nav.tokens' },
  loader: ({ context }) => context.queryClient.ensureQueryData(meQueries.tokens()),
  component: MyTokensPage,
})
