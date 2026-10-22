import type { components } from '@api/schema'
import { roleLabel } from '@helpers/roles'
import { i18n } from '@lib/i18n'

type Shape = Pick<components['schemas']['TopologyShape'], 'mode' | 'nodes'>

// Roles that run beside the database read by their role (etcd, a coordinator); proxies and
// poolers by the software they run. Patroni is said by the mode; the runner is not the
// database's.
const SERVICE_ROLES = new Set(['coordinator', 'etcd'])
const UNSAID = new Set(['runner', 'patroni', 'patroni-replica'])

/**
 * A topology in the UI language, by its shape rather than its role names: "Single server",
 * "Primary · Replica ×2", "Galera ×3 · proxysql", "Storage ×9 · Query ×3",
 * "Patroni · Primary · Replica ×2 · etcd ×3 · haproxy". Empty for a run whose shape is not known.
 */
export function topologyText(shape: Shape | undefined | null): string {
  if (!shape) return ''
  const t = i18n.t.bind(i18n)
  const times = (name: string, count: number) => (count > 1 ? `${name} ×${count}` : name)
  const has = (role: string) => shape.nodes.some((n) => n.role === role)
  const mode = shape.mode ? t(`common.topologyMode.${shape.mode}`) : ''
  const parts: string[] = []
  let modeSaid = false
  for (const n of shape.nodes) {
    if (UNSAID.has(n.role)) continue
    switch (n.role) {
      case 'db':
        if (has('db-compute')) parts.push(times(t('common.topologyPart.storage'), n.count))
        else if (has('db-replica')) parts.push(t('common.topologyPart.primary'))
        else if (n.count > 1 && mode) {
          // A cluster of equal members is said by its mode: "Galera ×3".
          parts.push(times(mode, n.count))
          modeSaid = true
        } else if (n.count > 1) parts.push(times(t('common.topologyPart.cluster'), n.count))
        else if (!mode) parts.push(t('common.topologyPart.single'))
        break
      case 'db-replica':
        parts.push(times(t('common.topologyPart.replica'), n.count))
        break
      case 'db-compute':
        parts.push(times(t('common.topologyPart.compute'), n.count))
        break
      default:
        parts.push(
          times(SERVICE_ROLES.has(n.role) || !n.engine ? roleLabel(t, n.role) : n.engine, n.count)
        )
    }
  }
  if (mode && !modeSaid) parts.unshift(mode)
  return parts.join(' · ')
}

type Requirement = components['schemas']['RoleRequirement']

/** What drove a role's minimum hardware, in the UI language. */
export function requirementReason(r: Requirement): string {
  if (!r.reason) return ''
  return i18n.t(`library.requirements.reasons.${r.reason}`, r.reason_params ?? {})
}
