import { meQueries } from '@api/queries/me'
import { getAccessToken } from '@lib/auth'
import { loadPublicConfig } from '@lib/config'
import { SITE_URL } from '@lib/site'
import { createFileRoute, redirect } from '@tanstack/react-router'

// `/` → stroppy.io for a guest (the console has no front page); for the
// signed in: the welcome screen on the first sign-in, then the default tenant (or the
// first membership), or tenant creation when there is none.
export const Route = createFileRoute('/')({
  staticData: { bare: true },
  beforeLoad: async ({ context }) => {
    const cfg = await loadPublicConfig()
    if (cfg.auth.mode === 'kratos' && !(await getAccessToken())) {
      throw redirect({ href: SITE_URL })
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
