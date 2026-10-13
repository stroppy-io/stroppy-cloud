import { AppLink } from '@app/AppLink'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'
import { brand } from '@/styles/brand'

const getStyles = (theme: GrafanaTheme2) => ({
  foot: css({
    borderTop: `1px solid ${brand.line}`,
    background: brand.ink,
    color: brand.muted,
  }),
  inner: css({
    maxWidth: 1360,
    margin: '0 auto',
    padding: theme.spacing(6, 8, 5),
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1.4fr) repeat(3, minmax(0, 1fr))',
    gap: theme.spacing(4),
    [theme.breakpoints.down('md')]: {
      gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
      padding: theme.spacing(4, 2),
    },
  }),
  brandCol: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  name: css({ color: brand.text, fontWeight: theme.typography.fontWeightMedium }),
  colTitle: css({
    color: brand.text,
    marginBottom: theme.spacing(1),
    fontWeight: theme.typography.fontWeightMedium,
  }),
  list: css({
    listStyle: 'none',
    margin: 0,
    padding: 0,
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.75),
  }),
  link: css({ color: brand.muted, '&:hover': { color: brand.text, textDecoration: 'none' } }),
  legal: css({
    maxWidth: 1360,
    margin: '0 auto',
    padding: theme.spacing(0, 8, 4),
    fontSize: theme.typography.bodySmall.fontSize,
    [theme.breakpoints.down('md')]: { padding: theme.spacing(0, 2, 3) },
  }),
})

/** The bottom of every public page: product, community, legal. */
export function PublicFooter() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  return (
    <footer className={styles.foot}>
      <div className={styles.inner}>
        <div className={styles.brandCol}>
          <span className={styles.name}>Stroppy Cloud</span>
          <span>{t('landing.footer.about')}</span>
        </div>
        <div>
          <div className={styles.colTitle}>{t('landing.footer.product')}</div>
          <ul className={styles.list}>
            <li>
              <AppLink to="/leaderboard" plain className={styles.link}>
                {t('landing.nav.rating')}
              </AppLink>
            </li>
            <li>
              <AppLink to="/register" plain className={styles.link}>
                {t('landing.nav.start')}
              </AppLink>
            </li>
            <li>
              <AppLink to="/login" plain className={styles.link}>
                {t('landing.nav.signIn')}
              </AppLink>
            </li>
          </ul>
        </div>
        <div>
          <div className={styles.colTitle}>Stroppy</div>
          <ul className={styles.list}>
            <li>
              <a className={styles.link} href="https://stroppy.io/docs/introduction">
                {t('landing.nav.docs')}
              </a>
            </li>
            <li>
              <a className={styles.link} href="https://stroppy.io/blog">
                {t('landing.footer.blog')}
              </a>
            </li>
            <li>
              <a className={styles.link} href="https://github.com/stroppy-io/stroppy/releases">
                {t('landing.footer.releases')}
              </a>
            </li>
          </ul>
        </div>
        <div>
          <div className={styles.colTitle}>{t('landing.footer.community')}</div>
          <ul className={styles.list}>
            <li>
              <a className={styles.link} href="https://github.com/stroppy-io/stroppy">
                GitHub
              </a>
            </li>
            <li>
              <a className={styles.link} href="https://discord.gg/2mSSrkBkHm">
                Discord
              </a>
            </li>
          </ul>
        </div>
      </div>
      <div className={styles.legal}>
        {t('landing.footer.legal', { year: new Date().getFullYear() })}
      </div>
    </footer>
  )
}
