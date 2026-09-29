import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Text, useStyles2 } from '@grafana/ui'
import type { ReactNode } from 'react'

const getStyles = (theme: GrafanaTheme2) => ({
  panel: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    display: 'flex',
    flexDirection: 'column',
    minWidth: 0,
  }),
  head: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(1),
    padding: theme.spacing(1.25, 2),
    borderBottom: `1px solid ${theme.colors.border.weak}`,
  }),
  body: css({ padding: theme.spacing(1, 0), flex: 1, minWidth: 0 }),
  flush: css({ padding: 0 }),
  empty: css({
    padding: theme.spacing(3, 2),
    textAlign: 'center',
    color: theme.colors.text.secondary,
  }),
})

// Dashboard section: header row with title + right-side link, list body below.
export function Panel({
  title,
  right,
  children,
  flush,
  className,
}: {
  title: ReactNode
  right?: ReactNode
  children: ReactNode
  flush?: boolean
  className?: string
}) {
  const styles = useStyles2(getStyles)
  return (
    <section className={cx(styles.panel, className)}>
      <div className={styles.head}>
        <Text element="h2" variant="h5">
          {title ?? ''}
        </Text>
        {right}
      </div>
      <div className={cx(styles.body, flush && styles.flush)}>{children}</div>
    </section>
  )
}

export function PanelEmpty({ children }: { children: ReactNode }) {
  const styles = useStyles2(getStyles)
  return <div className={styles.empty}>{children}</div>
}
