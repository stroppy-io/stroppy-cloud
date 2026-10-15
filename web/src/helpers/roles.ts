import type { IconName } from '@grafana/data'
import type { TFunction } from 'i18next'

// The server's topology roles (internal/domain/topology): one place in the UI
// that knows them. A role it does not know keeps its own name and a neutral icon.
const ROLES: Record<string, { icon: IconName; primary?: boolean }> = {
  db: { icon: 'database', primary: true },
  'db-replica': { icon: 'database' },
  'db-compute': { icon: 'database' },
  etcd: { icon: 'layer-group' },
  coordinator: { icon: 'layer-group' },
  proxy: { icon: 'exchange-alt' },
  runner: { icon: 'rocket' },
}

export const TOPOLOGY_ROLES = Object.keys(ROLES)

export const roleIcon = (role: string): IconName => ROLES[role]?.icon ?? 'cube'
export const isPrimaryRole = (role: string): boolean => ROLES[role]?.primary ?? false

/** The role's name in the UI language, or the role itself when it is new. */
export function roleLabel(t: TFunction, role: string): string {
  return ROLES[role] ? t(`common.topologyRole.${role}`) : role
}
