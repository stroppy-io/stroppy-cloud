import { SITE_URL } from '@lib/site'
import { createFileRoute, redirect } from '@tanstack/react-router'

// How Stroppy Cloud is built: the page lives on stroppy.io now.
export const Route = createFileRoute('/how')({
  beforeLoad: () => {
    throw redirect({ href: SITE_URL })
  },
})
