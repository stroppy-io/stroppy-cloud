import { dashboardQueries } from '@api/queries/results'
import { DashboardPage } from '@components/dashboard/DashboardPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/')({
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(dashboardQueries.get(params.slug)),
  component: DashboardPage,
})
