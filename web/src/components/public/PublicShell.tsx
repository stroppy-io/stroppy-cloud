import { SiteBar } from '@components/public/SiteBar'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import type { ReactNode } from 'react'
import { brandOf } from '@/styles/brand'

// The frame of the pages a visitor sees outside the console — the welcome
// screen, the first tenant: the site bar, the kit palette, one scrolling area.

const getStyles = (theme: GrafanaTheme2) => {
  const brand = brandOf(theme)
  return {
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
  }
}

function Frame({ children }: { children: ReactNode }) {
  const styles = useStyles2(getStyles)
  return (
    <div className={styles.root}>
      <SiteBar />
      <main className={styles.main}>
        <div className={styles.inner}>{children}</div>
      </main>
    </div>
  )
}

export function PublicShell({ children }: { children: ReactNode }) {
  return <Frame>{children}</Frame>
}
