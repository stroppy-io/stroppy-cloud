import { workloadQueries } from '@api/queries/library'
import {
  WorkloadDetailPage,
  workloadDetailSearchSchema,
} from '@components/library/workloads/WorkloadDetailPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/library/workloads/$id')({
  validateSearch: workloadDetailSearchSchema,
  search: { middlewares: [stripSearchParams({ tab: 'overview' })] },
  loader: async ({ context, params }) => {
    const entity = await context.queryClient.ensureQueryData(
      workloadQueries.detail(params.slug, params.id)
    )
    return { crumb: entity.name }
  },
  component: () => {
    const { id } = Route.useParams()
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <WorkloadDetailPage
        id={id}
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
