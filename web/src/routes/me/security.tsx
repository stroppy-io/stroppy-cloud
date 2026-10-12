import { SecurityPage } from '@components/me/SecurityPage'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

// Kratos settings ui_url: password change; recovery hands over its `flow` here.
const search = z.object({ flow: z.string().optional().catch(undefined) })
export const Route = createFileRoute('/me/security')({
  validateSearch: search,
  staticData: { crumb: 'me.tabs.security' },
  component: () => {
    const { flow } = Route.useSearch()
    return <SecurityPage key={flow ?? 'new'} flowId={flow} />
  },
})
