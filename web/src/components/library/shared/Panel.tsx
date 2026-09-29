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
  }),
  head: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(1),
    padding: theme.spacing(1.25, 2),
    borderBottom: `1px solid ${theme.colors.border.weak}`,
  }),
  body: css({ padding: theme.spacing(2) }),
  dense: css({ padding: theme.spacing(1) }),
  desc: css({ color: theme.colors.text.secondary, fontSize: theme.typography.bodySmall.fontSize }),
})

// Bordered content block with a title row (GitHub "box").
export function Panel({
  title,
  description,
  actions,
  children,
  dense,
  className,
}: {
  title: ReactNode
  description?: ReactNode
  actions?: ReactNode
  children: ReactNode
  dense?: boolean
  className?: string
}) {
  const styles = useStyles2(getStyles)
  return (
    <section className={cx(styles.panel, className)}>
      <div className={styles.head}>
        <div>
          <Text element="h2" variant="h5">
            {title ?? ''}
          </Text>
          {description && <div className={styles.desc}>{description}</div>}
        </div>
        {actions}
      </div>
      <div className={cx(styles.body, dense && styles.dense)}>{children}</div>
    </section>
  )
}

const getLayoutStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) 320px',
    gap: theme.spacing(2),
    alignItems: 'start',
    [theme.breakpoints.down('lg')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  main: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2), minWidth: 0 }),
})

// Main column + right sidebar (GitHub entity layout).
export function WithSidebar({ children, sidebar }: { children: ReactNode; sidebar: ReactNode }) {
  const styles = useStyles2(getLayoutStyles)
  return (
    <div className={styles.grid}>
      <div className={styles.main}>{children}</div>
      <div>{sidebar}</div>
    </div>
  )
}
