import { ErrorState } from '@app/ErrorState'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Checkbox, EmptyState, Icon, Text, useStyles2 } from '@grafana/ui'
import {
  type ColumnDef,
  createSortedRowModel,
  flexRender,
  type RowData,
  type RowSelectionState,
  rowSelectionFeature,
  rowSortingFeature,
  type SortingState,
  tableFeatures,
  useTable,
} from '@tanstack/react-table'
import { useVirtualizer } from '@tanstack/react-virtual'
import { type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ColumnHeader, type SortDir, type SortOption } from './ColumnHeader'
import { OverlayContext, type OverlayRegistry } from './overlay'
import { DENSITY_PAD, DENSITY_ROW, type Density, DensityContext } from './prefs'
import type { ColumnFilter } from './types'

// List table (web/docs/tables-guide.md; conventions in web/AGENTS.md §17). Columns are built
// with the `col.*` factories from `./columns`, cells from `./cells`; this component owns layout,
// sticky columns, sorting, selection, density and every empty/loading/error state.

const features = tableFeatures({
  rowSortingFeature,
  rowSelectionFeature,
  sortedRowModel: createSortedRowModel(),
})

export type DataTableColumn<T extends RowData> = ColumnDef<typeof features, T, unknown> & {
  width?: number | string
  // Minimum width reserved for a flexible (no `width`) column; default 220.
  minWidth?: number
  align?: 'left' | 'right' | 'center'
  mono?: boolean
  // Server sort key sent as `sort=`; absent → the column shows no sort control
  // (in `clientSort` mode any column with an accessor is sortable instead).
  sortKey?: string
  // Composite column: sort by one of several server keys, picked from a menu.
  sortOptions?: SortOption[]
  // Header filter popover; the page maps its value to the URL.
  filter?: ColumnFilter
  // Pinned while the table scrolls horizontally: `left` for the identity (a leading prefix of
  // columns, every one but the last with a numeric width), `right` for favorite and actions.
  sticky?: 'left' | 'right'
  // Name in the column settings and the header's accessible label when the header is an icon.
  title?: string
  // `false` keeps the column out of the visibility toggle (identity, actions).
  hideable?: boolean
  // Icon-only column (status, trigger, favorite, actions): minimal horizontal padding.
  tight?: boolean
  // Off until the viewer turns it on in the column settings.
  defaultHidden?: boolean
}

export interface DataTableSort {
  field: string
  order: 'asc' | 'desc'
}

export interface DataTableEmpty {
  message: string
  // One sentence under the message: what this list is for / how it gets filled.
  hint?: string
  // Primary action that fills the table (e.g. «Создать тест»).
  button?: ReactNode
}

export interface DataTableProps<T extends RowData> {
  columns: DataTableColumn<T>[]
  data: T[]
  getRowId?: (row: T) => string
  // Server-driven sorting: current sort (matched against `sortKey`) + change handler.
  // `undefined` from the handler means "back to neutral" (page restores its default).
  sort?: DataTableSort
  onSortChange?: (sort: DataTableSort | undefined) => void
  // In-memory sorting of the loaded rows (bounded lists such as the rating).
  clientSort?: boolean
  // Row selection (checkbox column is added automatically when provided).
  selected?: Record<string, boolean>
  onSelectedChange?: (sel: Record<string, boolean>) => void
  // Bar shown above the table while rows are selected: «Выбрано N · <actions> · Снять выбор».
  bulkActions?: (ids: string[]) => ReactNode
  // Clicking a row (or Enter on it) navigates; the identity cell carries the real link.
  rowHref?: (row: T) => string | undefined
  onRowClick?: (row: T) => void
  rowClassName?: (row: T) => string | undefined
  loading?: boolean
  error?: unknown
  onRetry?: () => void
  // No records at all → call-to-action.
  empty?: DataTableEmpty
  // Filters are active: an empty result means «nothing matches», with a reset.
  filtered?: boolean
  onClearFilters?: () => void
  // Legacy free-form empty content; prefer `empty` + `filtered`.
  emptyState?: ReactNode
  density?: Density
  // Fill the rest of a `fill` page and scroll rows inside the table: the page itself never
  // scrolls, the header row and the toolbar above stay in view (web/AGENTS.md §18).
  fill?: boolean
  // Virtualize when the list is long (needs a bounded height: `fill` or `height`).
  virtual?: boolean
  height?: number | string
  rowHeight?: number
  // Legacy alias of `density="compact"`.
  dense?: boolean
  footer?: ReactNode
  // True while a filter popover or a row menu is open — pages pause auto-refresh on it.
  onOverlayChange?: (open: boolean) => void
  'aria-label'?: string
}

