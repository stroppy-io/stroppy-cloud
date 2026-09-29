import { meQueries } from '@api/queries/me'
import { MyInvitesPage } from '@components/me/MyInvitesPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/me/invites')({
  staticData: { crumb: 'nav.invites' },
  loader: ({ context }) => context.queryClient.ensureQueryData(meQueries.invites()),
  component: MyInvitesPage,
})
