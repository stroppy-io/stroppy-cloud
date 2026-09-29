import { AdminAuditPage, adminAuditSearchSchema } from '@components/admin/AdminAuditPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/admin/audit')({
  staticData: { crumb: 'admin.tabs.audit' },
  validateSearch: adminAuditSearchSchema,
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <AdminAuditPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
