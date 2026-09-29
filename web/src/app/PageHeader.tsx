import { css, cx } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import { Icon, Stack, Text, useStyles2 } from '@grafana/ui'
import type { LinkProps } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { AppLink } from './AppLink'

export interface Crumb {
  label: ReactNode
  to?: LinkProps['to']
  params?: LinkProps['params']
}

export interface TabDef {
  id: string
  label: ReactNode
  icon?: IconName
  to: LinkProps['to']
  params?: LinkProps['params']
  search?: LinkProps['search']
  counter?: number
  disabled?: boolean
}

const getStyles = (theme: GrafanaTheme2) => ({
  header: css({ marginBottom: theme.spacing(2) }),
  crumbs: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    marginBottom: theme.spacing(0.5),
    flexWrap: 'wrap',
  }),
  titleRow: css({
    display: 'flex',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
  }),
  title: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    minWidth: 0,
    flexWrap: 'wrap',
  }),
  h1: css({
    margin: 0,
    fontSize: theme.typography.h2.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: 1.3,
    wordBreak: 'break-word',
  }),
  subtitle: css({ color: theme.colors.text.secondary, marginTop: theme.spacing(0.5) }),
  actions: css({ display: 'flex', gap: theme.spacing(1), alignItems: 'center', flexWrap: 'wrap' }),
  tabs: css({
    display: 'flex',
    gap: theme.spacing(0.5),
    marginTop: theme.spacing(2),
    // The baseline is an inset shadow, not a border: the active tab's underline paints over it
    // without a negative margin, so the bar never overflows vertically (no stray scrollbar).
    boxShadow: `inset 0 -1px 0 ${theme.colors.border.weak}`,
    overflowX: 'auto',
    overflowY: 'hidden',
  }),
  tab: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.75),
    padding: theme.spacing(1, 1.5),
    borderBottom: '2px solid transparent',
    color: theme.colors.text.secondary,
    whiteSpace: 'nowrap',
    fontSize: theme.typography.body.fontSize,
    '&:hover': {
      color: theme.colors.text.primary,
      background: theme.colors.action.hover,
      textDecoration: 'none',
      borderRadius: `${theme.shape.radius.default} ${theme.shape.radius.default} 0 0`,
    },
    '&[data-status="active"]': {
      color: theme.colors.text.primary,
      borderBottomColor: theme.colors.primary.main,
      fontWeight: theme.typography.fontWeightMedium,
    },
  }),
  tabDisabled: css({ opacity: 0.5, pointerEvents: 'none' }),
  counter: css({
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: 999,
    padding: '0 6px',
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    lineHeight: '18px',
  }),
})

// GitHub-style page header: breadcrumbs, title + badges, actions on the right, tabs below.
export function PageHeader({
  title,
  subtitle,
  breadcrumbs: _breadcrumbs,
  badge,
  actions,
  tabs,
  icon,
}: {
  title: ReactNode
  subtitle?: ReactNode
  breadcrumbs?: Crumb[]
  badge?: ReactNode
  actions?: ReactNode
  tabs?: TabDef[]
  icon?: IconName
}) {
  const styles = useStyles2(getStyles)
  return (
    <header className={styles.header}>
      <div className={styles.titleRow}>
        <div style={{ minWidth: 0, flex: 1 }}>
          <div className={styles.title}>
            {icon && <Icon name={icon} size="xl" />}
            <h1 className={styles.h1}>{title}</h1>
            {badge}
          </div>
          {subtitle && <div className={styles.subtitle}>{subtitle}</div>}
        </div>
        {actions && <div className={styles.actions}>{actions}</div>}
      </div>
      {tabs && tabs.length > 0 && (
        <nav className={styles.tabs}>
          {tabs.map((t) => (
            <AppLink
              key={t.id}
              to={t.to}
              params={t.params}
              search={t.search}
              plain
              className={cx(styles.tab, t.disabled && styles.tabDisabled)}
              activeOptions={{
                exact: t.id === 'overview' || t.id === 'index',
                includeSearch: false,
              }}
            >
              {t.icon && <Icon name={t.icon} size="sm" />}
              {t.label}
              {t.counter !== undefined && <span className={styles.counter}>{t.counter}</span>}
            </AppLink>
          ))}
        </nav>
      )}
    </header>
  )
}

export function SectionTitle({ children, right }: { children: ReactNode; right?: ReactNode }) {
  return (
    <Stack justifyContent="space-between" alignItems="center">
      <Text element="h2" variant="h4">
        {children ?? ''}
      </Text>
      {right}
    </Stack>
  )
}
