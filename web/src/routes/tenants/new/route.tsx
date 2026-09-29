import { meQueries } from '@api/queries/me'
import { AppShell } from '@app/AppShell'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/tenants/new')({
  staticData: { crumb: 'nav.createTenant' },
  loader: ({ context }) => context.queryClient.ensureQueryData(meQueries.me()),
  component: AppShell,
})
