import { Alert } from '@grafana/ui'
import { useTranslation } from 'react-i18next'

// Shown to roles that can look but not touch a settings tab.
export function ReadOnlyNotice({ requiredRole = 'admin' }: { requiredRole?: string }) {
  const { t } = useTranslation()
  return (
    <Alert severity="info" title={t('settings.readOnly.title')}>
      {t('settings.readOnly.body', { role: t(`common.role.${requiredRole}`) })}
    </Alert>
  )
}
