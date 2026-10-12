import { AuthErrorPage } from '@components/auth/AuthErrorPage'
import { loadPublicConfig } from '@lib/config'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

// Kratos's error ui_url: a self-service flow failed in a way the flow itself cannot show.
const search = z.object({ id: z.string().optional().catch(undefined) })
export const Route = createFileRoute('/auth/error')({
  validateSearch: search,
  staticData: { bare: true },
  loader: async () => {
    await loadPublicConfig()
  },
  component: () => {
    const { id } = Route.useSearch()
    return <AuthErrorPage id={id} />
  },
})
