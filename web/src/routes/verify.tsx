import { VerifyPage } from '@components/auth/VerifyPage'
import { loadPublicConfig } from '@lib/config'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

// No auth, no AppShell: the mail link opens it in any browser. `flow` comes
// from registration (continue_with) or from Kratos's redirect of the link.
const search = z.object({ flow: z.string().optional().catch(undefined) })
export const Route = createFileRoute('/verify')({
  validateSearch: search,
  staticData: { bare: true },
  loader: async () => {
    await loadPublicConfig()
  },
  component: () => {
    const { flow } = Route.useSearch()
    return <VerifyPage key={flow ?? 'new'} flowId={flow} />
  },
})
