import { EmbedLeaderboard, embedSearchSchema } from '@components/leaderboard/EmbedLeaderboard'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

// The rating widget for other sites' <iframe> (public, no chrome); with
// filters=1 the whole board, its filters in the frame's URL like /leaderboard.
export const Route = createFileRoute('/embed/leaderboard')({
  staticData: { bare: true },
  validateSearch: embedSearchSchema,
  search: {
    middlewares: [stripSearchParams({ theme: 'dark', metric: 'tps', period: 'all', limit: 10 })],
  },
  component: () => {
    const navigate = Route.useNavigate()
    return (
      <EmbedLeaderboard
        search={Route.useSearch()}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
