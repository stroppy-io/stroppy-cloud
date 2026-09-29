import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'

// Per-viewer table preferences (web/docs/tables-guide.md §8): which columns are visible and the
// row density. They live in localStorage under `stroppy.table.<id>`; the URL keeps only what
// describes the data (filters, sort, page size), not how this viewer likes to look at it.

export type Density = 'compact' | 'default' | 'comfortable'
export const DENSITIES: Density[] = ['compact', 'default', 'comfortable']

// Vertical cell padding per density, in theme spacing units (8px).
export const DENSITY_PAD: Record<Density, number> = { compact: 0.5, default: 1, comfortable: 1.5 }
// Estimated row height for the virtualizer: a two-line identity cell at each density.
export const DENSITY_ROW: Record<Density, number> = { compact: 32, default: 52, comfortable: 60 }

// Cells read the density to drop secondary lines in compact mode.
export const DensityContext = createContext<Density>('default')
export function useDensity(): Density {
  return useContext(DensityContext)
}

// What a column exposes to the settings menu.
export interface PrefColumn {
  id?: string
  title?: string
  header?: unknown
  hideable?: boolean
  defaultHidden?: boolean
}

interface Stored {
  hidden?: string[]
  shown?: string[]
  density?: Density
}

function read(key: string): Stored {
  try {
    const v = JSON.parse(localStorage.getItem(key) ?? 'null')
    return v && typeof v === 'object' ? (v as Stored) : {}
  } catch {
    return {}
  }
}

function write(key: string, v: Stored) {
  try {
    localStorage.setItem(key, JSON.stringify(v))
  } catch {
    // private mode / blocked storage: preferences simply do not persist
  }
}

export function columnTitle(c: PrefColumn): string {
  if (c.title) return c.title
  if (typeof c.header === 'string' && c.header) return c.header
  return c.id ?? ''
}

export interface TablePrefs<C extends PrefColumn> {
  // Columns to render, in declaration order, with the hidden ones removed.
  visible: C[]
  // Every column the viewer can toggle (non-hideable ones are listed as locked).
  all: C[]
  isVisible: (id: string) => boolean
  toggle: (id: string) => void
  density: Density
  setDensity: (d: Density) => void
  // True when anything differs from the table's defaults.
  customized: boolean
  reset: () => void
}

// Explicit choices are stored as deltas against the column defaults (`hidden` for default-visible
// columns the viewer turned off, `shown` for default-hidden ones they turned on), so a column
// added later appears according to its own default rather than the viewer's old snapshot.
export function useTablePrefs<C extends PrefColumn>(tableId: string, columns: C[]): TablePrefs<C> {
  const key = `stroppy.table.${tableId}`
  const [stored, setStored] = useState<Stored>(() => read(key))
  useEffect(() => write(key, stored), [key, stored])

  const hidden = useMemo(() => new Set(stored.hidden ?? []), [stored.hidden])
  const shown = useMemo(() => new Set(stored.shown ?? []), [stored.shown])

  const isVisibleCol = useCallback(
    (c: C) => {
      const id = c.id ?? ''
      if (c.hideable === false) return true
      return c.defaultHidden ? shown.has(id) : !hidden.has(id)
    },
    [hidden, shown]
  )

  const visible = useMemo(() => columns.filter(isVisibleCol), [columns, isVisibleCol])

  const isVisible = useCallback(
    (id: string) => {
      const c = columns.find((x) => x.id === id)
      return c ? isVisibleCol(c) : false
    },
    [columns, isVisibleCol]
  )

  const toggle = useCallback(
    (id: string) => {
      const c = columns.find((x) => x.id === id)
      if (!c || c.hideable === false) return
      setStored((s) => {
        const h = new Set(s.hidden ?? [])
        const sh = new Set(s.shown ?? [])
        if (c.defaultHidden) {
          if (sh.has(id)) sh.delete(id)
          else sh.add(id)
        } else if (h.has(id)) h.delete(id)
        else h.add(id)
        return { ...s, hidden: [...h], shown: [...sh] }
      })
    },
    [columns]
  )

  const density = stored.density ?? 'default'
  const setDensity = useCallback((d: Density) => setStored((s) => ({ ...s, density: d })), [])
  const reset = useCallback(() => setStored({}), [])
  const customized =
    (stored.hidden?.length ?? 0) > 0 ||
    (stored.shown?.length ?? 0) > 0 ||
    (stored.density !== undefined && stored.density !== 'default')

  return { visible, all: columns, isVisible, toggle, density, setDensity, customized, reset }
}
