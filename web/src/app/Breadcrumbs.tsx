import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, useStyles2 } from '@grafana/ui'
import { Link } from '@tanstack/react-router'
import { useBreadcrumbs } from './crumbs'

const getStyles = (theme: GrafanaTheme2) => ({
  nav: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    minWidth: 0,
    overflow: 'hidden',
    whiteSpace: 'nowrap',
  }),
  link: css({
    color: theme.colors.text.secondary,
    textDecoration: 'none',
    padding: theme.spacing(0.25, 0.5),
    borderRadius: theme.shape.radius.default,
    maxWidth: 260,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    '&:hover': { color: theme.colors.text.primary, background: theme.colors.action.hover },
  }),
  current: css({
    color: theme.colors.text.primary,
    fontWeight: theme.typography.fontWeightMedium,
    padding: theme.spacing(0.25, 0.5),
    maxWidth: 360,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  sep: css({ color: theme.colors.text.disabled, display: 'inline-flex' }),
})

// Global breadcrumb bar fed by the router (see crumbs.ts). Pages never render their own.
export function Breadcrumbs() {
  const styles = useStyles2(getStyles)
  const crumbs = useBreadcrumbs()
  if (crumbs.length < 2) return null
  return (
    <nav className={styles.nav} aria-label="breadcrumb">
      {crumbs.map((c, i) => (
        <span key={c.key} style={{ display: 'contents' }}>
          {i > 0 && (
            <span className={styles.sep}>
              <Icon name="angle-right" size="sm" />
            </span>
          )}
          {c.current ? (
            <span className={styles.current} aria-current="page">
              {c.label}
            </span>
          ) : (
            <Link to={c.href as never} className={styles.link}>
              {c.label}
            </Link>
          )}
        </span>
      ))}
    </nav>
  )
}
