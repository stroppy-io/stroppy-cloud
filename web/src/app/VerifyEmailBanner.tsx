import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Icon, Text, useStyles2 } from '@grafana/ui'
import { tokenClaims } from '@lib/auth'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  bar: css({
    flex: '0 0 auto',
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(0.5, 2),
    background: theme.colors.warning.transparent,
    borderBottom: `1px solid ${theme.colors.border.weak}`,
    color: theme.colors.warning.text,
  }),
})

/**
 * Kratos mode with an unconfirmed address: invites and platform admin
 * match only confirmed e-mails, so the account is half-working until then.
 */
export function VerifyEmailBanner() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const navigate = useNavigate()
  const claims = tokenClaims()
  if (claims.emailVerified !== false) return null
  return (
    <div className={styles.bar} role="status">
      <Icon name="exclamation-triangle" />
      <Text variant="bodySmall">{t('auth.verify.banner', { email: claims.email ?? '' })}</Text>
      <Button
        size="sm"
        variant="secondary"
        fill="text"
        onClick={() => void navigate({ to: '/verify', search: {} })}
      >
        {t('auth.verify.bannerAction')}
      </Button>
    </div>
  )
}
