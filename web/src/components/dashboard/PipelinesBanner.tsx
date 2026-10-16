import type { TenantDashboard } from '@api/types'
import { Alert } from '@grafana/ui'
import { useTranslation } from 'react-i18next'

// A tenant without this server's pipelines cannot start runs yet: a new tenant gets them in a few
// minutes, every tenant after a server upgrade. Launches are accepted and wait in the queue.
export function PipelinesBanner({ pipelines }: { pipelines: TenantDashboard['pipelines'] }) {
  const { t } = useTranslation()
  if (pipelines.status === 'synced') return null
  if (pipelines.status === 'failed')
    return (
      <Alert severity="error" title={t('dashboard.pipelines.failedTitle')}>
        {t('dashboard.pipelines.failed', { error: pipelines.error ?? '' })}
      </Alert>
    )
  return (
    <Alert severity="info" title={t(`dashboard.pipelines.${pipelines.status}Title`)}>
      {t('dashboard.pipelines.progress', { pushed: pipelines.pushed, total: pipelines.total })}
    </Alert>
  )
}
