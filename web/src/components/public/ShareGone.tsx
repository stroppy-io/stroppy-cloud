import { isApiError } from '@api/errors'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Icon, Text, useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  wrap: css({
    maxWidth: 520,
    margin: `${theme.spacing(10)} auto`,
    padding: theme.spacing(4),
    textAlign: 'center',
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: theme.spacing(1.5),
  }),
  icon: css({ color: theme.colors.text.secondary }),
})

// Friendly page for revoked / expired / unknown share tokens.
export function ShareGone({ error }: { error: unknown }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const gone = isApiError(error) && error.code === 'share_gone'
  const notFound = isApiError(error) && error.status === 404 && !gone
  return (
    <div className={styles.wrap}>
      <Icon name={gone ? 'lock' : 'search'} size="xxl" className={styles.icon} />
      <Text element="h1" variant="h2">
        {t('public.gone.title')}
      </Text>
      <Text color="secondary">
        {notFound
          ? t('public.gone.notFound')
          : gone
            ? t('public.gone.hint')
            : error instanceof Error
              ? error.message
              : t('common.errors.generic')}
      </Text>
      <Button variant="secondary" onClick={() => window.location.assign('/')}>
        {t('public.gone.home')}
      </Button>
    </div>
  )
}
