import { tenantQueries } from '@api/queries/tenants'
import { MembersPage } from '@components/settings/MembersPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/settings/members')({
  staticData: { crumb: 'settings.tabs.members' },
  loader: ({ context, params }) =>
    Promise.all([
      context.queryClient.ensureQueryData(tenantQueries.members(params.slug)),
      context.queryClient.ensureQueryData(tenantQueries.invites(params.slug)),
    ]),
  component: MembersPage,
})
