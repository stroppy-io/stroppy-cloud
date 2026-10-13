import { AppLink } from '@app/AppLink'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import { motionValue } from 'motion/react'
import { useTranslation } from 'react-i18next'
import { brand } from '@/styles/brand'
import { HeroMark } from './HeroMark'
import { PublicFooter } from './PublicFooter'

// The closing screen shows the headline's logo as the reader left it: open.
const OPEN = motionValue(1)

const getStyles = (theme: GrafanaTheme2) => ({
  page: css({ height: '100%', display: 'flex', flexDirection: 'column' }),
  // Holds the logo; the text sits above it.
  stage: css({ position: 'relative', flex: 1, display: 'flex', minHeight: 0 }),
  text: css({ position: 'relative', zIndex: 1 }),
  cta: css({
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    justifyContent: 'center',
    width: '100%',
    maxWidth: 1360,
    margin: '0 auto',
    padding: theme.spacing(0, 8),
    [theme.breakpoints.down('md')]: { padding: theme.spacing(0, 2) },
  }),
  title: css({
    margin: 0,
    maxWidth: 820,
    fontSize: 'clamp(32px, 4.4vw, 60px)',
    lineHeight: 1.05,
    fontWeight: 700,
    letterSpacing: '-0.03em',
    color: brand.text,
  }),
  lead: css({
    marginTop: theme.spacing(2.5),
    maxWidth: 600,
    fontSize: 'clamp(16px, 1.3vw, 19px)',
    lineHeight: 1.55,
    color: brand.muted,
  }),
  actions: css({
    display: 'flex',
    gap: theme.spacing(2),
    marginTop: theme.spacing(4),
    flexWrap: 'wrap',
  }),
  primary: css({
    display: 'inline-flex',
    alignItems: 'center',
    height: 48,
    padding: theme.spacing(0, 3),
    borderRadius: theme.shape.radius.default,
    background: theme.colors.primary.main,
    color: theme.colors.primary.contrastText,
    fontWeight: theme.typography.fontWeightMedium,
    fontSize: theme.typography.h5.fontSize,
    '&:hover': {
      color: theme.colors.primary.contrastText,
      textDecoration: 'none',
      background: theme.colors.primary.shade,
    },
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: 3 },
  }),
  secondary: css({
    display: 'inline-flex',
    alignItems: 'center',
    height: 48,
    padding: theme.spacing(0, 3),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${brand.line}`,
    color: brand.text,
    fontSize: theme.typography.h5.fontSize,
    '&:hover': { color: brand.text, textDecoration: 'none', borderColor: brand.muted },
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: 3 },
  }),
})

/** The last screen of the landing: the next step, then the footer. */
export function EndPage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  return (
    <div className={styles.page}>
      <div className={styles.stage}>
        <HeroMark open={OPEN} />
        <div className={styles.cta}>
          <div className={styles.text}>
            <h2 className={styles.title}>{t('landing.end.title')}</h2>
            <p className={styles.lead}>{t('landing.end.lead')}</p>
            <div className={styles.actions}>
              <AppLink to="/register" plain className={styles.primary}>
                {t('landing.hero.start')}
              </AppLink>
              <a className={styles.secondary} href="https://stroppy.io/docs/introduction">
                {t('landing.end.docs')}
              </a>
            </div>
          </div>
        </div>
      </div>
      <PublicFooter />
    </div>
  )
}
