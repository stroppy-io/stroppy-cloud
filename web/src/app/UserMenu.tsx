import { useTour } from '@components/onboarding/tour'
import { ConceptDiagram } from '@components/public/ConceptDiagram'
import { UserAvatar } from '@components/UserAvatar'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Dropdown, Menu, Modal, Text, ToolbarButton, useStyles2 } from '@grafana/ui'
import { useMe } from '@hooks/useMe'
import { logout } from '@lib/auth'
import { getPublicConfig } from '@lib/config'
import { type Lang, setLang } from '@lib/i18n'
import { SITE_URL } from '@lib/site'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useThemeMode } from './theme-context'

const getStyles = (theme: GrafanaTheme2) => ({
  // Wide enough for the diagram with its side card.
  concepts: css({ width: 'min(1180px, 94vw)' }),
  header: css({
    display: 'flex',
    flexDirection: 'column',
    padding: theme.spacing(0.5, 1),
  }),
})

// One menu for everything personal: profile, tokens, invites, theme, language, shortcuts, admin.
export function UserMenu({ onShortcuts }: { onShortcuts: () => void }) {
  const styles = useStyles2(getStyles)
  const { t, i18n } = useTranslation()
  const me = useMe()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { mode, setMode } = useThemeMode()
  const startTour = useTour()
  const [conceptsOpen, setConceptsOpen] = useState(false)
  const signOut = async () => {
    await logout()
    qc.clear()
    // Out of the console is back to the site.
    window.location.assign(SITE_URL)
  }
  return (
    <>
      <Dropdown
        placement="bottom-end"
        overlay={
          <Menu
            header={
              <div className={styles.header}>
                <Text weight="medium">{me.display_name}</Text>
                <Text variant="bodySmall" color="secondary">
                  {me.email ?? me.id}
                </Text>
              </div>
            }
          >
            <Menu.Item
              label={t('nav.profile')}
              icon="user"
              onClick={() => void navigate({ to: '/me/profile' as never })}
            />
            <Menu.Item
              label={t('nav.tokens')}
              icon="key-skeleton-alt"
              onClick={() => void navigate({ to: '/me/tokens' as never })}
            />
            {getPublicConfig()?.auth.mode === 'kratos' && (
              <Menu.Item
                label={t('me.tabs.security')}
                icon="lock"
                onClick={() => void navigate({ to: '/me/security', search: {} })}
              />
            )}
            <Menu.Item
              label={t('nav.invites')}
              icon="envelope"
              onClick={() => void navigate({ to: '/me/invites' as never })}
            />
            <Menu.Divider />
            <Menu.Group label={t('nav.theme')}>
              <Menu.Item
                label={t('common.theme.dark')}
                icon={mode === 'dark' ? 'check' : undefined}
                onClick={() => setMode('dark')}
              />
              <Menu.Item
                label={t('common.theme.light')}
                icon={mode === 'light' ? 'check' : undefined}
                onClick={() => setMode('light')}
              />
            </Menu.Group>
            <Menu.Group label={t('nav.language')}>
              {(['en', 'ru'] as Lang[]).map((l) => (
                <Menu.Item
                  key={l}
                  label={t(`common.lang.${l}`)}
                  icon={i18n.language.startsWith(l) ? 'check' : undefined}
                  onClick={() => setLang(l)}
                />
              ))}
            </Menu.Group>
            <Menu.Item label={t('nav.shortcuts')} icon="keyboard" onClick={onShortcuts} />
            <Menu.Item
              label={t('onboarding.menu.concepts')}
              icon="sitemap"
              onClick={() => setConceptsOpen(true)}
            />
            <Menu.Item label={t('onboarding.menu.tour')} icon="compass" onClick={startTour} />
            <Menu.Divider />
            {me.is_platform_admin && (
              <Menu.Item
                label={t('nav.admin')}
                icon="shield"
                onClick={() => void navigate({ to: '/admin' as never })}
              />
            )}
            <Menu.Item label={t('nav.signOut')} icon="signout" onClick={() => void signOut()} />
          </Menu>
        }
      >
        <ToolbarButton
          aria-label={me.display_name}
          iconOnly
          narrow
          icon={<UserAvatar user={me} size={24} />}
        />
      </Dropdown>
      <Modal
        title={t('onboarding.welcome.howTitle')}
        isOpen={conceptsOpen}
        onDismiss={() => setConceptsOpen(false)}
        className={styles.concepts}
      >
        <ConceptDiagram />
      </Modal>
    </>
  )
}
