import { runQueries } from '@api/queries/runs'
import { RunEventsTab, runEventsSearchSchema } from '@components/runs/events/RunEventsTab'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/runs/$id/events')({
  staticData: { crumb: 'runs.tabs.events' },
  validateSearch: runEventsSearchSchema,
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(runQueries.events(params.slug, params.id)),
  component: () => {
    const { id } = Route.useParams()
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <RunEventsTab
        id={id}
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
