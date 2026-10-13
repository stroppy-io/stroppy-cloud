import type { Schemas } from '@api/types'

// A validation issue's `suggested` value: a plain value (a size, a number of
// GB) or, for a role without a size, the preset that fits it `{size, disk?}`.

type Suggested = { size: Schemas['Size']; disk?: { gb?: number } }

/** The suggested preset of a role, when the suggestion is one. */
export function suggestedRoleSize(v: unknown): Suggested | undefined {
  if (v && typeof v === 'object' && 'size' in v && typeof (v as Suggested).size === 'string') {
    return v as Suggested
  }
  return undefined
}

/** How a suggestion reads in a sentence: «S», «S · 120 GB», «200». */
export function suggestedText(v: unknown): string {
  const rs = suggestedRoleSize(v)
  if (rs) return rs.disk?.gb ? `${rs.size} · ${rs.disk.gb} GB` : rs.size
  return String(v)
}
