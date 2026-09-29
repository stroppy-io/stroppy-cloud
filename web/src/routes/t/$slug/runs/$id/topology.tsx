import { runQueries } from '@api/queries/runs'
import { TopologyView } from '@components/runs/topology/TopologyView'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/runs/$id/topology')({
  staticData: { crumb: 'runs.tabs.topology' },
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(runQueries.overview(params.slug, params.id)),
  component: () => {
    const { id } = Route.useParams()
    return <TopologyView id={id} />
  },
})