const CHECK_W = 36
const SKELETON_ROWS = 8

const getStyles = (theme: GrafanaTheme2) => ({
  wrap: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    overflow: 'hidden',
  }),
  scroll: css({ overflow: 'auto', position: 'relative' }),
  fill: css({ flex: '1 1 auto', minHeight: 0, display: 'flex', flexDirection: 'column' }),
  fillScroll: css({ flex: '1 1 auto', minHeight: 0, overscrollBehavior: 'contain' }),
  table: css({
    width: '100%',
    borderCollapse: 'separate',
    borderSpacing: 0,
    tableLayout: 'fixed',
    fontSize: theme.typography.body.fontSize,
  }),
  th: css({
    position: 'sticky',
    top: 0,
    zIndex: 1,
    textAlign: 'left',
    padding: theme.spacing(0.75, 1),
    background: theme.colors.background.secondary,
    borderBottom: `1px solid ${theme.colors.border.weak}`,
    color: theme.colors.text.secondary,
    fontWeight: theme.typography.fontWeightMedium,
    fontSize: theme.typography.bodySmall.fontSize,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    userSelect: 'none',
  }),
  // Cells decide their own wrapping (text wraps to two lines, numbers never wrap).
  td: css({
    borderBottom: `1px solid ${theme.colors.border.weak}`,
    verticalAlign: 'middle',
    overflow: 'hidden',
    overflowWrap: 'anywhere',
  }),
  pad: {
    compact: css({ padding: theme.spacing(DENSITY_PAD.compact, 1.5) }),
    default: css({ padding: theme.spacing(DENSITY_PAD.default, 1.5) }),
    comfortable: css({ padding: theme.spacing(DENSITY_PAD.comfortable, 1.5) }),
  } satisfies Record<Density, string>,
  tr: css({
    '&:hover td': { background: theme.colors.action.hover },
    '&:hover td[data-sticky]': {
      background: theme.colors.emphasize(theme.colors.background.primary, 0.03),
    },
    '&:last-child td': { borderBottom: 0 },
  }),
  trClickable: css({
    cursor: 'pointer',
    '&:focus-visible': { outline: `2px solid ${theme.colors.primary.border}`, outlineOffset: -2 },
  }),
  trSelected: css({
    td: { background: theme.colors.action.selected },
    // Pinned cells must stay opaque or the scrolled columns show through them.
    'td[data-sticky]': {
      background: theme.colors.emphasize(theme.colors.background.primary, 0.08),
    },
    'td:first-child': { boxShadow: `inset 3px 0 0 ${theme.colors.primary.main}` },
  }),
  right: css({ textAlign: 'right' }),
  center: css({ textAlign: 'center' }),
  sticky: css({ position: 'sticky', zIndex: 2, background: theme.colors.background.primary }),
  stickyTh: css({ zIndex: 3, background: theme.colors.background.secondary }),
  edgeRight: css({ boxShadow: `inset 1px 0 0 ${theme.colors.border.weak}` }),
  edgeLeft: css({ boxShadow: `inset -1px 0 0 ${theme.colors.border.weak}` }),
  mono: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  checkCell: css({ width: CHECK_W, padding: theme.spacing(0, 1) }),
  tight: css({ paddingLeft: theme.spacing(0.5), paddingRight: theme.spacing(0.5) }),
  // Empty / error block under the header, centred on the visible area (not the full table width).
  state: css({ padding: theme.spacing(4, 2), whiteSpace: 'normal' }),
  stateIcon: css({ color: theme.colors.text.disabled }),
  skeleton: css({
    height: 10,
    borderRadius: theme.shape.radius.default,
    background: `linear-gradient(90deg, ${theme.colors.background.secondary} 25%, ${theme.colors.action.hover} 50%, ${theme.colors.background.secondary} 75%)`,
    backgroundSize: '200% 100%',
    animation: 'dt-shimmer 1.5s ease-in-out infinite',
    '@keyframes dt-shimmer': {
      from: { backgroundPosition: '200% 0' },
      to: { backgroundPosition: '-200% 0' },
    },
  }),
  bulk: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(0.75, 1.5),
    background: theme.colors.primary.transparent,
    borderBottom: `1px solid ${theme.colors.primary.border}`,
  }),
  bulkRight: css({ marginLeft: 'auto' }),
  footer: css({
    padding: theme.spacing(1, 1.5),
    borderTop: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
  }),
})

