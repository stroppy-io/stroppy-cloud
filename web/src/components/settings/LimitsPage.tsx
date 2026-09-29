import { settingsQueries } from '@api/queries/settings'
import { KeyValueList } from '@components/KeyValueList'
import { Alert, Badge, Stack, Text } from '@grafana/ui'
import { formatDuration } from '@helpers/format'
import { useTenant } from '@hooks/useTenant'
import { useSuspenseQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

// Read-only for the tenant: platform ceilings the admin can override per tenant.
export function LimitsPage() {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const limits = useSuspenseQuery(settingsQueries.limits(slug)).data
  const override = limits.source === 'tenant_override'
  return (
    <Stack direction="column" gap={2}>
      <Alert severity="info" title={t('settings.limits.aboutTitle')}>
        {t('settings.limits.aboutBody')}
      </Alert>
      <div style={{ maxWidth: 640 }}>
        <KeyValueList
          title={t('settings.limits.title')}
          items={[
            {
              label: t('settings.limits.source'),
              value: (
                <Badge
                  text={
                    override
                      ? t('settings.limits.sourceOverride')
                      : t('settings.limits.sourceDefault')
                  }
                  color={override ? 'purple' : 'darkgrey'}
                  icon={override ? 'shield' : 'cog'}
                />
              ),
            },
            { label: t('settings.limits.maxConcurrentRuns'), value: limits.max_concurrent_runs },
            { label: t('settings.limits.maxMachinesPerRun'), value: limits.max_machines_per_run },
            { label: t('settings.limits.maxSize'), value: limits.max_size },
            { label: t('settings.limits.maxKeep'), value: formatDuration(limits.max_keep) },
            {
              label: t('settings.limits.retentionMax'),
              value: t('settings.limits.days', { count: limits.run_retention_max_days }),
            },
          ]}
        />
      </div>
      <Text color="secondary" variant="bodySmall">
        {override ? t('settings.limits.overrideNote') : t('settings.limits.defaultNote')}
      </Text>
    </Stack>
  )
}
