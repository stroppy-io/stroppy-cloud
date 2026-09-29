import { RunSpecTab } from '@components/runs/spec/RunSpecTab'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/runs/$id/spec')({
  staticData: { crumb: 'runs.tabs.spec' },
  component: () => {
    const { id } = Route.useParams()
    return <RunSpecTab id={id} />
  },
})
