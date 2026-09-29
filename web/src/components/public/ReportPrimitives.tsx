import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Text, useStyles2 } from '@grafana/ui'
import type { ReactNode } from 'react'

const getStyles = (theme: GrafanaTheme2) => ({
  section: css({
    marginTop: theme.spacing(5),
    breakInside: 'avoid-page',
    '&:first-of-type': { marginTop: theme.spacing(3) },
  }),
  head: css({
    display: 'flex',
    alignItems: 'baseline',
    gap: theme.spacing(1.5),
    paddingBottom: theme.spacing(1),
    borderBottom: `1px solid ${theme.colors.border.medium}`,
    marginBottom: theme.spacing(2),
    flexWrap: 'wrap',
  }),
  num: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.h4.fontSize,
  }),
  hint: css({ color: theme.colors.text.secondary, marginLeft: 'auto' }),
  sub: css({
    margin: theme.spacing(3, 0, 1),
    fontSize: theme.typography.h5.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  table: css({
    width: '100%',
    borderCollapse: 'collapse',
    fontSize: theme.typography.bodySmall.fontSize,
    'th, td': {
      padding: theme.spacing(0.75, 1.25),
      borderBottom: `1px solid ${theme.colors.border.weak}`,
      textAlign: 'left',
      verticalAlign: 'top',
    },
    th: {
      color: theme.colors.text.secondary,
      fontWeight: theme.typography.fontWeightMedium,
      whiteSpace: 'nowrap',
      background: theme.colors.background.secondary,
    },
    'td.num, th.num': { textAlign: 'right', fontVariantNumeric: 'tabular-nums' },
    'td.mono': { fontFamily: theme.typography.fontFamilyMonospace },
    'tbody tr:last-child td': { borderBottom: 0 },
  }),
  tableWrap: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    overflowX: 'auto',
    background: theme.colors.background.primary,
  }),
  card: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(1.5, 2),
    minWidth: 0,
  }),
  cards: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))',
    gap: theme.spacing(1.5),
  }),
  muted: css({ color: theme.colors.text.secondary }),
  pre: css({
    margin: 0,
    padding: theme.spacing(1.5, 2),
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    lineHeight: 1.5,
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-word',
    background: theme.colors.background.canvas,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    maxHeight: 480,
    overflow: 'auto',
    '@media print': { maxHeight: 'none' },
  }),
})

export function useReportStyles() {
  return useStyles2(getStyles)
}

export function ReportSection({
  id,
  index,
  title,
  hint,
  children,
}: {
  id: string
  index: number
  title: ReactNode
  hint?: ReactNode
  children: ReactNode
}) {
  const styles = useReportStyles()
  return (
    <section id={id} className={styles.section}>
      <div className={styles.head}>
        <span className={styles.num}>{String(index).padStart(2, '0')}</span>
        <Text element="h2" variant="h3">
          {title ?? ''}
        </Text>
        {hint && <span className={styles.hint}>{hint}</span>}
      </div>
      {children}
    </section>
  )
}

export function SubTitle({ children }: { children: ReactNode }) {
  const styles = useReportStyles()
  return <h3 className={styles.sub}>{children}</h3>
}

export function SimpleTable({ children, className }: { children: ReactNode; className?: string }) {
  const styles = useReportStyles()
  return (
    <div className={cx(styles.tableWrap, className)}>
      <table className={styles.table}>{children}</table>
    </div>
  )
}

export function Pre({ children }: { children: string }) {
  const styles = useReportStyles()
  return <pre className={styles.pre}>{children}</pre>
}

export function valueToText(v: unknown): string {
  if (v === null || v === undefined) return '—'
  if (typeof v === 'string') return v
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  return JSON.stringify(v)
}
