import { z } from 'zod'
import type { ExplorerState } from './MetricExplorer'

// The explorer's URL keys, shared by the run's Metrics tab and the public report: search,
// filters, sort and the picked charts (`m`, encodePick form) — a link opens the same view.
const list = z.array(z.string()).optional().catch(undefined)
export const explorerSearchSchema = z.object({
  xq: z.string().optional().catch(undefined),
  xorigin: list,
  xkind: list,
  xrole: list,
  xmachine: list,
  xcontainer: list,
  xsort: z.enum(['name', '-name', 'series', '-series', 'origin']).optional().catch(undefined),
  m: list,
})
export type ExplorerSearch = z.infer<typeof explorerSearchSchema>

export function explorerStateOf(s: ExplorerSearch): ExplorerState {
  return {
    q: s.xq,
    origin: s.xorigin,
    kind: s.xkind,
    role: s.xrole,
    machine: s.xmachine,
    container: s.xcontainer,
    sort: s.xsort,
    pick: s.m,
  }
}

export function explorerSearchOf(next: Partial<ExplorerState>): Partial<ExplorerSearch> {
  const out: Partial<ExplorerSearch> = {}
  if ('q' in next) out.xq = next.q
  if ('origin' in next) out.xorigin = next.origin
  if ('kind' in next) out.xkind = next.kind
  if ('role' in next) out.xrole = next.role
  if ('machine' in next) out.xmachine = next.machine
  if ('container' in next) out.xcontainer = next.container
  if ('sort' in next) out.xsort = next.sort
  if ('pick' in next) out.m = next.pick
  return out
}
