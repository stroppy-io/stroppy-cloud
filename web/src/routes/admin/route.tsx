import { adminQueries } from '@api/queries/admin'
import { meQueries } from '@api/queries/me'
import { AppShell } from '@app/AppShell'
import { Page, PageFill } from '@app/Page'
import { AdminHeader } from '@components/admin/AdminHeader'
import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'

export const Route = createFileRoute('/admin')({
  staticData: { crumb: 'nav.admin' },
  beforeLoad: async ({ context }) => {
    const me = await context.queryClient.ensureQueryData(meQueries.me())
    if (!me.is_platform_admin) throw redirect({ to: '/' })
  },
  loader: ({ context }) => context.queryClient.ensureQueryData(adminQueries.status()),
  component: () => (
    <AppShell>
      <Page width="wide" fill>
        <AdminHeader />
        <PageFill scroll>
          <Outlet />
        </PageFill>
      </Page>
    </AppShell>
  ),
})
