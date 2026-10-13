import { meQueries } from '@api/queries/me'
import { AppShell } from '@app/AppShell'
import { PublicShell } from '@components/public/PublicShell'
import { createFileRoute, Outlet } from '@tanstack/react-router'

// A first tenant is created before there is a console to show, in the public
// frame; further ones — from inside the console.
export const Route = createFileRoute('/tenants/new')({
  staticData: { crumb: 'nav.createTenant' },
  loader: ({ context }) => context.queryClient.ensureQueryData(meQueries.me()),
  component: () => {
    const me = Route.useLoaderData()
    return me.tenants.length > 0 ? (
      <AppShell />
    ) : (
      <PublicShell>
        <Outlet />
      </PublicShell>
    )
  },
})
