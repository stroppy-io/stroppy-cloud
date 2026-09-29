import { adminQueries } from '@api/queries/admin'
import { AdminStatusPage } from '@components/admin/AdminStatusPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/admin/')({
  staticData: { crumb: 'admin.tabs.status' },
  loader: ({ context }) => context.queryClient.ensureQueryData(adminQueries.status()),
  component: AdminStatusPage,
})
