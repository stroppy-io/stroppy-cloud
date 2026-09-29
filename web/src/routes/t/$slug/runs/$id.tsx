import { runQueries } from '@api/queries/runs'
import { RunDetailLayout } from '@components/runs/detail/RunDetailLayout'
import { createFileRoute, Outlet } from '@tanstack/react-router'

// Layout route: header + tabs; child routes render the tab content in <Outlet/>.
export const Route = createFileRoute('/t/$slug/runs/$id')({
  loader: async ({ context, params }) => {
    const entity = await context.queryClient.ensureQueryData(
      runQueries.detail(params.slug, params.id)
    )
    return { crumb: entity.name }
  },
  component: () => {
    const { id } = Route.useParams()
    return (
      <RunDetailLayout id={id}>
        <Outlet />
      </RunDetailLayout>
    )
  },
})
