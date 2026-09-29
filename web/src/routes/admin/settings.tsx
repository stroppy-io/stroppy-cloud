import { adminQueries } from '@api/queries/admin'
import { AdminSettingsPage } from '@components/admin/AdminSettingsPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/admin/settings')({
  staticData: { crumb: 'admin.tabs.settings' },
  loader: ({ context }) => context.queryClient.ensureQueryData(adminQueries.settings()),
  component: AdminSettingsPage,
})
