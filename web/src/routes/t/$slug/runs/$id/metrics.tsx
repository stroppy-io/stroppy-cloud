import { catalogQueries } from '@api/queries/catalog'
import { RunMetricsTab, runMetricsSearchSchema } from '@components/runs/metrics/RunMetricsTab'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/runs/$id/metrics')({
  staticData: { crumb: 'runs.tabs.metrics' },
  validateSearch: runMetricsSearchSchema,
  search: { middlewares: [stripSearchParams({ layout: 'grid' })] },
  loader: ({ context }) => context.queryClient.ensureQueryData(catalogQueries.metrics()),
  component: () => {
    const { id } = Route.useParams()
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <RunMetricsTab
        id={id}
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
