import {
  ADMIN_USERS_DEFAULTS,
  AdminUsersPage,
  adminUsersSearchSchema,
} from '@components/admin/AdminUsersPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/admin/users')({
  staticData: { crumb: 'admin.tabs.users' },
  validateSearch: adminUsersSearchSchema,
  search: { middlewares: [stripSearchParams(ADMIN_USERS_DEFAULTS)] },
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <AdminUsersPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
