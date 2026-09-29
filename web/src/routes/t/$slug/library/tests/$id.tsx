import { testQueries } from '@api/queries/library'
import { TestDetailPage, testDetailSearchSchema } from '@components/library/tests/TestDetailPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/library/tests/$id')({
  validateSearch: testDetailSearchSchema,
  search: { middlewares: [stripSearchParams({ tab: 'overview' })] },
  loader: async ({ context, params }) => {
    const entity = await context.queryClient.ensureQueryData(
      testQueries.detail(params.slug, params.id)
    )
    return { crumb: entity.name }
  },
  component: () => {
    const { id } = Route.useParams()
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <TestDetailPage
        id={id}
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
