import { suiteQueries } from '@api/queries/suites'
import {
  SUITE_LIST_DEFAULTS,
  SuiteListPage,
  suiteListSearchSchema,
  toSuiteListQuery,
} from '@components/suites/SuiteListPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/suites/')({
  validateSearch: suiteListSearchSchema,
  search: { middlewares: [stripSearchParams(SUITE_LIST_DEFAULTS)] },
  loaderDeps: ({ search }) => ({ search }),
  loader: ({ context, params, deps }) =>
    context.queryClient.ensureInfiniteQueryData(
      suiteQueries.list(params.slug, toSuiteListQuery(deps.search))
    ),
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <SuiteListPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
