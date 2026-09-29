// Column-filter metadata carried by `DataTableColumn.filter`. The values are what the URL
// currently holds; `onChange` writes back to the URL (the page owns the search state).

export interface ChecklistOption {
  value: string
  label: string
  count?: number
}

export interface ChecklistColumnFilter {
  kind: 'checklist'
  options: ChecklistOption[]
  value: string[] | undefined
  // Radio-like: picking an option replaces the selection (server param takes one value).
  single?: boolean
  onChange: (next: string[] | undefined) => void
}

export interface TextColumnFilter {
  kind: 'text'
  value: string | undefined
  placeholder?: string
  onChange: (next: string | undefined) => void
}

export interface DateColumnFilter {
  kind: 'date'
  from: string | undefined
  to: string | undefined
  onChange: (from: string | undefined, to: string | undefined) => void
}

export interface NumberColumnFilter {
  kind: 'number'
  min: number | undefined
  max: number | undefined
  unit?: string
  step?: number
  onChange: (min: number | undefined, max: number | undefined) => void
}

export type ColumnFilter =
  | ChecklistColumnFilter
  | TextColumnFilter
  | DateColumnFilter
  | NumberColumnFilter

export function isFilterActive(f: ColumnFilter | undefined): boolean {
  if (!f) return false
  switch (f.kind) {
    case 'checklist':
      return (f.value?.length ?? 0) > 0
    case 'text':
      return !!f.value
    case 'date':
      return !!f.from || !!f.to
    case 'number':
      return f.min !== undefined || f.max !== undefined
  }
}
