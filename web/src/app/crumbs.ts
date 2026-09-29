import { useMatches } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

// Breadcrumbs are a router feature, not a page prop. Every route contributes its crumb in one of
// two ways and the shell renders the chain from the matched routes:
//   1. `staticData: { crumb: 'nav.runs' }`          — i18n key (static section)
//   2. `loader` returns `{ crumb: run.name, ... }`  — entity name from data (detail routes)
// A route without either is skipped (layouts, index routes). Links point at each match's pathname.
export interface CrumbContext {
  params: Record<string, string>
  loaderData: unknown
  t: (key: string) => string
}

declare module '@tanstack/react-router' {
  interface StaticDataRouteOption {
    crumb?: string | ((ctx: CrumbContext) => ReactNode)
    // Hide the section nav for this subtree (public pages).
    bare?: boolean
  }
}

export interface Crumb {
  key: string
  label: ReactNode
  href: string
  current: boolean
}

export function useBreadcrumbs(): Crumb[] {
  const { t } = useTranslation()
  const matches = useMatches()
  const out: Crumb[] = []
  matches.forEach((m, i) => {
    const data = m.loaderData as { crumb?: ReactNode } | undefined
    const stat = (
      m.staticData as { crumb?: string | ((ctx: CrumbContext) => ReactNode) } | undefined
    )?.crumb
    let label: ReactNode | undefined
    if (data && typeof data === 'object' && data.crumb !== undefined) label = data.crumb
    else if (typeof stat === 'function')
      label = stat({ params: m.params as Record<string, string>, loaderData: m.loaderData, t })
    else if (typeof stat === 'string') label = t(stat)
    if (label === undefined || label === null || label === '') return
    const href = m.pathname.replace(/\/$/, '') || '/'
    // collapse consecutive matches that resolve to the same path (layout + index)
    const prev = out[out.length - 1]
    if (prev && prev.href === href) {
      prev.label = label
      prev.key = m.id
      return
    }
    out.push({ key: m.id, label, href, current: i === matches.length - 1 })
  })
  if (out.length) out[out.length - 1].current = true
  return out
}

export function useIsBare(): boolean {
  const matches = useMatches()
  return matches.some((m) => (m.staticData as { bare?: boolean } | undefined)?.bare)
}
