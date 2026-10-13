import { AppLink } from '@app/AppLink'
import { Logo } from '@components/Logo'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import { type Lang, SUPPORTED_LANGS, setLang } from '@lib/i18n'
import { useTranslation } from 'react-i18next'
import { brand } from '@/styles/brand'

/** Height of the public top bar; pages lay out below it. */
export const HEADER_HEIGHT = 60

const getStyles = (theme: GrafanaTheme2) => ({
  bar: css({
    position: 'sticky',
    top: 0,
    zIndex: 10,
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(3),
    height: HEADER_HEIGHT,
    padding: theme.spacing(0, 4),
    background: `color-mix(in srgb, ${brand.ink} 82%, transparent)`,
    backdropFilter: 'blur(10px)',
    borderBottom: `1px solid ${brand.line}`,
    [theme.breakpoints.down('md')]: { padding: theme.spacing(0, 2), gap: theme.spacing(2) },
  }),
  logo: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    color: brand.text,
    fontWeight: theme.typography.fontWeightBold,
    fontSize: theme.typography.h5.fontSize,
    letterSpacing: '-0.01em',
    '&:hover': { color: brand.text, textDecoration: 'none' },
  }),
  logoButton: css({ background: 'none', border: 'none', padding: 0, cursor: 'pointer' }),
  nav: css({
    display: 'flex',
    gap: theme.spacing(2.5),
    [theme.breakpoints.down('md')]: { display: 'none' },
  }),
  link: css({
    color: brand.muted,
    fontSize: theme.typography.body.fontSize,
    '&:hover': { color: brand.text, textDecoration: 'none' },
  }),
  right: css({ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: theme.spacing(2) }),
  // RU | EN: the current language is lit, the other one is the switch.
  langs: css({
    display: 'flex',
    margin: 0,
    minWidth: 0,
    padding: 2,
    gap: 2,
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${brand.line}`,
  }),
  lang: css({
    minWidth: 32,
    height: 26,
    padding: theme.spacing(0, 0.75),
    border: 'none',
    borderRadius: theme.shape.radius.default,
    background: 'none',
    color: brand.muted,
    cursor: 'pointer',
    fontSize: theme.typography.bodySmall.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    '&:hover': { color: brand.text },
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: 1 },
  }),
  langActive: css({ background: brand.raised, color: brand.text, cursor: 'default' }),
  cta: css({
    display: 'inline-flex',
    alignItems: 'center',
    height: 36,
    padding: theme.spacing(0, 2),
    borderRadius: theme.shape.radius.default,
    background: theme.colors.primary.main,
    color: theme.colors.primary.contrastText,
    fontWeight: theme.typography.fontWeightMedium,
    '&:hover': {
      color: theme.colors.primary.contrastText,
      textDecoration: 'none',
      background: theme.colors.primary.shade,
    },
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: 2 },
  }),
})

/**
 * The top bar of every public page: product, rating, docs; sign in or start.
 * `onHome` lets the landing rewind its deck when the logo is clicked there.
 */
export function PublicHeader({ onHome }: { onHome?: () => void }) {
  const { t, i18n } = useTranslation()
  const styles = useStyles2(getStyles)
  const current: Lang = i18n.language.startsWith('ru') ? 'ru' : 'en'
  const logo = (
    <>
      <Logo size={32} color={brand.text} />
      Stroppy Cloud
    </>
  )
  return (
    <header className={styles.bar}>
      {onHome ? (
        <button type="button" className={cx(styles.logo, styles.logoButton)} onClick={onHome}>
          {logo}
        </button>
      ) : (
        <AppLink to="/" plain className={styles.logo}>
          {logo}
        </AppLink>
      )}
      <nav className={styles.nav} aria-label={t('landing.nav.label')}>
        <AppLink to="/leaderboard" plain className={styles.link}>
          {t('landing.nav.rating')}
        </AppLink>
        <a className={styles.link} href="https://stroppy.io/docs/introduction">
          {t('landing.nav.docs')}
        </a>
        <a className={styles.link} href="https://github.com/stroppy-io/stroppy">
          GitHub
        </a>
      </nav>
      <div className={styles.right}>
        <fieldset className={styles.langs} aria-label={t('nav.language')}>
          {SUPPORTED_LANGS.map((l) => (
            <button
              key={l}
              type="button"
              lang={l}
              className={cx(styles.lang, l === current && styles.langActive)}
              aria-pressed={l === current}
              title={t(`common.lang.${l}`)}
              onClick={() => setLang(l)}
            >
              {l.toUpperCase()}
            </button>
          ))}
        </fieldset>
        <AppLink to="/login" plain className={styles.link}>
          {t('landing.nav.signIn')}
        </AppLink>
        <AppLink to="/register" plain className={cx(styles.cta)}>
          {t('landing.nav.start')}
        </AppLink>
      </div>
    </header>
  )
}
