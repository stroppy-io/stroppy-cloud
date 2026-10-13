import { meQueries } from '@api/queries/me'
import { AppLink } from '@app/AppLink'
import { Logo } from '@components/Logo'
import { UserAvatar } from '@components/UserAvatar'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Dropdown, Icon, Menu, Text, useStyles2 } from '@grafana/ui'
import { useSignedIn } from '@hooks/useSignedIn'
import { logout } from '@lib/auth'
import { type Lang, SUPPORTED_LANGS, setLang } from '@lib/i18n'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useRouterState } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { brand } from '@/styles/brand'

/** Height of the public top bar; pages lay out below it. */
export const HEADER_HEIGHT = 60

// The GitHub mark (Octicons, MIT): there is none among the Grafana icons.
const GITHUB_MARK =
  'M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z'

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
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    color: brand.muted,
    fontSize: theme.typography.body.fontSize,
    '&:hover': { color: brand.text, textDecoration: 'none' },
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: 2 },
  }),
  linkActive: css({ color: brand.text }),
  external: css({
    [theme.breakpoints.down('lg')]: { display: 'none' },
  }),
  icon: css({ display: 'inline-flex', color: brand.muted, '&:hover': { color: brand.text } }),
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
  // The signed-in visitor, as one control: the way into the console on the
  // left, the account (profile, sign out) behind the avatar on the right.
  account: css({
    display: 'inline-flex',
    alignItems: 'stretch',
    height: 38,
    borderRadius: theme.shape.radius.pill,
    border: `1px solid ${brand.line}`,
    overflow: 'hidden',
    '&:hover': { borderColor: brand.muted },
  }),
  accountGo: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.75),
    padding: theme.spacing(0, 1.5),
    color: brand.text,
    fontWeight: theme.typography.fontWeightMedium,
    '&:hover': { color: brand.text, textDecoration: 'none', background: brand.raised },
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: -2 },
  }),
  accountSplit: css({ width: 1, background: brand.line }),
  accountMenu: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    padding: theme.spacing(0, 1, 0, 0.75),
    border: 'none',
    background: 'none',
    color: brand.muted,
    cursor: 'pointer',
    '&:hover': { color: brand.text, background: brand.raised },
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: -2 },
  }),
  menuHead: css({ display: 'flex', flexDirection: 'column', padding: theme.spacing(1, 1.5) }),
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
 * The top bar of every public page. Left: the name (back to the landing) and
 * the rating; right: links out (docs, GitHub), language, sign in or start.
 * On the landing `onHome` rewinds its deck in place.
 */
export function PublicHeader({ onHome }: { onHome?: () => void }) {
  const { t, i18n } = useTranslation()
  const signedIn = useSignedIn()
  // Only with a session: a guest has no profile to show (and must not be bounced to sign-in).
  const me = useQuery({ ...meQueries.me(), enabled: signedIn })
  const navigate = useNavigate()
  const pathname = useRouterState({ select: (st) => st.location.pathname })
  const qc = useQueryClient()
  // Signing out keeps the visitor on the page they are on, now as a guest.
  const signOut = async () => {
    await logout()
    // Tell the page first (its observers keep this query), then drop the rest.
    qc.setQueryData(['auth', 'signed-in'], false)
    qc.removeQueries({ predicate: (q) => q.queryKey[0] !== 'auth' })
  }
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
        <AppLink to="/land" plain className={styles.logo}>
          {logo}
        </AppLink>
      )}
      <nav className={styles.nav} aria-label={t('landing.nav.label')}>
        <AppLink
          to="/leaderboard"
          plain
          className={styles.link}
          activeProps={{ className: cx(styles.link, styles.linkActive), 'aria-current': 'page' }}
        >
          {t('landing.nav.rating')}
        </AppLink>
        <AppLink
          to="/how"
          plain
          className={styles.link}
          activeProps={{ className: cx(styles.link, styles.linkActive), 'aria-current': 'page' }}
        >
          {t('landing.nav.how')}
        </AppLink>
      </nav>
      <div className={styles.right}>
        <a
          className={cx(styles.link, styles.external)}
          href="https://stroppy.io/docs/introduction"
          target="_blank"
          rel="noreferrer"
        >
          {t('landing.nav.docs')}
          <Icon name="external-link-alt" size="xs" />
        </a>
        <a
          className={styles.icon}
          href="https://github.com/stroppy-io/stroppy"
          target="_blank"
          rel="noreferrer"
        >
          <svg width="20" height="20" viewBox="0 0 16 16" fill="currentColor" role="img">
            <title>GitHub</title>
            <path d={GITHUB_MARK} />
          </svg>
        </a>
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
        {signedIn ? (
          <div className={styles.account}>
            <AppLink to="/" plain className={styles.accountGo}>
              <Icon name="apps" size="sm" />
              {t('landing.nav.console')}
            </AppLink>
            <span className={styles.accountSplit} />
            <Dropdown
              placement="bottom-end"
              overlay={
                <Menu
                  header={
                    me.data && (
                      <div className={styles.menuHead}>
                        <Text weight="medium">{me.data.display_name}</Text>
                        <Text variant="bodySmall" color="secondary">
                          {me.data.email ?? me.data.id}
                        </Text>
                      </div>
                    )
                  }
                >
                  <Menu.Item
                    label={t('nav.profile')}
                    icon="user"
                    onClick={() => void navigate({ to: '/me/profile' as never })}
                  />
                  <Menu.Divider />
                  <Menu.Item
                    label={t('nav.signOut')}
                    icon="signout"
                    onClick={() => void signOut()}
                  />
                </Menu>
              }
            >
              <button
                type="button"
                className={styles.accountMenu}
                aria-label={t('landing.nav.account')}
                title={me.data?.display_name}
              >
                {me.data && <UserAvatar user={me.data} size={26} />}
                <Icon name="angle-down" size="sm" />
              </button>
            </Dropdown>
          </div>
        ) : (
          <>
            {/* No link to the screen the visitor is on. */}
            {pathname !== '/login' && (
              <AppLink to="/login" plain className={styles.link}>
                {t('landing.nav.signIn')}
              </AppLink>
            )}
            {pathname !== '/register' && (
              <AppLink to="/register" plain className={styles.cta}>
                {t('landing.nav.start')}
              </AppLink>
            )}
          </>
        )}
      </div>
    </header>
  )
}
