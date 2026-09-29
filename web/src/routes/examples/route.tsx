import { meQueries } from '@api/queries/me'
import { AppShell } from '@app/AppShell'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/examples')({
  loader: ({ context }) => context.queryClient.ensureQueryData(meQueries.me()),
  component: AppShell,
})
