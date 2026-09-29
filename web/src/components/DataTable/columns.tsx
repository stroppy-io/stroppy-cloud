import { FavoriteButton } from '@components/FavoriteButton'
import { RelativeTime } from '@components/RelativeTime'
import { StatusBadge } from '@components/StatusBadge'
import type { IconName } from '@grafana/data'
import { durationSeconds, formatDuration, formatMetric } from '@helpers/format'
import type { RowData } from '@tanstack/react-table'
import i18next from 'i18next'
import type { ReactNode } from 'react'
import type { SortOption } from './ColumnHeader'
import {
  BoolCell,
  type CellLink,
  Dash,
  IdentityCell,
  LinkCell,
  NumberCell,
  PairCell,
  StackCell,
  TagsCell,
  TextCell,
} from './cells'
import type { DataTableColumn } from './DataTable'
import { type RowAction, RowActionsMenu } from './RowActionsMenu'
import type { ColumnFilter } from './types'

// Column factories (web/docs/tables-guide.md §2–§3). Each one fixes the column's type: width,
// alignment, monospace, pinning and the cell renderer. Pages describe *what* a column shows;
// *how* it reads is decided here once for every table.
//
//   const columns = [
//     col.status({ id: 'status', title: t('…'), status: (r) => r.status, sortKey: 'status' }),
//     col.identity({ id: 'name', header: t('…'), sortKey: 'name', render: (r) => ({ … }) }),
//     col.number({ id: 'tps', header: 'TPS', unit: 'tps', value: (r) => r.tps, bar: true }),
//     col.time({ id: 'started_at', header: t('…'), value: (r) => r.started_at }),
//     col.favorite({ kind: 'run', id: (r) => r.id, value: (r) => r.is_favorite }),
//     col.actions({ title: (r) => r.name, actions: rowActions }),
//   ]

export interface ColumnBase {
  id: string
  header?: string
  // Required when `header` is empty (icon columns): settings menu + accessible label.
  title?: string
  sortKey?: string
  // Composite column: several sort keys offered in a menu instead of one `sortKey`.
  sortOptions?: SortOption[]
  filter?: ColumnFilter
  width?: number
  minWidth?: number
  hideable?: boolean
  defaultHidden?: boolean
}

// Standard widths per cell type (px). Flexible text columns get a `minWidth` instead.
export const WIDTH = {
  icon: 56,
  status: 132,
  number: 104,
  duration: 156,
  time: 132,
  bool: 88,
  tags: 180,
  pair: 170,
  link: 190,
  favorite: 40,
  actions: 44,
} as const

function base<T extends RowData>(b: ColumnBase): Partial<DataTableColumn<T>> {
  return {
    id: b.id,
    header: b.header ?? '',
    title: b.title,
    sortKey: b.sortKey,
    sortOptions: b.sortOptions,
    filter: b.filter,
    width: b.width,
    minWidth: b.minWidth,
    hideable: b.hideable,
    defaultHidden: b.defaultHidden,
  }
}

// Largest value of a column on the current page, cached per row-model instance so a column of
// N cells computes it once, not N times.
const maxCache = new WeakMap<object, Map<string, number>>()
function columnMax<T>(rows: { original: T }[], id: string, value: (r: T) => number | undefined) {
  let byCol = maxCache.get(rows)
  if (!byCol) {
    byCol = new Map()
    maxCache.set(rows, byCol)
  }
  const hit = byCol.get(id)
  if (hit !== undefined) return hit
  let max = 0
  for (const r of rows) {
    const v = value(r.original)
    if (v !== undefined && v !== null && !Number.isNaN(v) && v > max) max = v
  }
  byCol.set(id, max)
  return max
}

