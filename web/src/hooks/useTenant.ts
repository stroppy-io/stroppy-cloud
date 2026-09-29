import type { TenantRole } from '@api/types'
import { useParams } from '@tanstack/react-router'
import { useMe } from './useMe'

const RANK: Record<TenantRole, number> = { viewer: 0, member: 1, admin: 2, owner: 3 }

export type Action =
  | 'view'
  | 'run'
  | 'edit-library'
  | 'cancel-any-run'
  | 'manage-members'
  | 'manage-settings'
  | 'manage-providers'
  | 'manage-tokens'
  | 'delete-tenant'
  | 'transfer'
  | 'share'

const REQUIRED: Record<Action, TenantRole> = {
  view: 'viewer',
  share: 'viewer',
  run: 'member',
  'edit-library': 'member',
  'cancel-any-run': 'admin',
  'manage-members': 'admin',
  'manage-settings': 'admin',
  'manage-providers': 'admin',
  'manage-tokens': 'admin',
  'delete-tenant': 'owner',
  transfer: 'owner',
}

// Current tenant from the `/t/$slug` route + the caller's role in it.
export function useTenant() {
  const { slug: routeSlug } = useParams({ strict: false }) as { slug?: string }
  const me = useMe()
  // Outside /t/$slug (admin, profile, examples) fall back to the preferred tenant so nav links stay valid.
  const fallback =
    me.tenants.find((m) => m.tenant.slug === me.preferences.default_tenant)?.tenant.slug ??
    me.tenants[0]?.tenant.slug
  const slug = routeSlug ?? fallback
  const membership = me.tenants.find((m) => m.tenant.slug === slug)
  const role: TenantRole | undefined =
    membership?.role ?? (me.is_platform_admin ? 'admin' : undefined)
  const can = (action: Action, ownerId?: string): boolean => {
    if (!role) return false
    if (RANK[role] >= RANK[REQUIRED[action]]) return true
    // members may cancel/delete their own runs
    if (action === 'cancel-any-run' && ownerId && ownerId === me.id && RANK[role] >= RANK.member)
      return true
    return false
  }
  return { slug: slug ?? '', tenant: membership?.tenant, role, can, me, inTenantRoute: !!routeSlug }
}
