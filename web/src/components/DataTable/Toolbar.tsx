import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Button,
  Checkbox,
  FilterInput,
  FilterPill,
  RefreshPicker,
  Select,
  Text,
  useStyles2,
} from '@grafana/ui'
import { type ReactNode, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { PAGE_SIZES, type PageSize, REFRESH_INTERVALS, type RefreshInterval } from './list-search'

const getStyles = (theme: GrafanaTheme2) => ({
  bar: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    flexWrap: 'wrap',
    marginBottom: theme.spacing(1.5),
  }),
  toggles: css({ display: 'flex', gap: theme.spacing(1.5), alignItems: 'center' }),
  pills: css({ display: 'flex', gap: theme.spacing(0.5), flexWrap: 'wrap', alignItems: 'center' }),
  right: css({ marginLeft: 'auto', display: 'flex', gap: theme.spacing(1), alignItems: 'center' }),
  search: css({ width: 280 }),
})

export interface ActivePill {
  key: string
  label: string
  onRemove: () => void
}

export interface ToolbarToggle {
  key: string
  label: string
  value: boolean
  onChange: (next: boolean) => void
}

export interface ToolbarRefresh {
  interval: RefreshInterval
  onIntervalChange: (next: RefreshInterval) => void
  onRefresh: () => void
  isFetching?: boolean
  // While an overlay is open the interval is paused; show it.
  paused?: boolean
}

// Debounced search box; the URL is written 300 ms after the last keystroke.
function SearchBox({
  value,
  onChange,
  placeholder,
}: {
  value: string
  onChange: (v: string) => void
  placeholder: string
}) {
  const styles = useStyles2(getStyles)
  const [local, setLocal] = useState(value)
  useEffect(() => setLocal(value), [value])
  useEffect(() => {
    if (local === value) return
    const id = setTimeout(() => onChange(local), 300)
    return () => clearTimeout(id)
  }, [local, value, onChange])
  return (
    <div className={styles.search}>
      <FilterInput value={local} onChange={setLocal} placeholder={placeholder} />
    </div>
  )
}

// Search + boolean toggles + active-filter pills + refresh controls + row count. The URL owns
// every value shown here; this component only renders and forwards changes.
export function DataTableToolbar({
  search,
  onSearch,
  searchPlaceholder,
  toggles,
  pills,
  onClearAll,
  left,
  right,
  count,
  size,
  onSizeChange,
  refresh,
  settings,
}: {
  search?: string
  onSearch?: (v: string) => void
  searchPlaceholder?: string
  toggles?: ToolbarToggle[]
  pills?: ActivePill[]
  onClearAll?: () => void
  left?: ReactNode
  right?: ReactNode
  count?: number
  size?: PageSize
  onSizeChange?: (size: PageSize) => void
  refresh?: ToolbarRefresh
  // Right-most slot: the table view menu (`<TableSettings prefs={…} />`).
  settings?: ReactNode
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  return (
    <div className={styles.bar}>
      {onSearch && (
        <SearchBox
          value={search ?? ''}
          onChange={(v) => onSearch(v.trim())}
          placeholder={searchPlaceholder ?? t('common.actions.search')}
        />
      )}
      {toggles && toggles.length > 0 && (
        <div className={styles.toggles}>
          {toggles.map((tg) => (
            <Checkbox
              key={tg.key}
              label={tg.label}
              value={tg.value}
              onChange={(e) => tg.onChange(e.currentTarget.checked)}
            />
          ))}
        </div>
      )}
      {left}
      {pills && pills.length > 0 && (
        <div className={styles.pills}>
          {pills.map((p) => (
            <FilterPill key={p.key} selected label={p.label} icon="times" onClick={p.onRemove} />
          ))}
          {onClearAll && (
            <Button size="sm" variant="secondary" fill="text" onClick={onClearAll}>
              {t('common.empty.clearFilters')}
            </Button>
          )}
        </div>
      )}
      <div className={styles.right}>
        {right}
        {count !== undefined && (
          <Text color="secondary" variant="bodySmall">
            {t('common.misc.showing', { count })}
          </Text>
        )}
        {size !== undefined && onSizeChange && (
          <Select<number>
            width={12}
            options={PAGE_SIZES.map((s) => ({
              label: t('common.table.perPage', { count: s }),
              value: s,
            }))}
            value={size}
            onChange={(o) => o.value !== undefined && onSizeChange(o.value as PageSize)}
            aria-label={t('common.table.pageSize')}
          />
        )}
        {refresh && (
          <RefreshPicker
            value={refresh.interval === 'off' ? '' : refresh.interval}
            intervals={REFRESH_INTERVALS.filter((i) => i !== 'off')}
            onIntervalChanged={(v) => refresh.onIntervalChange((v || 'off') as RefreshInterval)}
            onRefresh={refresh.onRefresh}
            isLoading={refresh.isFetching}
            tooltip={
              refresh.paused && refresh.interval !== 'off'
                ? t('common.table.refreshPaused')
                : t('common.actions.refresh')
            }
          />
        )}
        {settings}
      </div>
    </div>
  )
}
