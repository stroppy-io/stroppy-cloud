import { ratingQueries } from '@api/queries/results'
import { RatingPage, ratingSearchSchema, toRatingQuery } from '@components/results/RatingPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/rating')({
  staticData: { crumb: 'nav.rating' },
  validateSearch: ratingSearchSchema,
  search: { middlewares: [stripSearchParams({ tab: 'tenant', metric: 'tps', period: 'all' })] },
  loaderDeps: ({ search }) => ({ search }),
  loader: ({ context, params, deps }) =>
    deps.search.tab === 'global'
      ? undefined
      : context.queryClient.ensureInfiniteQueryData(
          ratingQueries.tenant(params.slug, toRatingQuery(deps.search))
        ),
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
