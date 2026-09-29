import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import type { ReactNode } from 'react'

// One scroll container per screen (web/AGENTS.md §18): a `fill` page is exactly as tall as the
// shell's content area; its header / tabs / toolbar stay put and a single `PageFill` (or a
// `DataTable fill`) takes the rest and scrolls on its own. The page itself never scrolls.
const getStyles = (theme: GrafanaTheme2) => ({
  page: css({
    width: '100%',
    maxWidth: 1440,
    margin: '0 auto',
    padding: theme.spacing(2, 3, 6),
    [theme.breakpoints.down('md')]: { padding: theme.spacing(1.5, 2, 4) },
  }),
  wide: css({ maxWidth: 'none' }),
  narrow: css({ maxWidth: 880 }),
  fill: css({
    height: '100%',
    minHeight: 0,
    display: 'flex',
    flexDirection: 'column',
    paddingBottom: theme.spacing(2),
    [theme.breakpoints.down('md')]: { paddingBottom: theme.spacing(1.5) },
  }),
  area: css({
    flex: '1 1 auto',
    minHeight: 0,
    minWidth: 0,
    display: 'flex',
    flexDirection: 'column',
  }),
  scroll: css({ overflow: 'auto', overscrollBehavior: 'contain' }),
})

export function Page({
  children,
  width = 'default',
  fill,
  className,
}: {
  children: ReactNode
  width?: 'default' | 'wide' | 'narrow'
  // Viewport-height page: children are laid out in a column and one of them fills the rest.
  fill?: boolean
  className?: string
}) {
  const styles = useStyles2(getStyles)
  return (
    <div
      className={cx(
        styles.page,
        width === 'wide' && styles.wide,
        width === 'narrow' && styles.narrow,
        fill && styles.fill,
        className
      )}
    >
      {children}
    </div>
  )
}

// The part of a `fill` page that takes the remaining height. `scroll` makes it the page's one
// scroll container (tab bodies with ordinary content); without it the child fills and scrolls
// itself (a table, a log view, a canvas).
export function PageFill({
  children,
  scroll,
  className,
}: {
  children: ReactNode
  scroll?: boolean
  className?: string
}) {
  const styles = useStyles2(getStyles)
  return <div className={cx(styles.area, scroll && styles.scroll, className)}>{children}</div>
}
