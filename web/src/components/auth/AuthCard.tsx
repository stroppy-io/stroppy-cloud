import { HeroMark } from '@components/landing/HeroMark'
import { HEADER_HEIGHT, PublicHeader } from '@components/landing/PublicHeader'
import { css } from '@emotion/css'
import { type GrafanaTheme2, ThemeContext } from '@grafana/data'
import { Icon, useStyles2 } from '@grafana/ui'
import { motionValue } from 'motion/react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { brand } from '@/styles/brand'
import { getAppTheme } from '@/styles/theme'

// The screens before the console (sign in, sign up, verification, recovery)
// are the landing's continuation: its header, its dark, its logo — the form
// on the left, what the account is for on the right.

const DARK = getAppTheme('dark')
const OPEN = motionValue(1)
const POINTS = ['stand', 'checked', 'rating'] as const
const POINT_ICON = { stand: 'cloud', checked: 'check-circle', rating: 'star' } as const

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    height: '100vh',
    display: 'flex',
    flexDirection: 'column',
    background: brand.ink,
    color: brand.text,
    overflow: 'hidden',
  }),
  main: css({
    flex: 1,
    minHeight: 0,
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr)',
    [theme.breakpoints.down('lg')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  formSide: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: theme.spacing(4, 3),
    overflowY: 'auto',
    height: `calc(100vh - ${HEADER_HEIGHT}px)`,
  }),
  form: css({ width: '100%', maxWidth: 400 }),
  title: css({
    margin: 0,
    fontSize: 'clamp(28px, 2.4vw, 34px)',
    lineHeight: 1.15,
    fontWeight: 700,
    letterSpacing: '-0.02em',
    color: brand.text,
  }),
  subtitle: css({ margin: theme.spacing(1, 0, 0), color: brand.muted, lineHeight: 1.5 }),
  body: css({
    marginTop: theme.spacing(4),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
  }),
  brandSide: css({
    position: 'relative',
    borderLeft: `1px solid ${brand.line}`,
    background: brand.panel,
    overflow: 'hidden',
    [theme.breakpoints.down('lg')]: { display: 'none' },
  }),
  // Smaller than on the landing: centred over the points, clear of them.
  mark: css({
    top: '8%',
    bottom: 'auto',
    left: 0,
    right: 0,
    margin: '0 auto',
    height: '50%',
  }),
  points: css({
    position: 'absolute',
    left: theme.spacing(6),
    right: theme.spacing(6),
    bottom: theme.spacing(6),
    zIndex: 1,
    margin: 0,
    padding: 0,
    listStyle: 'none',
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
  }),
  point: css({
    display: 'grid',
    gridTemplateColumns: '24px minmax(0, 1fr)',
    gap: theme.spacing(1.5),
  }),
  pointIcon: css({ color: theme.colors.primary.text, marginTop: 2 }),
  pointTitle: css({ color: brand.text, fontWeight: theme.typography.fontWeightMedium }),
  pointBody: css({ color: brand.muted, lineHeight: 1.5, maxWidth: 420 }),
})

function Frame({
  title,
  subtitle,
  children,
}: {
  title: string
  subtitle?: string
  children: ReactNode
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  return (
    <div className={styles.root}>
      <PublicHeader />
      <div className={styles.main}>
        <div className={styles.formSide}>
          <div className={styles.form}>
            <h1 className={styles.title}>{title}</h1>
            {subtitle && <p className={styles.subtitle}>{subtitle}</p>}
            <div className={styles.body}>{children}</div>
          </div>
        </div>
        <aside className={styles.brandSide} aria-label={t('auth.shell.label')}>
          <HeroMark open={OPEN} className={styles.mark} tone="solid" />
          <ul className={styles.points}>
            {POINTS.map((p) => (
              <li key={p} className={styles.point}>
                {/* Its own cell: Icon renders nothing until the SVG loads, and the text must not slide into its column. */}
                <span className={styles.pointIcon}>
                  <Icon name={POINT_ICON[p]} size="lg" />
                </span>
                <div>
                  <div className={styles.pointTitle}>{t(`auth.shell.${p}.title`)}</div>
                  <div className={styles.pointBody}>{t(`auth.shell.${p}.body`)}</div>
                </div>
              </li>
            ))}
          </ul>
        </aside>
      </div>
    </div>
  )
}

/** The frame of every bare auth screen: the public header, the form, the brand side. */
export function AuthCard(props: { title: string; subtitle?: string; children: ReactNode }) {
  return (
    <ThemeContext.Provider value={DARK}>
      <Frame {...props} />
    </ThemeContext.Provider>
  )
}