export const col = {
  // First column: the human-readable record id (name + secondary line), pinned left, never hidden.
  identity<T extends RowData>(
    b: ColumnBase & {
      render: (r: T) => {
        title: string
        link?: CellLink
        subtitle?: ReactNode
        icon?: IconName
        lead?: ReactNode
        badges?: ReactNode
        progress?: number
      }
    }
  ): DataTableColumn<T> {
    return {
      ...base<T>(b),
      minWidth: b.minWidth ?? 260,
      sticky: 'left',
      hideable: false,
      accessorFn: (r: T) => b.render(r).title,
      cell: ({ row }) => <IdentityCell {...b.render(row.original)} />,
    } as DataTableColumn<T>
  },

  // Narrow status icon with the label in the tooltip. Put the status first so rows scan by it.
  status<T extends RowData>(
    b: ColumnBase & {
      title: string
      status: (r: T) => string | undefined
      // Tooltip text when the entity words its states differently («На паузе», «Отозвана»).
      label?: (r: T) => string | undefined
    }
  ): DataTableColumn<T> {
    return {
      ...base<T>(b),
      width: b.width ?? WIDTH.icon,
      align: 'center',
      tight: true,
      sticky: 'left',
      hideable: false,
      accessorFn: (r: T) => b.status(r),
      cell: ({ row }) => (
        <StatusBadge status={b.status(row.original)} label={b.label?.(row.original)} iconOnly />
      ),
    } as DataTableColumn<T>
  },

  // Status pill with text, for tables where the status is a column of its own.
  statusText<T extends RowData>(
    b: ColumnBase & { status: (r: T) => string | undefined; label?: (r: T) => string | undefined }
  ): DataTableColumn<T> {
    return {
      ...base<T>(b),
      width: b.width ?? WIDTH.status,
      accessorFn: (r: T) => b.status(r),
      cell: ({ row }) => {
        const s = b.status(row.original)
        return s ? <StatusBadge status={s} label={b.label?.(row.original)} /> : <Dash />
      },
    } as DataTableColumn<T>
  },

  text<T extends RowData>(
    b: ColumnBase & { value: (r: T) => string | undefined | null }
  ): DataTableColumn<T> {
    return {
      ...base<T>(b),
      minWidth: b.width ? undefined : (b.minWidth ?? 140),
      accessorFn: (r: T) => b.value(r) ?? '',
      cell: ({ row }) => <TextCell value={b.value(row.original)} />,
    } as DataTableColumn<T>
  },

  // One entity in two lines: identifying value over its qualifier. The default for composite
  // columns (database = engine over topology, time = start over duration).
  stack<T extends RowData>(
    b: ColumnBase & {
      render: (r: T) => { primary: ReactNode; secondary?: ReactNode; title?: string }
      value?: (r: T) => string | number | undefined
      align?: 'left' | 'right'
    }
  ): DataTableColumn<T> {
    return {
      ...base<T>(b),
      minWidth: b.width ? undefined : (b.minWidth ?? 180),
      align: b.align,
      accessorFn: b.value ? (r: T) => b.value?.(r) : undefined,
      cell: ({ row }) => <StackCell {...b.render(row.original)} />,
    } as DataTableColumn<T>
  },

  // Value + qualifier in one line: «postgres 17», «YC live · M».
  pair<T extends RowData>(
    b: ColumnBase & {
      primary: (r: T) => string | undefined
      secondary?: (r: T) => string | undefined
      icon?: IconName
    }
  ): DataTableColumn<T> {
    return {
      ...base<T>(b),
      width: b.width ?? WIDTH.pair,
      accessorFn: (r: T) => b.primary(r) ?? '',
      cell: ({ row }) => (
        <PairCell
          primary={b.primary(row.original)}
          secondary={b.secondary?.(row.original)}
          icon={b.icon}
        />
      ),
    } as DataTableColumn<T>
  },

  // Right-aligned tabular number. `unit` formats through `formatMetric` (ms, tps, bytes, %);
  // `bar` draws the value against the page maximum so the column compares at a glance.
  number<T extends RowData>(
    b: ColumnBase & {
      value: (r: T) => number | undefined | null
      unit?: string
      format?: (v: number) => string
      bar?: boolean
    }
  ): DataTableColumn<T> {
    const v = (r: T) => b.value(r) ?? undefined
    return {
      ...base<T>(b),
      width: b.width ?? WIDTH.number,
      align: 'right',
      accessorFn: (r: T) => v(r),
      cell: ({ row, table }) => {
        const value = v(row.original)
        const text =
          value === undefined ? undefined : b.format ? b.format(value) : formatMetric(value, b.unit)
        const max = b.bar ? columnMax(table.getRowModel().rows, b.id, v) : undefined
        return <NumberCell value={value} text={text} max={max} />
      },
    } as DataTableColumn<T>
  },

  // Duration from seconds or a Go duration string, right-aligned, optional bar.
  duration<T extends RowData>(
    b: ColumnBase & { value: (r: T) => string | number | undefined | null; bar?: boolean }
  ): DataTableColumn<T> {
    const v = (r: T) => durationSeconds(b.value(r))
    return {
      ...base<T>(b),
      width: b.width ?? WIDTH.duration,
      align: 'right',
      accessorFn: (r: T) => v(r),
      cell: ({ row, table }) => {
        const value = v(row.original)
        const max = b.bar ? columnMax(table.getRowModel().rows, b.id, v) : undefined
        return <NumberCell value={value} text={formatDuration(value)} max={max} />
      },
    } as DataTableColumn<T>
  },

  // Relative time («21 час назад») with the absolute timestamp in the tooltip.
  time<T extends RowData>(
    b: ColumnBase & { value: (r: T) => string | undefined | null }
  ): DataTableColumn<T> {
    return {
      ...base<T>(b),
      width: b.width ?? WIDTH.time,
      accessorFn: (r: T) => b.value(r) ?? '',
      cell: ({ row }) => {
        const value = b.value(row.original)
        return value ? <RelativeTime value={value} /> : <Dash />
      },
    } as DataTableColumn<T>
  },

  tags<T extends RowData>(
    b: ColumnBase & { items: (r: T) => string[]; max?: number }
  ): DataTableColumn<T> {
    return {
      ...base<T>(b),
      width: b.width ?? WIDTH.tags,
      accessorFn: (r: T) => b.items(r).join(','),
      cell: ({ row }) => <TagsCell items={b.items(row.original)} max={b.max} />,
    } as DataTableColumn<T>
  },

  // Reference to another record: kind icon + name link.
  link<T extends RowData>(
    b: ColumnBase & {
      render: (r: T) => {
        text: string | undefined
        link?: CellLink
        secondary?: string
      }
      icon?: IconName
    }
  ): DataTableColumn<T> {
    return {
      ...base<T>(b),
      width: b.width ?? WIDTH.link,
      accessorFn: (r: T) => b.render(r).text ?? '',
      cell: ({ row }) => <LinkCell {...b.render(row.original)} icon={b.icon} />,
    } as DataTableColumn<T>
  },

  bool<T extends RowData>(
    b: ColumnBase & { value: (r: T) => boolean | undefined }
  ): DataTableColumn<T> {
    return {
      ...base<T>(b),
      width: b.width ?? WIDTH.bool,
      align: 'center',
      accessorFn: (r: T) => (b.value(r) ? 1 : 0),
      cell: ({ row }) => <BoolCell value={b.value(row.original)} label={b.title ?? b.header} />,
    } as DataTableColumn<T>
  },

  // Escape hatch for a cell the kit does not cover (trend sparkline, sizes). Keep it rare.
  custom<T extends RowData>(
    b: ColumnBase & {
      cell: (r: T) => ReactNode
      value?: (r: T) => string | number | undefined
      align?: 'left' | 'right' | 'center'
      mono?: boolean
      tight?: boolean
    }
  ): DataTableColumn<T> {
    return {
      ...base<T>(b),
      align: b.align,
      mono: b.mono,
      tight: b.tight,
      accessorFn: b.value ? (r: T) => b.value?.(r) : undefined,
      cell: ({ row }) => b.cell(row.original),
    } as DataTableColumn<T>
  },

  favorite<T extends RowData>(b: {
    kind: 'run' | 'database' | 'workload' | 'test' | 'suite' | 'schedule'
    id: (r: T) => string
    value: (r: T) => boolean | undefined
  }): DataTableColumn<T> {
    return {
      id: 'fav',
      header: '',
      title: i18next.t('common.table.favorite'),
      width: WIDTH.favorite,
      tight: true,
      align: 'center',
      sticky: 'right',
      hideable: false,
      cell: ({ row }) => (
        <FavoriteButton
          kind={b.kind}
          id={b.id(row.original)}
          value={b.value(row.original)}
          size="sm"
        />
      ),
    } as DataTableColumn<T>
  },

  actions<T extends RowData>(b: {
    title: (r: T) => string
    actions: (r: T) => RowAction[]
  }): DataTableColumn<T> {
    return {
      id: 'actions',
      header: '',
      title: i18next.t('common.table.actions'),
      width: WIDTH.actions,
      tight: true,
      align: 'center',
      sticky: 'right',
      hideable: false,
      cell: ({ row }) => (
        <RowActionsMenu title={b.title(row.original)} actions={b.actions(row.original)} />
      ),
    } as DataTableColumn<T>
  },
}
