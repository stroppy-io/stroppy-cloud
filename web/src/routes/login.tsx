import { LoginPage } from '@components/auth/LoginPage'
import { loadPublicConfig } from '@lib/config'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

// No auth, no AppShell. The mode decides: the Kratos form or the dev token card.
const search = z.object({ next: z.string().optional().catch(undefined) })
export const Route = createFileRoute('/login')({
  validateSearch: search,
  staticData: { bare: true },
  loader: async () => ({ mode: (await loadPublicConfig()).auth.mode }),
  component: () => {
    const { mode } = Route.useLoaderData()
    const { next } = Route.useSearch()
    return <LoginPage mode={mode} next={next} />
  },
})
