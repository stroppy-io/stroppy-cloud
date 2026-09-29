import { meQueries } from '@api/queries/me'
import { createFileRoute, redirect } from '@tanstack/react-router'

// `/` → the default tenant (or the first membership), or tenant creation when there is none.
export const Route = createFileRoute('/')({
  beforeLoad: async ({ context }) => {
    const me = await context.queryClient.ensureQueryData(meQueries.me())
    const preferred = me.preferences.default_tenant
    const slug =
      me.tenants.find((m) => m.tenant.slug === preferred)?.tenant.slug ?? me.tenants[0]?.tenant.slug
    if (slug) throw redirect({ to: '/t/$slug', params: { slug } })
    throw redirect({ to: '/tenants/new' })
  },
})
