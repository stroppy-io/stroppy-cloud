import { ratingQueries } from '@api/queries/results'
import {
  boardHardware,
  RatingPage,
  ratingCensusQuery,
  ratingSearchSchema,
  toRatingQuery,
} from '@components/results/RatingPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/rating')({
  staticData: { crumb: 'nav.rating' },
  validateSearch: ratingSearchSchema,
  search: { middlewares: [stripSearchParams({ tab: 'tenant', metric: 'tps', period: 'all' })] },
  loaderDeps: ({ search }) => ({ search }),
  loader: async ({ context, params, deps }) => {
    if (deps.search.tab === 'global') return
    // The board's default hardware class is counted on the census first.
    const census = await context.queryClient.ensureInfiniteQueryData(
      ratingQueries.tenant(params.slug, ratingCensusQuery(deps.search))
    )
    const hw = boardHardware(deps.search.hardware, census.pages[0]?.data)
    await context.queryClient.ensureInfiniteQueryData(
      ratingQueries.tenant(params.slug, toRatingQuery(deps.search, hw.key))
    )
  },
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <RatingPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
