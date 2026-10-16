import type { components } from '@api/schema'
import { roleLabel } from '@helpers/roles'
import { i18n } from '@lib/i18n'

type Shape = Pick<components['schemas']['TopologyShape'], 'mode' | 'nodes'>

// Roles that are the database itself read by their role; the rest (proxies, pgbouncer) by the
// software they run. Patroni is said by the mode; the runner is not the database's.
const DATABASE_ROLES = new Set(['db', 'db-replica', 'db-compute', 'coordinator', 'etcd'])
const UNSAID = new Set(['runner', 'patroni', 'patroni-replica'])

/**
 * A topology in the UI language: "Patroni · Primary · Replica ×2 · etcd ×3 · haproxy".
 * Empty for a run whose shape is not known.
 */
export function topologyText(shape: Shape | undefined | null): string {
  if (!shape) return ''
  const t = i18n.t.bind(i18n)
  const parts: string[] = []
  if (shape.mode) parts.push(t(`common.topologyMode.${shape.mode}`))
  for (const n of shape.nodes) {
    if (UNSAID.has(n.role)) continue
    const name = DATABASE_ROLES.has(n.role) || !n.engine ? roleLabel(t, n.role) : n.engine
    parts.push(n.count > 1 ? `${name} ×${n.count}` : name)
  }
  return parts.join(' · ')
}

type Requirement = components['schemas']['RoleRequirement']

/** What drove a role's minimum hardware, in the UI language. */
export function requirementReason(r: Requirement): string {
  if (!r.reason) return ''
  return i18n.t(`library.requirements.reasons.${r.reason}`, r.reason_params ?? {})
}
