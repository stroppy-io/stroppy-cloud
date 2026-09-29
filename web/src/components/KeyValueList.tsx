import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import type { ReactNode } from 'react'

const getStyles = (theme: GrafanaTheme2) => ({
  dl: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(96px, max-content) 1fr',
    gap: theme.spacing(0.75, 2),
    margin: 0,
    alignItems: 'baseline',
  }),
  dt: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    whiteSpace: 'nowrap',
  }),
  dd: css({ margin: 0, minWidth: 0, overflowWrap: 'anywhere' }),
  section: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  title: css({
    fontSize: theme.typography.bodySmall.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: 0.4,
    margin: 0,
  }),
})

export interface KV {
  label: ReactNode
  value: ReactNode
}

// Sidebar-style metadata list (GitHub "About" block).
export function KeyValueList({ items, title }: { items: KV[]; title?: ReactNode }) {
  const styles = useStyles2(getStyles)
  return (
    <section className={styles.section}>
      {title && <h3 className={styles.title}>{title}</h3>}
      <dl className={styles.dl}>
        {items.map((it, i) => (
          <div key={i} style={{ display: 'contents' }}>
            <dt className={styles.dt}>{it.label}</dt>
            <dd className={styles.dd}>{it.value ?? '—'}</dd>
          </div>
        ))}
      </dl>
    </section>
  )
}
