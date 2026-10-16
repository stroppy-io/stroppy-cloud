import { explorerSearchSchema } from '@components/metrics/explorer-search'
import { PublicReportPage, reportTabs } from '@components/public/PublicReportPage'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

// Public lab-report: no auth, no AppShell. Errors (revoked/expired) render inside the page.
// The search holds the open tab and the metric explorer's view: a link opens the same charts.
export const Route = createFileRoute('/s/$token')({
  staticData: { crumb: 'public.title', bare: true },
  validateSearch: explorerSearchSchema.extend({
    tab: z.enum(reportTabs).optional().catch(undefined),
  }),
  component: () => {
    const { token } = Route.useParams()
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <PublicReportPage
        token={token}
        search={search}
        onSearchChange={(next) =>
          void navigate({
            search: (prev) => ({ ...prev, ...next }),
            replace: true,
            hashScrollIntoView: false,
          })
        }
      />
    )
  },
})
