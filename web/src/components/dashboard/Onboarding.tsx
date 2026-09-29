import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Icon, Text, useStyles2 } from '@grafana/ui'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  wrap: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(3),
    maxWidth: 760,
    margin: `${theme.spacing(2)} auto 0`,
  }),
  steps: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
    marginTop: theme.spacing(3),
  }),
  step: css({
    display: 'grid',
    gridTemplateColumns: '32px minmax(0, 1fr) auto',
    gap: theme.spacing(2),
    alignItems: 'start',
  }),
  num: css({
    width: 32,
    height: 32,
    borderRadius: '50%',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    border: `1px solid ${theme.colors.border.medium}`,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.secondary,
  }),
  done: css({
    background: theme.colors.success.transparent,
    borderColor: theme.colors.success.border,
    color: theme.colors.success.text,
  }),
  hint: css({ color: theme.colors.text.secondary, marginTop: theme.spacing(0.25) }),
  actions: css({ display: 'flex', gap: theme.spacing(1), flexWrap: 'wrap' }),
})

// Empty-tenant state: add provider → create test → launch.
export function Onboarding({
  slug,
  tenantName,
  hasProvider,
}: {
  slug: string
  tenantName: string
  hasProvider: boolean
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const navigate = useNavigate()
  return (
    <div className={styles.wrap}>
      <Text element="h2" variant="h3">
        {t('dashboard.onboarding.title', { tenant: tenantName })}
      </Text>
      <div className={styles.hint}>{t('dashboard.onboarding.subtitle')}</div>
      <div className={styles.steps}>
        <div className={styles.step}>
          <span className={cx(styles.num, hasProvider && styles.done)}>
            {hasProvider ? <Icon name="check" /> : '1'}
          </span>
          <div>
            <Text weight="medium">{t('dashboard.onboarding.provider.title')}</Text>
            <div className={styles.hint}>
              {hasProvider
                ? t('dashboard.onboarding.done')
                : t('dashboard.onboarding.provider.hint')}
            </div>
          </div>
          <div className={styles.actions}>
            <Button
              variant={hasProvider ? 'secondary' : 'primary'}
              size="sm"
              icon="cloud"
              onClick={() => void navigate({ to: '/t/$slug/settings/providers', params: { slug } })}
            >
              {t('dashboard.onboarding.provider.cta')}
            </Button>
          </div>
        </div>
        <div className={styles.step}>
          <span className={styles.num}>2</span>
          <div>
            <Text weight="medium">{t('dashboard.onboarding.test.title')}</Text>
            <div className={styles.hint}>{t('dashboard.onboarding.test.hint')}</div>
          </div>
          <div className={styles.actions}>
            <Button
              variant={hasProvider ? 'primary' : 'secondary'}
              size="sm"
              icon="plus"
              onClick={() => void navigate({ to: '/t/$slug/library/tests/new', params: { slug } })}
            >
              {t('dashboard.onboarding.test.cta')}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              icon="book-open"
              onClick={() => void navigate({ to: '/examples' })}
            >
              {t('dashboard.onboarding.test.example')}
            </Button>
          </div>
        </div>
        <div className={styles.step}>
          <span className={styles.num}>3</span>
          <div>
            <Text weight="medium">{t('dashboard.onboarding.launch.title')}</Text>
            <div className={styles.hint}>{t('dashboard.onboarding.launch.hint')}</div>
          </div>
          <div className={styles.actions}>
            <Button
              variant="secondary"
              size="sm"
              icon="play"
              onClick={() => void navigate({ to: '/t/$slug/library/tests', params: { slug } })}
            >
              {t('dashboard.onboarding.launch.cta')}
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
