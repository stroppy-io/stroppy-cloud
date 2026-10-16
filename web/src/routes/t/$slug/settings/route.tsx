import { tenantQueries } from '@api/queries/tenants'
import { Page, PageFill } from '@app/Page'
import { SettingsHeader } from '@components/settings/SettingsHeader'
import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/settings')({
  staticData: { crumb: 'nav.settings' },
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(tenantQueries.detail(params.slug)),
  component: () => (
    <Page fill>
      <SettingsHeader />
      <PageFill scroll>
        <Outlet />
      </PageFill>
    </Page>
  ),
})
