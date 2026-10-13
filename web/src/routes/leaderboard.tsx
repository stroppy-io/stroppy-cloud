import { PublicHeader } from '@components/landing/PublicHeader'
import { createFileRoute } from '@tanstack/react-router'

// The public rating (no sign-in). Built next; the header already links here.
export const Route = createFileRoute('/leaderboard')({
  staticData: { bare: true },
  component: () => <PublicHeader />,
})