function colWidth(c: { width?: number | string; minWidth?: number }): number {
  return typeof c.width === 'number' ? c.width : (c.minWidth ?? 220)
}

export function DataTable<T extends RowData>(props: DataTableProps<T>) {
  const {
    columns,
    data,
    getRowId,
    sort,
    onSortChange,
    clientSort,
    selected,
    onSelectedChange,
    bulkActions,
    rowHref,
    onRowClick,
    rowClassName,
    loading,
    error,
    onRetry,
    empty,
    filtered,
    onClearFilters,
    emptyState,
    virtual,
    fill,
    height,
    footer,
    onOverlayChange,
  } = props
  const density: Density = props.density ?? (props.dense ? 'compact' : 'default')
  const rowHeight = props.rowHeight ?? DENSITY_ROW[density]
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const selectable = !!onSelectedChange

  // Open filter popover (one at a time) + every open row menu, keyed by id. Lifted here so a
  // refetch / re-render of the page never closes them.
  const [openFilter, setOpenFilter] = useState<string | null>(null)
  const [openOverlays, setOpenOverlays] = useState<Set<string>>(() => new Set())
  const registry = useMemo<OverlayRegistry>(
    () => ({
      setOpen: (id, open) =>
        setOpenOverlays((prev) => {
          if (open === prev.has(id)) return prev
          const next = new Set(prev)
          if (open) next.add(id)
          else next.delete(id)
          return next
        }),
    }),
    []
  )
  const anyOverlay = openFilter !== null || openOverlays.size > 0
  const overlayRef = useRef(onOverlayChange)
  overlayRef.current = onOverlayChange
  useEffect(() => {
    overlayRef.current?.(anyOverlay)
  }, [anyOverlay])

  const [localSorting, setLocalSorting] = useState<SortingState>([])
  const sorting: SortingState = useMemo(
    () =>
      clientSort ? localSorting : sort ? [{ id: sort.field, desc: sort.order === 'desc' }] : [],
    [clientSort, localSorting, sort]
  )
  const rowSelection: RowSelectionState = useMemo(
    () =>
      Object.fromEntries(
        Object.entries(selected ?? {})
          .filter(([, v]) => v)
          .map(([k]) => [k, true as const])
      ),
    [selected]
  )
  const selectedIds = useMemo(() => Object.keys(rowSelection), [rowSelection])

  const table = useTable<typeof features, T>({
    features,
    columns: columns as ColumnDef<typeof features, T, unknown>[],
    data,
    getRowId: getRowId ? (row: T) => getRowId(row) : undefined,
    manualSorting: !clientSort,
    enableSortingRemoval: true,
    enableMultiSort: false,
    state: { sorting, rowSelection },
    onSortingChange: (u) => {
      const next = typeof u === 'function' ? u(sorting) : u
      if (clientSort) setLocalSorting(next)
    },
    enableRowSelection: selectable,
    onRowSelectionChange: (u) => {
      const next = typeof u === 'function' ? u(rowSelection) : u
      onSelectedChange?.(next as Record<string, boolean>)
    },
  })

  // Tri-state: neutral → asc → desc → neutral.
  const cycle = useCallback(
    (key: string, current: SortDir) => {
      const next: SortDir = current === undefined ? 'asc' : current === 'asc' ? 'desc' : undefined
      if (clientSort) setLocalSorting(next ? [{ id: key, desc: next === 'desc' }] : [])
      else onSortChange?.(next ? { field: key, order: next } : undefined)
    },
    [clientSort, onSortChange]
  )

  const rows = table.getRowModel().rows
  const scrollRef = useRef<HTMLDivElement>(null)
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => rowHeight,
    overscan: 12,
    enabled: !!virtual,
  })
  const vItems = virtual ? virtualizer.getVirtualItems() : undefined
  const padTop = vItems?.length ? vItems[0].start : 0
  const padBottom = vItems?.length ? virtualizer.getTotalSize() - vItems[vItems.length - 1].end : 0
  const renderRows = vItems ? vItems.map((v) => rows[v.index]) : rows
  const colCount = columns.length + (selectable ? 1 : 0)
  // With `table-layout: fixed`, fixed widths that exceed the container squeeze the flexible
  // columns to zero. Reserve room for them and let the wrapper scroll instead.
  const minTableWidth =
    (selectable ? CHECK_W : 0) +
    columns.reduce(
      (sum, c) => sum + (typeof c.width === 'number' ? c.width : c.width ? 0 : (c.minWidth ?? 220)),
      0
    )

  // Pinned columns. Left: a leading prefix after the checkbox, offset by the widths before it.
  // Right: offset by the pinned columns to its right.
  const pinned = useMemo(() => {
    const offsets = new Map<number, { side: 'left' | 'right'; at: number }>()
    let left = selectable ? CHECK_W : 0
    let lastLeft = -1
    for (let i = 0; i < columns.length && columns[i].sticky === 'left'; i++) {
      offsets.set(i, { side: 'left', at: left })
      left += colWidth(columns[i])
      lastLeft = i
    }
    let right = 0
    let firstRight = columns.length
    for (let i = columns.length - 1; i >= 0 && columns[i].sticky === 'right'; i--) {
      offsets.set(i, { side: 'right', at: right })
      right += colWidth(columns[i])
      firstRight = i
    }
    return { offsets, lastLeft, firstRight }
  }, [columns, selectable])
  const pinClass = (i: number, head: boolean) => {
    const p = pinned.offsets.get(i)
    if (!p) return undefined
    return cx(
      styles.sticky,
      head && styles.stickyTh,
      i === pinned.lastLeft && styles.edgeLeft,
      i === pinned.firstRight && styles.edgeRight
    )
  }
  const pinStyle = (i: number) => {
    const p = pinned.offsets.get(i)
    return p ? { [p.side]: p.at } : undefined
  }
  const checkPin = pinned.offsets.size > 0 && pinned.lastLeft >= 0

  const open = (row: T, e: { metaKey?: boolean; ctrlKey?: boolean }) => {
    const href = rowHref?.(row)
    if (href && (e.metaKey || e.ctrlKey)) {
      window.open(href, '_blank')
      return
    }
    if (onRowClick) onRowClick(row)
    else if (href) {
      window.history.pushState(null, '', href)
      window.dispatchEvent(new PopStateEvent('popstate'))
    }
  }

  let state: ReactNode
  let body: ReactNode = null
  if (error && rows.length === 0) {
    state = <ErrorState error={error} onRetry={onRetry} compact />
  } else if (loading && rows.length === 0) {
    body = Array.from({ length: SKELETON_ROWS }, (_, r) => (
      <tr key={r} aria-hidden>
        {selectable && <td className={cx(styles.td, styles.pad[density], styles.checkCell)} />}
        {columns.map((c, i) => (
          <td key={c.id ?? i} className={cx(styles.td, styles.pad[density])}>
            <div
              className={styles.skeleton}
              style={{
                width: `${55 + ((r * 7 + i * 13) % 40)}%`,
                marginLeft: c.align === 'right' ? 'auto' : undefined,
              }}
            />
          </td>
        ))}
      </tr>
    ))
  } else if (rows.length === 0) {
    state = filtered ? (
      <EmptyState
        variant="not-found"
        message={t('common.empty.filtered')}
        image={<Icon name="search" size="xxxl" className={styles.stateIcon} />}
        button={
          onClearFilters ? (
            <Button variant="secondary" icon="times" onClick={onClearFilters}>
              {t('common.empty.clearFilters')}
            </Button>
          ) : undefined
        }
      />
    ) : empty ? (
      <EmptyState
        variant="call-to-action"
        message={empty.message}
        image={<Icon name="folder-open" size="xxxl" className={styles.stateIcon} />}
        button={empty.button}
      >
        {empty.hint}
      </EmptyState>
    ) : (
      (emptyState ?? (
        <EmptyState
          variant="not-found"
          message={t('common.empty.title')}
          image={<Icon name="folder-open" size="xxxl" className={styles.stateIcon} />}
        />
      ))
    )
  } else {
    body = (
      <>
        {padTop > 0 && (
          <tr>
            <td colSpan={colCount} style={{ height: padTop, padding: 0, border: 0 }} />
          </tr>
        )}
        {renderRows.map((row) => {
          const clickable = !!rowHref?.(row.original) || !!onRowClick
          return (
            <tr
              key={row.id}
              className={cx(
                styles.tr,
                clickable && styles.trClickable,
                row.getIsSelected() && styles.trSelected,
                rowClassName?.(row.original)
              )}
              style={virtual ? { height: rowHeight } : undefined}
              tabIndex={clickable ? 0 : undefined}
              onClick={(e) => {
                if ((e.target as HTMLElement).closest('a,button,input,label,[data-no-row-click]'))
                  return
                if (clickable) open(row.original, e)
              }}
              onKeyDown={(e) => {
                if (e.key !== 'Enter' || e.target !== e.currentTarget || !clickable) return
                e.preventDefault()
                open(row.original, e)
              }}
            >
              {selectable && (
                <td
                  className={cx(styles.td, styles.checkCell, checkPin && styles.sticky)}
                  style={checkPin ? { left: 0 } : undefined}
                  data-sticky={checkPin ? 'left' : undefined}
                >
                  <Checkbox
                    value={row.getIsSelected()}
                    onChange={row.getToggleSelectedHandler()}
                    aria-label={t('common.actions.select')}
                  />
                </td>
              )}
              {row.getAllCells().map((cell, ci) => {
                const def = cell.column.columnDef as DataTableColumn<T>
                return (
                  <td
                    key={cell.id}
                    className={cx(
                      styles.td,
                      styles.pad[density],
                      def.align === 'right' && styles.right,
                      def.align === 'center' && styles.center,
                      def.mono && styles.mono,
                      def.tight && styles.tight,
                      pinClass(ci, false)
                    )}
                    style={pinStyle(ci)}
                    data-sticky={pinned.offsets.get(ci)?.side}
                  >
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                )
              })}
            </tr>
          )
        })}
        {padBottom > 0 && (
          <tr>
            <td colSpan={colCount} style={{ height: padBottom, padding: 0, border: 0 }} />
          </tr>
        )}
      </>
    )
  }

  return (
    <OverlayContext.Provider value={registry}>
      <DensityContext.Provider value={density}>
        <section className={cx(styles.wrap, fill && styles.fill)} aria-label={props['aria-label']}>
          {bulkActions && selectedIds.length > 0 && (
            <div className={styles.bulk} role="toolbar" aria-label={t('common.table.bulk')}>
              <Text weight="medium">
                {t('common.table.selected', { count: selectedIds.length })}
              </Text>
              {bulkActions(selectedIds)}
              <span className={styles.bulkRight}>
                <Button
                  size="sm"
                  variant="secondary"
                  fill="text"
                  icon="times"
                  onClick={() => onSelectedChange?.({})}
                >
                  {t('common.table.clearSelection')}
                </Button>
              </span>
            </div>
          )}
          <div
            ref={scrollRef}
            className={cx(styles.scroll, fill && styles.fillScroll)}
            style={fill ? undefined : { maxHeight: height }}
          >
            <table
              className={styles.table}
              style={{ minWidth: minTableWidth }}
              aria-busy={loading || undefined}
            >
              <colgroup>
                {selectable && <col style={{ width: CHECK_W }} />}
                {columns.map((c, i) => (
                  <col key={c.id ?? i} style={{ width: c.width }} />
                ))}
              </colgroup>
              <thead>
                {table.getHeaderGroups().map((hg) => (
                  <tr key={hg.id}>
                    {selectable && (
                      <th
                        className={cx(
                          styles.th,
                          styles.checkCell,
                          checkPin && cx(styles.sticky, styles.stickyTh)
                        )}
                        style={checkPin ? { left: 0 } : undefined}
                      >
                        <Checkbox
                          value={table.getIsAllRowsSelected()}
                          indeterminate={table.getIsSomeRowsSelected()}
                          onChange={table.getToggleAllRowsSelectedHandler()}
                          aria-label={t('common.table.selectAll')}
                        />
                      </th>
                    )}
                    {hg.headers.map((h, hi) => {
                      const def = h.column.columnDef as DataTableColumn<T>
                      const sortKey = clientSort ? h.column.id : def.sortKey
                      const canSort = clientSort
                        ? h.column.getCanSort() && h.column.accessorFn !== undefined
                        : !!def.sortKey && !!onSortChange
                      const optionActive =
                        !clientSort && sort && def.sortOptions?.some((o) => o.key === sort.field)
                      const dir: SortDir = clientSort
                        ? h.column.getIsSorted() || undefined
                        : sort && ((def.sortKey && sort.field === def.sortKey) || optionActive)
                          ? sort.order
                          : undefined
                      const label = h.isPlaceholder
                        ? null
                        : flexRender(h.column.columnDef.header, h.getContext())
                      const bare = !canSort && !def.filter && !def.sortOptions
                      const titleText =
                        def.title ??
                        (typeof h.column.columnDef.header === 'string'
                          ? h.column.columnDef.header
                          : undefined)
                      return (
                        <th
                          key={h.id}
                          scope="col"
                          className={cx(
                            styles.th,
                            def.align === 'right' && styles.right,
                            def.align === 'center' && styles.center,
                            def.tight && styles.tight,
                            pinClass(hi, true)
                          )}
                          style={pinStyle(hi)}
                          aria-label={titleText || undefined}
                          title={!label && titleText ? titleText : undefined}
                          aria-sort={dir ? (dir === 'asc' ? 'ascending' : 'descending') : undefined}
                        >
                          {bare && !label ? null : (
                            <ColumnHeader
                              label={label || null}
                              name={titleText}
                              align={def.align}
                              canSort={canSort}
                              sortDir={dir}
                              onSort={sortKey ? () => cycle(sortKey, dir) : undefined}
                              sortOptions={
                                !clientSort && onSortChange ? def.sortOptions : undefined
                              }
                              activeSortKey={optionActive ? sort?.field : undefined}
                              onSortSelect={(key, d) =>
                                onSortChange?.(key && d ? { field: key, order: d } : undefined)
                              }
                              filter={def.filter}
                              open={openFilter === h.column.id}
                              onOpenChange={(o) =>
                                setOpenFilter((cur) =>
                                  o ? h.column.id : cur === h.column.id ? null : cur
                                )
                              }
                            />
                          )}
                        </th>
                      )
                    })}
                  </tr>
                ))}
              </thead>
              <tbody>{body}</tbody>
            </table>
          </div>
          {state && <div className={styles.state}>{state}</div>}
          {footer && <div className={styles.footer}>{footer}</div>}
        </section>
      </DensityContext.Provider>
    </OverlayContext.Provider>
  )
}
