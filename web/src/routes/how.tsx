import { HowPage } from '@components/how/HowPage'
import { createFileRoute } from '@tanstack/react-router'

// How Stroppy Cloud is built — public, linked from the landing's header.
export const Route = createFileRoute('/how')({
  staticData: { bare: true },
  component: HowPage,
})
