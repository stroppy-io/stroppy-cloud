import { LandingPage } from '@components/landing/LandingPage'
import { createFileRoute } from '@tanstack/react-router'

// The public front page, for guests and the signed in alike (the app's logo
// leads here; `/` sends guests here too).
export const Route = createFileRoute('/land')({
  staticData: { bare: true },
  component: LandingPage,
})
