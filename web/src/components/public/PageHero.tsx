import { HeroMark } from '@components/landing/HeroMark'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import { motionValue } from 'motion/react'
import type { ReactNode } from 'react'
import { brand } from '@/styles/brand'

// The head of a public page: a large title, one lead line, and the brand
// mark — open, muted — behind it, the way the landing ends.

const OPEN = motionValue(1)

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    position: 'relative',
    display: 'flex',
    alignItems: 'flex-end',
    gap: theme.spacing(4),
    flexWrap: 'wrap',
    marginBottom: theme.spacing(4),
  }),
  mark: css({
    top: '-30%',
    bottom: 'auto',
    right: 0,
    height: '170%',
    opacity: 0.22,
    [theme.breakpoints.down('lg')]: { right: '-8vw', opacity: 0.14 },
  }),
  text: css({ position: 'relative', zIndex: 1, flex: '1 1 520px', minWidth: 0 }),
  title: css({
    margin: 0,
    maxWidth: 860,
    fontSize: 'clamp(36px, 4.2vw, 60px)',
    lineHeight: 1.04,
    fontWeight: 700,
    letterSpacing: '-0.035em',
    color: brand.text,
  }),
  lead: css({
    margin: theme.spacing(2, 0, 0),
    maxWidth: 680,
    fontSize: 'clamp(16px, 1.25vw, 19px)',
    lineHeight: 1.55,
    color: brand.muted,
  }),
  actions: css({ position: 'relative', zIndex: 1 }),
})

export function PageHero({
  title,
  lead,
  actions,
}: {
  title: string
  lead: string
  actions?: ReactNode
}) {
  const styles = useStyles2(getStyles)
  return (
    <header className={styles.root}>
      <HeroMark open={OPEN} className={styles.mark} />
      <div className={styles.text}>
        <h1 className={styles.title}>{title}</h1>
        <p className={styles.lead}>{lead}</p>
      </div>
      {actions && <div className={styles.actions}>{actions}</div>}
    </header>
  )
}
