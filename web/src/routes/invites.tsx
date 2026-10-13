import { createFileRoute, redirect } from '@tanstack/react-router'

// The link of invite e-mails sent before the invites moved under /me.
export const Route = createFileRoute('/invites')({
  beforeLoad: () => {
    throw redirect({ to: '/me/invites' })
  },
})
