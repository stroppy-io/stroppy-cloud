import { PublicReportPage } from '@components/public/PublicReportPage'
import { createFileRoute } from '@tanstack/react-router'

// Public lab-report: no auth, no AppShell. Errors (revoked/expired) render inside the page.
export const Route = createFileRoute('/s/$token')({
  staticData: { crumb: 'public.title', bare: true },
  component: () => {
    const { token } = Route.useParams()
    return <PublicReportPage token={token} />
  },
})
