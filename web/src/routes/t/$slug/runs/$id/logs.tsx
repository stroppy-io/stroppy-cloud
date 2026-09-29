import { RunLogsTab, runLogsSearchSchema } from '@components/runs/logs/RunLogsTab'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/runs/$id/logs')({
  staticData: { crumb: 'runs.tabs.logs' },
  validateSearch: runLogsSearchSchema,
  component: () => {
    const { id } = Route.useParams()
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <RunLogsTab
        id={id}
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
