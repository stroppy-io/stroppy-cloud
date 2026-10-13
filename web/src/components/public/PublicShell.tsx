import { PublicHeader } from '@components/landing/PublicHeader'
import { css } from '@emotion/css'
import { type GrafanaTheme2, ThemeContext } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import type { ReactNode } from 'react'
import { brand } from '@/styles/brand'
import { getAppTheme } from '@/styles/theme'

// The frame of the pages a visitor sees outside the console — the welcome
// screen, the first tenant: the landing's header and dark, one scrolling area.

const DARK = getAppTheme('dark')

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    height: '100vh',
    display: 'flex',
    flexDirection: 'column',
    background: brand.ink,
    color: brand.text,
  }),
  main: css({ flex: 1, minHeight: 0, overflowY: 'auto' }),
  inner: css({
    maxWidth: 1040,
    margin: '0 auto',
    padding: theme.spacing(6, 4, 8),
    [theme.breakpoints.down('md')]: { padding: theme.spacing(3, 2, 6) },
  }),
})

function Frame({ children }: { children: ReactNode }) {
  const styles = useStyles2(getStyles)
  return (
    <div className={styles.root}>
      <PublicHeader />
      <main className={styles.main}>
        <div className={styles.inner}>{children}</div>
      </main>
    </div>
  )
}

export function PublicShell({ children }: { children: ReactNode }) {
  return (
    <ThemeContext.Provider value={DARK}>
      <Frame>{children}</Frame>
    </ThemeContext.Provider>
  )
}
