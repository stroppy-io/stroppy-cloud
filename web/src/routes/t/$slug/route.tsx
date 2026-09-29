import { meQueries } from '@api/queries/me'
import { tenantQueries } from '@api/queries/tenants'
import { AppShell } from '@app/AppShell'
import { createFileRoute, notFound } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug')({
  loader: async ({ context, params }) => {
    const me = await context.queryClient.ensureQueryData(meQueries.me())
    if (!me.tenants.some((m) => m.tenant.slug === params.slug) && !me.is_platform_admin)
      throw notFound()
    const tenant = await context.queryClient.ensureQueryData(tenantQueries.detail(params.slug))
    return { crumb: tenant.name }
  },
  component: AppShell,
})
