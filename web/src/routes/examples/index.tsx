import { exampleQueries } from '@api/queries/results'
import { ExamplesPage, examplesSearchSchema } from '@components/examples/ExamplesPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/examples/')({
  staticData: { crumb: 'nav.examples' },
  validateSearch: examplesSearchSchema,
  loaderDeps: ({ search }) => ({ kind: search.kind, db: search.db }),
  loader: ({ context, deps }) =>
    context.queryClient.ensureQueryData(
      exampleQueries.list({ kind: deps.kind, db_kind: deps.db as never })
    ),
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <ExamplesPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
