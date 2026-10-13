import { meQueries } from '@api/queries/me'
import { WelcomePage } from '@components/onboarding/WelcomePage'
import { loadPublicConfig } from '@lib/config'
import { createFileRoute } from '@tanstack/react-router'

// The first screen after the first sign-in (no shell): what to do next and how the product is built.
export const Route = createFileRoute('/welcome')({
  staticData: { bare: true },
  loader: async ({ context }) => {
    await Promise.all([loadPublicConfig(), context.queryClient.ensureQueryData(meQueries.me())])
  },
  component: WelcomePage,
})
