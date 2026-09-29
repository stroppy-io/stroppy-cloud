import type { TenantDashboard } from '@api/types'
import { AppLink } from '@app/AppLink'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, useStyles2 } from '@grafana/ui'
import { formatPercent } from '@helpers/format'
import type { StatusTone } from '@helpers/run-status'
import type { LinkProps } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(6, minmax(0, 1fr))',
    gap: theme.spacing(1.5),
    [theme.breakpoints.down('lg')]: { gridTemplateColumns: 'repeat(3, minmax(0, 1fr))' },
    [theme.breakpoints.down('sm')]: { gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' },
  }),
  tile: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
    padding: theme.spacing(1.5, 2),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    minWidth: 0,
    color: 'inherit',
    textDecoration: 'none',
    '&:hover': { borderColor: theme.colors.border.medium, color: 'inherit' },
  }),
  label: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.75),
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  value: css({
    fontSize: theme.typography.h2.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: 1.1,
    fontVariantNumeric: 'tabular-nums',
  }),
  sub: css({ fontSize: theme.typography.bodySmall.fontSize, color: theme.colors.text.secondary }),
  tone: (tone: StatusTone) =>
    css({
      color: tone === 'secondary' ? theme.colors.text.primary : theme.colors[tone].text,
    }),
  pulse: css({
    animation: 'spin 2s linear infinite',
    '@keyframes spin': { to: { transform: 'rotate(360deg)' } },
  }),
})

interface Tile {
  key: string
  label: string
  value: ReactNode
  sub?: ReactNode
  tone: StatusTone
  icon: 'sync' | 'clock-nine' | 'check-circle' | 'exclamation-circle' | 'percentage' | 'lock'
  spin?: boolean
  to: LinkProps['to']
  search?: Record<string, unknown>
}

export function StatTiles({ data, slug }: { data: TenantDashboard; slug: string }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const c = data.run_counts
  const hasWeek = (c.completed ?? 0) + (c.failed ?? 0) > 0
  const tiles: Tile[] = [
    {
      key: 'running',
      label: t('dashboard.tiles.running'),
      value: c.running ?? 0,
      tone: (c.running ?? 0) > 0 ? 'info' : 'secondary',
      icon: 'sync',
      spin: (c.running ?? 0) > 0,
      to: '/t/$slug/runs',
      search: { status: ['running'] },
    },
    {
      key: 'pending',
      label: t('dashboard.tiles.pending'),
      value: c.pending ?? 0,
      tone: 'secondary',
      icon: 'clock-nine',
      to: '/t/$slug/runs',
      search: { status: ['pending'] },
    },
    {
      key: 'completed',
      label: t('dashboard.tiles.completed7d'),
      value: c.completed ?? 0,
      tone: 'success',
      icon: 'check-circle',
      to: '/t/$slug/runs',
      search: { status: ['completed'] },
    },
    {
      key: 'failed',
      label: t('dashboard.tiles.failed7d'),
      value: c.failed ?? 0,
      tone: (c.failed ?? 0) > 0 ? 'error' : 'secondary',
      icon: 'exclamation-circle',
      to: '/t/$slug/runs',
      search: { status: ['failed'] },
    },
    {
      key: 'rate',
      label: t('dashboard.tiles.successRate'),
      value: hasWeek ? formatPercent(data.success_rate, 0) : '—',
      sub: hasWeek ? undefined : t('dashboard.tiles.noData'),
      tone: !hasWeek
        ? 'secondary'
        : (data.success_rate ?? 0) >= 90
          ? 'success'
          : (data.success_rate ?? 0) >= 70
            ? 'warning'
            : 'error',
      icon: 'percentage',
      to: '/t/$slug/runs',
    },
    {
      key: 'kept',
      label: t('dashboard.tiles.kept'),
      value: c.kept_stands ?? 0,
      tone: (c.kept_stands ?? 0) > 0 ? 'warning' : 'secondary',
      icon: 'lock',
      to: '/t/$slug/runs',
      search: { kept: true },
    },
  ]
  return (
    <div className={styles.grid}>
      {tiles.map((tile) => (
        <AppLink
          key={tile.key}
          to={tile.to}
          params={{ slug }}
          search={tile.search as never}
          plain
          className={styles.tile}
        >
          <span className={styles.label}>
            <Icon name={tile.icon} size="sm" className={tile.spin ? styles.pulse : undefined} />
            {tile.label}
          </span>
          <span className={cx(styles.value, styles.tone(tile.tone))}>{tile.value}</span>
          {tile.sub && <span className={styles.sub}>{tile.sub}</span>}
        </AppLink>
      ))}
    </div>
  )
}
