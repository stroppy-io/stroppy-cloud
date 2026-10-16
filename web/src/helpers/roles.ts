import type { TopologyRole } from '@api/types'
import type { IconName } from '@grafana/data'
import type { TFunction } from 'i18next'

// The contract's topology roles: one place in the UI that knows them (a new
// role does not compile until it is here). Its name is `common.topologyRole.*`.
const ROLES: Record<TopologyRole, { icon: IconName; primary?: boolean }> = {
  db: { icon: 'database', primary: true },
  'db-replica': { icon: 'database' },
  'db-compute': { icon: 'database' },
  etcd: { icon: 'layer-group' },
  coordinator: { icon: 'layer-group' },
  proxy: { icon: 'exchange-alt' },
  runner: { icon: 'rocket' },
  patroni: { icon: 'shield' },
  'patroni-replica': { icon: 'shield' },
  pgbouncer: { icon: 'exchange-alt' },
}

export const TOPOLOGY_ROLES = Object.keys(ROLES)

// Some callers hold a role as a plain string (log labels, size keys): an unknown one keeps its
// own name and a neutral icon.
const known = (role: string): role is TopologyRole => role in ROLES

export const roleIcon = (role: string): IconName => (known(role) ? ROLES[role].icon : 'cube')
export const isPrimaryRole = (role: string): boolean => known(role) && !!ROLES[role].primary

/** The role's name in the UI language. */
export function roleLabel(t: TFunction, role: string): string {
  return known(role) ? t(`common.topologyRole.${role}`) : role
}
