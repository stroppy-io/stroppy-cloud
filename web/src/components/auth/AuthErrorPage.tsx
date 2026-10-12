import { AuthCard } from '@components/auth/AuthCard'
import { Alert, Button, LoadingPlaceholder, Text } from '@grafana/ui'
import { flowErrorDetails } from '@lib/auth'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

type Shown = { title: string; detail?: string }

/** Kratos's user-facing error page (`selfservice.flows.error.ui_url`): the error by id. */
export function AuthErrorPage({ id }: { id?: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [shown, setShown] = useState<Shown>()
  useEffect(() => {
    if (!id) {
      setShown({ title: t('auth.error.unknown') })
      return
    }
    flowErrorDetails(id)
      .then((e) =>
        setShown(
          e.status === 404
            ? { title: t('auth.error.notFound') }
            : {
                title: e.id
                  ? t(`auth.kratosError.${e.id}`, { defaultValue: t('auth.error.unknown') })
                  : t('auth.error.unknown'),
                // The service's own words, for whoever reports the problem.
                detail: e.text,
              }
        )
      )
      .catch(() => setShown({ title: t('auth.kratosError.network') }))
  }, [id, t])
  return (
    <AuthCard title={t('auth.error.title')}>
      {shown === undefined ? (
        <LoadingPlaceholder text={t('common.misc.loading')} />
      ) : (
        <Alert severity="error" title={shown.title}>
          {shown.detail && (
            <Text variant="bodySmall" color="secondary">
              {shown.detail}
            </Text>
          )}
        </Alert>
      )}
      <Button onClick={() => void navigate({ to: '/login' })}>{t('auth.error.toLogin')}</Button>
    </AuthCard>
  )
}
