import { databaseQueries } from '@api/queries/library'
import {
  DATABASE_LIST_DEFAULTS,
  DatabaseListPage,
  databaseListSearchSchema,
  toDatabaseListQuery,
} from '@components/library/databases/DatabaseListPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/library/databases/')({
  validateSearch: databaseListSearchSchema,
  search: { middlewares: [stripSearchParams(DATABASE_LIST_DEFAULTS)] },
  loaderDeps: ({ search }) => ({ search }),
  loader: ({ context, params, deps }) =>
    context.queryClient.ensureInfiniteQueryData(
      databaseQueries.list(params.slug, toDatabaseListQuery(deps.search))
    ),
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <DatabaseListPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
