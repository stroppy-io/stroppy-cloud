import {
  ADMIN_TENANTS_DEFAULTS,
  AdminTenantsPage,
  adminTenantsSearchSchema,
} from '@components/admin/AdminTenantsPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/admin/tenants')({
  staticData: { crumb: 'admin.tabs.tenants' },
  validateSearch: adminTenantsSearchSchema,
  search: { middlewares: [stripSearchParams(ADMIN_TENANTS_DEFAULTS)] },
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <AdminTenantsPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
