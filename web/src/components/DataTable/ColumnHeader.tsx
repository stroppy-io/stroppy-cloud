import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Dropdown, Icon, Menu, Toggletip, useStyles2 } from '@grafana/ui'
import { Fragment, type ReactNode, useId } from 'react'
import { useTranslation } from 'react-i18next'
import { ChecklistFilter, DateRangeFilter, NumberRangeFilter, TextFilter } from './filters'
import { useOverlayRegistry } from './overlay'
import { type ColumnFilter, isFilterActive } from './types'

export type SortDir = 'asc' | 'desc' | undefined

// A composite column (several numbers in one cell) sorts by one of several server keys.
export interface SortOption {
  key: string
  label: string
}

// Slot width reserved on both sides of the label so labels line up across columns whether or
// not the column has a sort / filter control.
const SLOT = 20

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    minWidth: 0,
    width: '100%',
  }),
  right: css({ justifyContent: 'flex-end' }),
  center: css({ justifyContent: 'center' }),
  label: css({
    minWidth: 0,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  }),
  slot: css({
    width: SLOT,
    height: SLOT,
    flex: `0 0 ${SLOT}px`,
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
  }),
  btn: css({
    all: 'unset',
    boxSizing: 'border-box',
    width: SLOT,
    height: SLOT,
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: theme.shape.radius.default,
    color: theme.colors.text.disabled,
    cursor: 'pointer',
    '&:hover': { color: theme.colors.text.primary, background: theme.colors.action.hover },
    '&:focus-visible': { outline: `2px solid ${theme.colors.primary.border}` },
  }),
  active: css({
    color: theme.colors.primary.text,
    '&:hover': { color: theme.colors.primary.text },
  }),
  open: css({ background: theme.colors.action.selected }),
  popover: css({ padding: theme.spacing(0.5, 0) }),
  sortKey: css({
    color: theme.colors.primary.text,
    fontWeight: theme.typography.fontWeightRegular,
  }),
})

function FilterBody({ filter }: { filter: ColumnFilter }) {
  switch (filter.kind) {
    case 'checklist':
      return <ChecklistFilter filter={filter} />
    case 'text':
      return <TextFilter filter={filter} />
    case 'date':
      return <DateRangeFilter filter={filter} />
    case 'number':
      return <NumberRangeFilter filter={filter} />
  }
}

// Three-slot header `[sort] Label [filter]`. Sort is tri-state (neutral → asc → desc → neutral);
// the filter opens a controlled popover whose open state the table owns (keyed by column id).
export function ColumnHeader({
  label,
  name,
  align,
  canSort,
  sortDir,
  onSort,
  sortOptions,
  activeSortKey,
  onSortSelect,
  filter,
  open,
  onOpenChange,
}: {
  label: ReactNode
  // Column name for tooltips and accessible labels when `label` is an icon or empty.
  name?: string
  align?: 'left' | 'right' | 'center'
  canSort: boolean
  sortDir: SortDir
  onSort?: () => void
  // Composite column: the sort button opens a menu of keys instead of cycling one key.
  sortOptions?: SortOption[]
  activeSortKey?: string
  onSortSelect?: (key: string | undefined, dir: SortDir) => void
  filter?: ColumnFilter
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const active = isFilterActive(filter)
  const text = typeof label === 'string' && label ? label : name
  const registry = useOverlayRegistry()
  const menuId = useId()
  const activeOption = sortOptions?.find((o) => o.key === activeSortKey)
  const sortIcon =
    sortDir === 'asc' ? 'arrow-up' : sortDir === 'desc' ? 'arrow-down' : 'sort-amount-down'
  return (
    <div
      className={cx(
        styles.root,
        align === 'right' && styles.right,
        align === 'center' && styles.center
      )}
    >
      {sortOptions && onSortSelect ? (
        <Dropdown
          placement="bottom-start"
          onVisibleChange={(v) => registry?.setOpen(menuId, v)}
          overlay={
            <Menu>
              {sortOptions.map((o) => (
                <Fragment key={o.key}>
                  {(['desc', 'asc'] as const).map((d) => (
                    <Menu.Item
                      key={d}
                      label={`${o.label} ${d === 'asc' ? '↑' : '↓'}`}
                      ariaLabel={`${o.label}, ${d === 'asc' ? t('common.table.ascending') : t('common.table.descending')}`}
                      icon={activeSortKey === o.key && sortDir === d ? 'check' : undefined}
                      onClick={() => onSortSelect(o.key, d)}
                    />
                  ))}
                </Fragment>
              ))}
              <Menu.Divider />
              <Menu.Item
                label={t('common.table.resetSort')}
                icon="history"
                disabled={!activeOption}
                onClick={() => onSortSelect(undefined, undefined)}
              />
            </Menu>
          }
        >
          <button
            type="button"
            className={cx(styles.btn, activeOption && styles.active)}
            onClick={(e) => e.stopPropagation()}
            aria-label={t('common.table.sortBy', { column: text ?? '' })}
            title={
              activeOption
                ? `${activeOption.label} ${sortDir === 'asc' ? '↑' : '↓'}`
                : t('common.table.sortBy', { column: text ?? '' })
            }
          >
            <Icon name={activeOption ? sortIcon : 'sort-amount-down'} size="sm" />
          </button>
        </Dropdown>
      ) : canSort ? (
        <button
          type="button"
          className={cx(styles.btn, sortDir && styles.active)}
          onClick={(e) => {
            e.stopPropagation()
            onSort?.()
          }}
          aria-label={t('common.table.sortBy', { column: text ?? '' })}
          title={
            sortDir === 'asc'
              ? t('common.table.sortedAsc')
              : sortDir === 'desc'
                ? t('common.table.sortedDesc')
                : t('common.table.sortBy', { column: text ?? '' })
          }
        >
          <Icon
            name={
              sortDir === 'asc'
                ? 'arrow-up'
                : sortDir === 'desc'
                  ? 'arrow-down'
                  : 'sort-amount-down'
            }
            size="sm"
          />
        </button>
      ) : (
        <span className={styles.slot} aria-hidden />
      )}
      <span className={styles.label} title={text}>
        {label}
        {activeOption && <span className={styles.sortKey}> · {activeOption.label}</span>}
      </span>
      {filter ? (
        <Toggletip
          show={open}
          onOpen={() => onOpenChange(true)}
          onClose={() => onOpenChange(false)}
          placement="bottom"
          title={text}
          fitContent
          content={
            <div className={styles.popover}>
              <FilterBody filter={filter} />
            </div>
          }
        >
          <button
            type="button"
            className={cx(styles.btn, active && styles.active, open && styles.open)}
            onClick={(e) => e.stopPropagation()}
            aria-label={t('common.table.filterBy', { column: text ?? '' })}
            title={
              active
                ? t('common.table.filterActive', { column: text ?? '' })
                : t('common.table.filterBy', { column: text ?? '' })
            }
          >
            <Icon name="filter" size="sm" type={active ? 'solid' : 'default'} />
          </button>
        </Toggletip>
      ) : (
        <span className={styles.slot} aria-hidden />
      )}
    </div>
  )
}
