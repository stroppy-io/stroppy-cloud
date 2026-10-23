import { ConceptDiagram } from '@components/public/ConceptDiagram'
import { PublicShell } from '@components/public/PublicShell'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Badge, Button, Stack, Text, TextLink, useStyles2 } from '@grafana/ui'
import { useMe } from '@hooks/useMe'
import { useOnboarding } from '@hooks/useOnboarding'
import { tokenClaims } from '@lib/auth'
import { getPublicConfig } from '@lib/config'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { brandOf } from '@/styles/brand'

const getStyles = (theme: GrafanaTheme2) => {
  const brand = brandOf(theme)
  return {
    title: css({
      margin: 0,
      fontSize: 'clamp(30px, 3.2vw, 44px)',
      lineHeight: 1.1,
      fontWeight: 700,
      letterSpacing: '-0.025em',
      color: brand.text,
    }),
    subtitle: css({
      margin: theme.spacing(1.5, 0, 0),
      color: brand.muted,
      maxWidth: 640,
      lineHeight: 1.55,
    }),
    // The one thing to do next, set apart from the explanation below it.
    next: css({
      marginTop: theme.spacing(4),
      padding: theme.spacing(3),
      borderRadius: 10,
      border: `1px solid ${brand.line}`,
      background: brand.panel,
    }),
    how: css({ marginTop: theme.spacing(6) }),
    howTitle: css({
      margin: theme.spacing(0, 0, 2),
      fontSize: theme.typography.h4.fontSize,
      fontWeight: theme.typography.fontWeightMedium,
      color: brand.text,
    }),
    skip: css({ marginTop: theme.spacing(3) }),
  }
}

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
    <PublicShell>
      <h1 className={styles.title}>{t('onboarding.welcome.title', { name: me.display_name })}</h1>
      <p className={styles.subtitle}>{t('onboarding.welcome.subtitle')}</p>

      <div className={styles.next}>
        {unverified ? (
          <Stack direction="column" gap={2}>
            <Alert severity="info" title={t('onboarding.welcome.verifyTitle')}>
              {t('onboarding.welcome.verifyBody')}
            </Alert>
            <Stack gap={1} wrap="wrap">
              <Button icon="envelope" onClick={() => void navigate({ to: '/verify', search: {} })}>
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
              <Button icon="plus" onClick={() => void done(() => navigate({ to: '/tenants/new' }))}>
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

      <section className={styles.how}>
        <h2 className={styles.howTitle}>{t('onboarding.welcome.howTitle')}</h2>
        <ConceptDiagram />
      </section>

      <div className={styles.skip}>
        <Button
          variant="secondary"
          fill="text"
          onClick={() => void done(() => navigate({ to: '/' }))}
        >
          {t('onboarding.welcome.skip')}
        </Button>
      </div>
    </PublicShell>
  )
}
