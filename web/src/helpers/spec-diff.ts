export interface SpecChange {
  path: string
  op: 'add' | 'remove' | 'replace'
  a?: unknown
  b?: unknown
}

function isObject(v: unknown): v is Record<string, unknown> {
  return v !== null && typeof v === 'object' && !Array.isArray(v)
}

// Deep field-level diff of two JSON-like values. Arrays are compared as a whole (one change per
// path) so a segment list reads as a single replaced value rather than index noise.
export function diffValues(a: unknown, b: unknown, path = ''): SpecChange[] {
  if (isObject(a) && isObject(b)) {
    const out: SpecChange[] = []
    for (const k of new Set([...Object.keys(a), ...Object.keys(b)])) {
      const p = path ? `${path}.${k}` : k
      if (!(k in a)) out.push({ path: p, op: 'add', b: b[k] })
      else if (!(k in b)) out.push({ path: p, op: 'remove', a: a[k] })
      else out.push(...diffValues(a[k], b[k], p))
    }
    return out
  }
  if (JSON.stringify(a) !== JSON.stringify(b)) return [{ path: path || '.', op: 'replace', a, b }]
  return []
}

export function formatDiffValue(v: unknown): string {
  if (v === undefined) return '—'
  if (v === null) return 'null'
  if (typeof v === 'string') return v
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  const s = JSON.stringify(v)
  return s.length > 80 ? `${s.slice(0, 77)}…` : s
}
