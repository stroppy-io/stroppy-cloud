import { Page } from '@app/Page'
import { LangSwitch } from '@components/auth/AuthCard'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Badge, Button, Stack, Text, TextLink, useStyles2 } from '@grafana/ui'
import { useMe } from '@hooks/useMe'
import { useOnboarding } from '@hooks/useOnboarding'
import { tokenClaims } from '@lib/auth'
import { getPublicConfig } from '@lib/config'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ConceptMap } from './ConceptMap'

const getStyles = (theme: GrafanaTheme2) => ({
  card: css({
    maxWidth: 920,
    margin: `${theme.spacing(8)} auto ${theme.spacing(4)}`,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(4),
  }),
  next: css({
    padding: theme.spacing(2),
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
  }),
  scroll: css({ height: '100%', overflow: 'auto' }),
})

/**
 * The first screen after the first sign-in. Three cases: an unconfirmed
 * e-mail (an invite applies only once it is confirmed), an account already
 * in a tenant (invited), an account with nothing yet (creates its tenant).
 * Below — how the product is built.
 */
export function WelcomePage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const navigate = useNavigate()
  const me = useMe()
  const onboarding = useOnboarding()
  const cfg = getPublicConfig()
  const unverified = cfg?.auth.mode === 'kratos' && tokenClaims().emailVerified === false
  const tenant = me.tenants[0]
  const demo = cfg?.demo_report_url

  const done = async (to: () => void) => {
    await onboarding.set({ welcome_done: true })
    to()
  }

  return (
    <Page>
      <div className={styles.scroll}>
        <LangSwitch />
        <div className={styles.card}>
          <Stack direction="column" gap={3}>
            <Stack direction="column" gap={1}>
              <Text element="h1" variant="h2">
                {t('onboarding.welcome.title', { name: me.display_name })}
              </Text>
              <Text color="secondary">{t('onboarding.welcome.subtitle')}</Text>
            </Stack>

            <div className={styles.next}>
              {unverified ? (
                <Stack direction="column" gap={2}>
                  <Alert severity="info" title={t('onboarding.welcome.verifyTitle')}>
                    {t('onboarding.welcome.verifyBody')}
                  </Alert>
                  <Stack gap={1} wrap="wrap">
                    <Button
                      icon="envelope"
                      onClick={() => void navigate({ to: '/verify', search: {} })}
                    >
                      {t('onboarding.welcome.verify')}
                    </Button>
                    <Button
                      variant="secondary"
                      onClick={() => void done(() => navigate({ to: '/tenants/new' }))}
                    >
                      {t('onboarding.welcome.notInvited')}
                    </Button>
                  </Stack>
                </Stack>
              ) : tenant ? (
                <Stack direction="column" gap={2}>
                  <Text weight="medium">{t('onboarding.welcome.invitedTitle')}</Text>
                  <Stack direction="column" gap={1}>
                    {me.tenants.map((m) => (
                      <Stack key={m.tenant.id} alignItems="center" gap={1}>
                        <Text>{m.tenant.name}</Text>
                        <Badge text={t(`common.role.${m.role}`)} color="blue" />
                      </Stack>
                    ))}
                  </Stack>
                  <div>
                    <Button
                      icon="arrow-right"
                      onClick={() =>
                        void done(() =>
                          navigate({ to: '/t/$slug', params: { slug: tenant.tenant.slug } })
                        )
                      }
                    >
                      {t('onboarding.welcome.goTenant', { tenant: tenant.tenant.name })}
                    </Button>
                  </div>
                </Stack>
              ) : (
                <Stack direction="column" gap={2}>
                  <Text weight="medium">{t('onboarding.welcome.createTitle')}</Text>
                  <Text color="secondary">{t('onboarding.welcome.createBody')}</Text>
                  <Stack gap={1} wrap="wrap" alignItems="center">
                    <Button
                      icon="plus"
                      onClick={() => void done(() => navigate({ to: '/tenants/new' }))}
                    >
                      {t('onboarding.welcome.create')}
                    </Button>
                    {demo && (
                      <TextLink href={demo} external>
                        {t('onboarding.welcome.demo')}
                      </TextLink>
                    )}
                  </Stack>
                </Stack>
              )}
            </div>

            <Stack direction="column" gap={1}>
              <Text element="h2" variant="h4">
                {t('onboarding.welcome.howTitle')}
              </Text>
              <ConceptMap />
            </Stack>

            <div>
              <Button
                variant="secondary"
                fill="text"
                onClick={() => void done(() => navigate({ to: '/' }))}
              >
                {t('onboarding.welcome.skip')}
              </Button>
            </div>
          </Stack>
        </div>
      </div>
    </Page>
  )
}
