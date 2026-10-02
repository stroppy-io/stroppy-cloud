import { RegisterPage } from '@components/auth/RegisterPage'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

// No auth, no AppShell.
const search = z.object({ next: z.string().optional().catch(undefined) })
export const Route = createFileRoute('/register')({
  validateSearch: search,
  staticData: { bare: true },
  component: () => {
    const { next } = Route.useSearch()
    return <RegisterPage next={next} />
  },
})
