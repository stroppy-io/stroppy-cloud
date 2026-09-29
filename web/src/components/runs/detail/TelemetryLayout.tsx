import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import type { ReactNode } from 'react'

const getStyles = (theme: GrafanaTheme2) => ({
  // Fills the run page's tab body (a flex column); below MIN the tab body scrolls instead.
  root: css({
    flex: '1 1 auto',
    minHeight: 320,
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1),
    minWidth: 0,
  }),
  toolbar: css({ flex: 'none', minWidth: 0 }),
  body: css({
    flex: '1 1 auto',
    minHeight: 0,
    minWidth: 0,
    display: 'flex',
    flexDirection: 'column',
  }),
})

// Telemetry tab frame: the toolbar stays at the top, the body takes the rest of the run page
// and scrolls on its own — neither the tab body nor the page scrolls (web/AGENTS.md §18).
export function TelemetryLayout({
  toolbar,
  children,
}: {
  toolbar: ReactNode
  children: ReactNode
}) {
  const styles = useStyles2(getStyles)
  return (
    <div className={styles.root}>
      <div className={styles.toolbar}>{toolbar}</div>
      <div className={styles.body}>{children}</div>
    </div>
  )
}
