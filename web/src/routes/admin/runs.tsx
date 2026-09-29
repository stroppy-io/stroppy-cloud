import {
  ADMIN_RUNS_DEFAULTS,
  AdminRunsPage,
  adminRunsSearchSchema,
} from '@components/admin/AdminRunsPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/admin/runs')({
  staticData: { crumb: 'admin.tabs.runs' },
  validateSearch: adminRunsSearchSchema,
  search: { middlewares: [stripSearchParams(ADMIN_RUNS_DEFAULTS)] },
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <AdminRunsPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
