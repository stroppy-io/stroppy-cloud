// Small helpers shared by mock handlers.
export function uuid(): string {
  return crypto.randomUUID()
}

export function iso(d: Date | number = Date.now()): string {
  return new Date(d).toISOString()
}

export function minutesAgo(m: number): string {
  return iso(Date.now() - m * 60_000)
}

export function hoursAgo(h: number): string {
  return minutesAgo(h * 60)
}

export function daysAgo(d: number): string {
  return hoursAgo(d * 24)
}

export function durationStr(ms: number): string {
  const s = Math.max(0, Math.round(ms / 1000))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  return h ? `${h}h${m}m${sec}s` : m ? `${m}m${sec}s` : `${sec}s`
}

export function pick<T>(arr: readonly T[], i: number): T {
  return arr[i % arr.length]
}

// Deterministic pseudo-random for stable fixtures.
export function rng(seed: number): () => number {
  let s = seed >>> 0
  return () => {
    s = (s * 1664525 + 1013904223) >>> 0
    return s / 0xffffffff
  }
}

export interface ListQuery {
  cursor?: string | null
  limit: number
  sort?: string
  order: 'asc' | 'desc'
  search?: string
}

export function parseListQuery(q: URLSearchParams, defaultSort = 'created'): ListQuery {
  return {
    cursor: q.get('cursor'),
    limit: Math.min(200, Number(q.get('limit') ?? 25) || 25),
    sort: q.get('sort') ?? defaultSort,
    order: q.get('order') === 'asc' ? 'asc' : 'desc',
    search: q.get('search') ?? q.get('q') ?? undefined,
  }
}

export function multi(q: URLSearchParams, key: string): string[] {
  const vals = q.getAll(key).flatMap((v) => v.split(','))
  return vals.map((v) => v.trim()).filter(Boolean)
}

// Cursor pagination over an already filtered+sorted array; cursor = offset.
export function paginate<T>(
  items: T[],
  lq: ListQuery
): { data: T[]; meta: { next_cursor: string | null; has_more: boolean } } {
  const offset = Number(lq.cursor ?? 0) || 0
  const data = items.slice(offset, offset + lq.limit)
  const has_more = offset + lq.limit < items.length
  return { data, meta: { next_cursor: has_more ? String(offset + lq.limit) : null, has_more } }
}

export function sortBy<T>(
  items: T[],
  key: (t: T) => string | number | null | undefined,
  order: 'asc' | 'desc'
): T[] {
  const dir = order === 'asc' ? 1 : -1
  return [...items].sort((a, b) => {
    const ka = key(a) ?? ''
    const kb = key(b) ?? ''
    if (ka < kb) return -1 * dir
    if (ka > kb) return 1 * dir
    return 0
  })
}

export function matchesSearch(text: string, search?: string): boolean {
  if (!search) return true
  return text.toLowerCase().includes(search.toLowerCase())
}

export function facet(
  field: string,
  values: (string | undefined)[]
): { field: string; values: { value: string; count: number }[] } {
  const counts = new Map<string, number>()
  for (const v of values) if (v) counts.set(v, (counts.get(v) ?? 0) + 1)
  return {
    field,
    values: [...counts]
      .map(([value, count]) => ({ value, count }))
      .sort((a, b) => b.count - a.count),
  }
}
