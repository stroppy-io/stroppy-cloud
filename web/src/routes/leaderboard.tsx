import { LeaderboardPage } from '@components/leaderboard/LeaderboardPage'
import { RATING_FILTER_DEFAULTS, ratingFiltersSchema } from '@components/results/RatingBoard'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

// The public rating (no sign-in); filters live in the URL.
export const Route = createFileRoute('/leaderboard')({
  staticData: { bare: true },
  validateSearch: ratingFiltersSchema,
  search: { middlewares: [stripSearchParams(RATING_FILTER_DEFAULTS)] },
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <LeaderboardPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
