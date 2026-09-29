import { runQueries } from '@api/queries/runs'
import { RunOverviewTab } from '@components/runs/overview/RunOverviewTab'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/runs/$id/')({
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(runQueries.overview(params.slug, params.id)),
  component: () => {
    const { id } = Route.useParams()
    return <RunOverviewTab id={id} />
  },
})
