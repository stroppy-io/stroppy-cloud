import { shareQueries } from '@api/queries/results'
import {
  SHARES_DEFAULTS,
  SharesPage,
  sharesSearchSchema,
  toShareQuery,
} from '@components/results/SharesPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/shares')({
  staticData: { crumb: 'nav.shares' },
  validateSearch: sharesSearchSchema,
  search: { middlewares: [stripSearchParams(SHARES_DEFAULTS)] },
  loaderDeps: ({ search }) => ({ search }),
  loader: ({ context, params, deps }) =>
    context.queryClient.ensureInfiniteQueryData(
      shareQueries.list(params.slug, toShareQuery(deps.search))
    ),
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <SharesPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
