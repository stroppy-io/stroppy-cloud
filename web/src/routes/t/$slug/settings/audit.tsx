import { auditSearchSchema } from '@components/audit/AuditTable'
import { AuditPage } from '@components/settings/AuditPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/settings/audit')({
  staticData: { crumb: 'settings.tabs.audit' },
  validateSearch: auditSearchSchema,
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <AuditPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})
