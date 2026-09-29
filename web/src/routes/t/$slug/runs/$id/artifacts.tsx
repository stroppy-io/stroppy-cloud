import { RunArtifactsTab } from '@components/runs/artifacts/RunArtifactsTab'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/runs/$id/artifacts')({
  staticData: { crumb: 'runs.tabs.artifacts' },
  component: () => {
    const { id } = Route.useParams()
    return <RunArtifactsTab id={id} />
  },
})
