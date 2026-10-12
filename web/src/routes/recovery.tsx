import { RecoveryPage } from '@components/auth/RecoveryPage'
import { loadPublicConfig } from '@lib/config'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

// No auth, no AppShell: password reset for someone who cannot sign in.
const search = z.object({ flow: z.string().optional().catch(undefined) })
export const Route = createFileRoute('/recovery')({
  validateSearch: search,
  staticData: { bare: true },
  loader: async () => {
    await loadPublicConfig()
  },
  component: () => {
    const { flow } = Route.useSearch()
    return <RecoveryPage key={flow ?? 'new'} flowId={flow} />
  },
})
