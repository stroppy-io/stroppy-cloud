import { testQueries } from '@api/queries/library'
import {
  TEST_LIST_DEFAULTS,
  TestListPage,
  testListSearchSchema,
  toTestListQuery,
} from '@components/library/tests/TestListPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/library/tests/')({
  staticData: { crumb: 'nav.tests' },
  validateSearch: testListSearchSchema,
  search: { middlewares: [stripSearchParams(TEST_LIST_DEFAULTS)] },
  loaderDeps: ({ search }) => ({ search }),
  loader: ({ context, params, deps }) =>
    context.queryClient.ensureInfiniteQueryData(
      testQueries.list(params.slug, toTestListQuery(deps.search))
    ),
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <TestListPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
