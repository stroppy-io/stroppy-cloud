import { databaseQueries } from '@api/queries/library'
import {
  DatabaseDetailPage,
  databaseDetailSearchSchema,
} from '@components/library/databases/DatabaseDetailPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/library/databases/$id')({
  validateSearch: databaseDetailSearchSchema,
  search: { middlewares: [stripSearchParams({ tab: 'overview' })] },
  loader: async ({ context, params }) => {
    const entity = await context.queryClient.ensureQueryData(
      databaseQueries.detail(params.slug, params.id)
    )
    return { crumb: entity.name }
  },
  component: () => {
    const { id } = Route.useParams()
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <DatabaseDetailPage
        id={id}
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
