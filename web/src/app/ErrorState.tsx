import { isApiError } from '@api/errors'
import { Alert, Button, EmptyState, Stack, Text } from '@grafana/ui'
import { useTranslation } from 'react-i18next'

// Uniform error rendering for queries and route errors.
export function ErrorState({
  error,
  onRetry,
  compact,
}: {
  error: unknown
  onRetry?: () => void
  compact?: boolean
}) {
  const { t } = useTranslation()
  const api = isApiError(error) ? error : undefined
  const title = api
    ? api.status === 404
      ? t('common.errors.notFound')
      : api.status === 403
        ? t('common.errors.forbidden')
        : api.status === 401
          ? t('common.errors.unauthorized')
          : api.title
    : error instanceof TypeError
      ? t('common.errors.network')
      : t('common.errors.generic')
  const detail = api?.detail ?? (error instanceof Error && !api ? error.message : undefined)
  if (compact) {
    return (
      <Alert
        severity="error"
        title={title}
        buttonContent={onRetry ? t('common.actions.retry') : undefined}
        onRemove={onRetry}
      >
        {detail}
        {api?.problem?.request_id && (
          <Text color="secondary" variant="bodySmall">
            {' '}
            · {t('common.errors.requestId')}: {api.problem.request_id}
          </Text>
        )}
      </Alert>
    )
  }
  return (
    <EmptyState
      variant="not-found"
      message={title}
      button={
        onRetry ? (
          <Button icon="sync" onClick={onRetry}>
            {t('common.actions.retry')}
          </Button>
        ) : undefined
      }
    >
      <Stack direction="column" alignItems="center" gap={0.5}>
        {detail && <Text color="secondary">{detail}</Text>}
        {api && (
          <Text color="secondary" variant="bodySmall">
            {api.code}
            {api.problem?.request_id
              ? ` · ${t('common.errors.requestId')}: ${api.problem.request_id}`
              : ''}
          </Text>
        )}
      </Stack>
    </EmptyState>
  )
}
