import { meQueries } from '@api/queries/me'
import { AppShell } from '@app/AppShell'
import { Page, PageFill } from '@app/Page'
import { MeHeader } from '@components/me/MeHeader'
import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/me')({
  staticData: { crumb: 'nav.account' },
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.ensureQueryData(meQueries.me()),
      context.queryClient.ensureQueryData(meQueries.invites()),
    ]),
  component: () => (
    <AppShell>
      <Page fill>
        <MeHeader />
        <PageFill scroll>
          <Outlet />
        </PageFill>
      </Page>
    </AppShell>
  ),
})
