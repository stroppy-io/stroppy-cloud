import { runQueries } from '@api/queries/runs'
import {
  RUN_LIST_DEFAULTS,
  RunListPage,
  runListSearchSchema,
  toRunListQuery,
} from '@components/runs/RunListPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/runs/')({
  validateSearch: runListSearchSchema,
  search: { middlewares: [stripSearchParams(RUN_LIST_DEFAULTS)] },
  loaderDeps: ({ search }) => ({ search }),
  loader: ({ context, params, deps }) =>
    context.queryClient.ensureInfiniteQueryData(
      runQueries.list(params.slug, toRunListQuery(deps.search))
    ),
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <RunListPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
