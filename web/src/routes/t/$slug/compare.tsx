import { ComparePage, compareSearchSchema } from '@components/runs/compare/ComparePage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/compare')({
  staticData: { crumb: 'nav.compare' },
  validateSearch: compareSearchSchema,
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <ComparePage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
