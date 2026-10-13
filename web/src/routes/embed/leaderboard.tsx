import { EmbedLeaderboard, embedSearchSchema } from '@components/leaderboard/EmbedLeaderboard'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

// The rating widget for other sites' <iframe> (public, no chrome).
export const Route = createFileRoute('/embed/leaderboard')({
  staticData: { bare: true },
  validateSearch: embedSearchSchema,
  search: {
    middlewares: [stripSearchParams({ theme: 'dark', metric: 'tps', period: 'all', limit: 10 })],
  },
  component: () => <EmbedLeaderboard search={Route.useSearch()} />,
})
