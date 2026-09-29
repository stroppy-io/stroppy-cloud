import type { Schemas } from '@api/types'

export const SIZE_ORDER: Schemas['Size'][] = ['XS', 'S', 'M', 'L', 'XL']

// Roles shown first in every sizes editor/summary; others follow alphabetically.
const ROLE_ORDER = ['db', 'proxy', 'etcd', 'runner']

export function sortRoles(roles: string[]): string[] {
  return [...roles].sort((a, b) => {
    const ia = ROLE_ORDER.indexOf(a)
    const ib = ROLE_ORDER.indexOf(b)
    if (ia === -1 && ib === -1) return a.localeCompare(b)
    if (ia === -1) return 1
    if (ib === -1) return -1
    return ia - ib
  })
}

// "db M · runner S"
export function formatRoleSizes(sizes: Schemas['RoleSizes'] | undefined): string {
  if (!sizes) return ''
  return sortRoles(Object.keys(sizes))
    .map((r) => `${r} ${sizes[r].size}`)
    .join(' · ')
}

// Only the roles that differ from the base: "db L" (or empty when identical).
export function diffRoleSizes(
  sizes: Schemas['RoleSizes'] | undefined,
  base: Schemas['RoleSizes'] | undefined
): string {
  if (!sizes) return ''
  if (!base) return formatRoleSizes(sizes)
  const changed = sortRoles(Object.keys(sizes)).filter((r) => sizes[r].size !== base[r]?.size)
  return changed.map((r) => `${r} ${sizes[r].size}`).join(' · ')
}
