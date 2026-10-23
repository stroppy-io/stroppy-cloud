import { meQueries } from '@api/queries/me'
import { AppLink } from '@app/AppLink'
import { useThemeMode } from '@app/theme-context'
import { Logo } from '@components/Logo'
import { UserAvatar } from '@components/UserAvatar'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Dropdown, Icon, IconButton, Menu, Text, useStyles2, useTheme2 } from '@grafana/ui'
import { useSignedIn } from '@hooks/useSignedIn'
import { logout } from '@lib/auth'
import { type Lang, SUPPORTED_LANGS, setLang } from '@lib/i18n'
import { SITE_URL } from '@lib/site'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useRouterState } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { brandOf } from '@/styles/brand'

/** Height of the public top bar; pages lay out below it. */
export const HEADER_HEIGHT = 60

const getStyles = (theme: GrafanaTheme2) => {
  const brand = brandOf(theme)
  return {
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
    link: css({
      display: 'inline-flex',
      alignItems: 'center',
      gap: theme.spacing(0.5),
      color: brand.muted,
      fontSize: theme.typography.body.fontSize,
      '&:hover': { color: brand.text, textDecoration: 'none' },
      '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: 2 },
    }),
    right: css({
      marginLeft: 'auto',
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(2),
    }),
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
  }
}

/**
 * The top bar of the console's public screens (sign-in, onboarding, the
 * rating): the logo and "back to the site" lead to stroppy.io, the menu lives
 * there. Right: language, theme, the account or sign in.
 */
export function SiteBar() {
  const { t, i18n } = useTranslation()
  const signedIn = useSignedIn()
  // Only with a session: a guest has no profile to show (and must not be bounced to sign-in).
  const me = useQuery({ ...meQueries.me(), enabled: signedIn })
  const navigate = useNavigate()
  const pathname = useRouterState({ select: (st) => st.location.pathname })
  const qc = useQueryClient()
  const { mode, setMode } = useThemeMode()
  // Signing out keeps the visitor on the page they are on, now as a guest.
  const signOut = async () => {
    await logout()
    // Tell the page first (its observers keep this query), then drop the rest.
    qc.setQueryData(['auth', 'signed-in'], false)
    qc.removeQueries({ predicate: (q) => q.queryKey[0] !== 'auth' })
  }
  const styles = useStyles2(getStyles)
  const brand = brandOf(useTheme2())
  const current: Lang = i18n.language.startsWith('ru') ? 'ru' : 'en'
  return (
    <header className={styles.bar}>
      <a href={SITE_URL} className={styles.logo}>
        <Logo size={32} color={brand.muted} />
        Stroppy Cloud
      </a>
      <a href={SITE_URL} className={styles.link}>
        <Icon name="arrow-left" size="sm" />
        {t('landing.nav.site')}
      </a>
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
        <IconButton
          name="adjust-circle"
          tooltip={t('nav.theme')}
          onClick={() => setMode(mode === 'dark' ? 'light' : 'dark')}
        />
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
