import { SITE_URL } from '@lib/site'
import { createFileRoute, redirect } from '@tanstack/react-router'

// The console's old front page: the landing lives on stroppy.io now.
export const Route = createFileRoute('/land')({
  beforeLoad: () => {
    throw redirect({ href: SITE_URL })
  },
})
