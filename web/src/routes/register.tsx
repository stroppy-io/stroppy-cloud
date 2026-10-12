import { RegisterPage } from '@components/auth/RegisterPage'
import { loadPublicConfig } from '@lib/config'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

// No auth, no AppShell. The loader primes the Kratos base from the public
// config — the page is reachable directly (the root gate skips public
// prefixes without loading it).
const search = z.object({ next: z.string().optional().catch(undefined) })
export const Route = createFileRoute('/register')({
  validateSearch: search,
  staticData: { bare: true },
  loader: async () => {
    await loadPublicConfig()
  },
  component: () => {
    const { next } = Route.useSearch()
    return <RegisterPage next={next} />
  },
})
