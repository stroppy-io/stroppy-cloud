import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, FilterPill, RefreshPicker, TimeRangePicker, useStyles2 } from '@grafana/ui'
import { REFRESH_INTERVALS } from '@helpers/time-range'
import type { TelemetryRange } from '@hooks/useTelemetryRange'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1),
    minWidth: 0,
  }),
  // One 32px-tall row; wraps on narrow width, every control keeps the same height.
  row: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    flexWrap: 'wrap',
    minWidth: 0,
    '& > *': { minHeight: theme.spacing(4) },
  }),
  left: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    flexWrap: 'wrap',
    minWidth: 0,
    flex: '1 1 auto',
  }),
  right: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    flexWrap: 'wrap',
    marginLeft: 'auto',
  }),
  pills: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    flexWrap: 'wrap',
  }),
})

export interface ActiveFilter {
  key: string
  label: string
  onRemove: () => void
}

export interface TelemetryToolbarProps {
  range: TelemetryRange
  // Whether the tab is fetching right now (RefreshPicker spinner).
  isLoading?: boolean
  onRefresh?: () => void
  // Tab-specific filters, left of the time controls.
  children?: ReactNode
  // Tab-specific view toggles, right of the time controls.
  actions?: ReactNode
  // Active filters as removable chips (second row, only when any).
  filters?: ActiveFilter[]
  onResetFilters?: () => void
}

// Grafana-Explore style toolbar shared by the Logs / Metrics / Events tabs: tab filters,
// `TimeRangePicker` and `RefreshPicker` driven by the URL, tab actions, active-filter chips.
export function TelemetryToolbar({
  range,
  isLoading,
  onRefresh,
  children,
  actions,
  filters,
  onResetFilters,
}: TelemetryToolbarProps) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  return (
    <div className={styles.root}>
      <div className={styles.row}>
        {children && <div className={styles.left}>{children}</div>}
        <div className={styles.right}>
          <TimeRangePicker
            value={range.range}
            timeZone="browser"
            quickRanges={range.quickRanges}
            onChange={range.setRange}
            onChangeTimeZone={() => undefined}
            onZoom={range.handleZoom}
            onMoveBackward={range.handleMoveBackward}
            onMoveForward={range.handleMoveForward}
            isOnCanvas
          />
          <RefreshPicker
            value={range.refresh}
            intervals={[...REFRESH_INTERVALS]}
            noIntervalPicker={range.terminal}
            isLoading={isLoading}
            onRefresh={onRefresh}
            onIntervalChanged={range.setRefresh}
            tooltip={range.terminal ? t('common.actions.refresh') : t('runs.telemetry.refresh')}
            isOnCanvas
          />
          {actions}
        </div>
      </div>
      {filters && filters.length > 0 && (
        <div className={styles.pills}>
          {filters.map((f) => (
            <FilterPill key={f.key} selected label={f.label} icon="times" onClick={f.onRemove} />
          ))}
          {onResetFilters && (
            <Button size="sm" variant="secondary" fill="text" onClick={onResetFilters}>
              {t('common.actions.reset')}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
