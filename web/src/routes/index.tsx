import { meQueries } from '@api/queries/me'
import { getAccessToken } from '@lib/auth'
import { loadPublicConfig } from '@lib/config'
import { createFileRoute, redirect } from '@tanstack/react-router'

// `/` → the landing (/land) for a guest, keeping its #anchor; for the signed
// in: the welcome screen on the first sign-in, then the default tenant (or the
// first membership), or tenant creation when there is none.
export const Route = createFileRoute('/')({
  staticData: { bare: true },
  beforeLoad: async ({ context, location }) => {
    const cfg = await loadPublicConfig()
    if (cfg.auth.mode === 'kratos' && !(await getAccessToken())) {
      throw redirect({ to: '/land', hash: location.hash || undefined })
    }
    const me = await context.queryClient.ensureQueryData(meQueries.me())
    if (!me.onboarding?.welcome_done) throw redirect({ to: '/welcome' })
    const preferred = me.preferences.default_tenant
    const slug =
      me.tenants.find((m) => m.tenant.slug === preferred)?.tenant.slug ?? me.tenants[0]?.tenant.slug
    if (slug) throw redirect({ to: '/t/$slug', params: { slug } })
    throw redirect({ to: '/tenants/new' })
  },
})
