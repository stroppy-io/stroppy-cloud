import { settingsMutations, settingsQueries } from '@api/queries/settings'
import type { ProviderProfile, QuotaReport } from '@api/types'
import { ErrorState } from '@app/ErrorState'
import { SectionTitle } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { RelativeTime } from '@components/RelativeTime'
import { StatusBadge } from '@components/StatusBadge'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Badge, Button, LoadingPlaceholder, Stack, Text, useStyles2 } from '@grafana/ui'
import { formatNumber, formatPercent } from '@helpers/format'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { KIND_LABEL } from './ProvidersPage'

const getStyles = (theme: GrafanaTheme2) => ({
  section: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(2),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1.5),
    scrollMarginTop: theme.spacing(8),
  }),
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))',
    gap: theme.spacing(1.5, 3),
  }),
  quota: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5), minWidth: 0 }),
  row: css({
    display: 'flex',
    justifyContent: 'space-between',
    gap: theme.spacing(1),
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  bar: css({
    height: 8,
    borderRadius: 4,
    background: theme.colors.background.secondary,
    overflow: 'hidden',
    div: { height: '100%', borderRadius: 4, transition: 'width .3s' },
  }),
  ok: css({ div: { background: theme.colors.success.main } }),
  warn: css({ div: { background: theme.colors.warning.main } }),
  err: css({ div: { background: theme.colors.error.main } }),
  pct: css({ fontVariantNumeric: 'tabular-nums' }),
  logo: css({
    width: 28,
    height: 28,
    borderRadius: theme.shape.radius.default,
    display: 'inline-grid',
    placeItems: 'center',
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    fontWeight: theme.typography.fontWeightBold,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
})

export function QuotasPage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug } = useTenant()
  const providers = useSuspenseQuery(settingsQueries.providers(slug)).data.data
  return (
    <Stack direction="column" gap={2}>
      <Text color="secondary">{t('settings.quotas.hint')}</Text>
      {providers.length === 0 && (
        <Alert severity="info" title={t('settings.quotas.noProviders')}>
          {t('settings.providers.emptyHint')}
        </Alert>
      )}
      {providers.map((p) => (
        <ProviderQuotas key={p.id} provider={p} styles={styles} />
      ))}
    </Stack>
  )
}

function ProviderQuotas({
  provider: p,
  styles,
}: {
  provider: ProviderProfile
  styles: ReturnType<typeof getStyles>
}) {
  const { t } = useTranslation()
  const { slug, can } = useTenant()
  const qc = useQueryClient()
  const quotas = useQuery({ ...settingsQueries.quotas(slug, p.id), refetchInterval: 60_000 })
  const refresh = useMutation({
    mutationFn: () => settingsMutations.refreshQuotas(slug, p.id),
    onSuccess: (data) => {
      qc.setQueryData(settingsQueries.quotas(slug, p.id).queryKey, data)
      void qc.invalidateQueries({ queryKey: settingsQueries.providers(slug).queryKey })
      toast.success(t('settings.quotas.refreshed', { name: p.name }))
    },
    onError: (e) => toast.error(e),
  })
  const report: QuotaReport | undefined = quotas.data
  return (
    <section id={p.id} className={styles.section} aria-label={p.name}>
      <SectionTitle
        right={
          <Stack gap={1} alignItems="center">
            {report?.observed_at && (
              <Text color="secondary" variant="bodySmall">
                {t('settings.quotas.observed')} <RelativeTime value={report.observed_at} />
              </Text>
            )}
            {report?.stale && report.observed_at && (
              <Badge
                text={t('common.status.stale')}
                color="orange"
                tooltip={t('settings.providers.staleHint')}
              />
            )}
            {can('view') && (
              <Button
                size="sm"
                variant="secondary"
                icon="sync"
                disabled={p.status !== 'ready' || refresh.isPending}
                onClick={() => refresh.mutate()}
              >
                {t('common.actions.refresh')}
              </Button>
            )}
          </Stack>
        }
      >
        <Stack gap={1} alignItems="center">
          <span className={styles.logo} aria-hidden>
            {KIND_LABEL[p.kind] ?? p.kind}
          </span>
          {p.name}
          <StatusBadge status={p.status} />
          {report?.scope && (
            <Text color="secondary" variant="bodySmall">
              {report.scope}
            </Text>
          )}
        </Stack>
      </SectionTitle>
      {quotas.isPending && <LoadingPlaceholder text={t('common.misc.loading')} />}
      {quotas.isError && (
        <ErrorState compact error={quotas.error} onRetry={() => void quotas.refetch()} />
      )}
      {report && (report.unavailable_reason || report.quotas.length === 0) && (
        <Alert
          severity={p.status === 'failed' ? 'error' : 'warning'}
          title={t('settings.quotas.unavailable')}
        >
          {report.unavailable_reason ?? t('settings.quotas.unavailableGeneric')}
          {p.status === 'failed' && p.status_reason ? ` — ${p.status_reason}` : ''}
        </Alert>
      )}
      {report && report.quotas.length > 0 && (
        <div className={styles.grid}>
          {report.quotas.map((q) => {
            const pct = q.limit > 0 ? (q.used / q.limit) * 100 : 0
            const tone = pct >= 95 ? styles.err : pct >= 80 ? styles.warn : styles.ok
            return (
              <div key={`${q.name}:${q.zone ?? ''}`} className={styles.quota}>
                <div className={styles.row}>
                  <span title={q.name}>
                    <Text weight="medium">{q.title ?? q.name}</Text>
                    {q.zone && (
                      <Text color="secondary" variant="bodySmall">
                        {' '}
                        · {q.zone}
                      </Text>
                    )}
                  </span>
                  <span className={styles.pct}>
                    <Text color={pct >= 95 ? 'error' : pct >= 80 ? 'warning' : 'secondary'}>
                      {formatPercent(pct, 0)}
                    </Text>
                  </span>
                </div>
                <div
                  className={cx(styles.bar, tone)}
                  role="progressbar"
                  aria-valuenow={Math.round(pct)}
                  aria-valuemin={0}
                  aria-valuemax={100}
                  aria-label={q.title ?? q.name}
                >
                  <div style={{ width: `${Math.min(100, pct)}%` }} />
                </div>
                <div className={styles.row}>
                  <Text color="secondary" variant="bodySmall">
                    {t('settings.quotas.used', {
                      used: formatNumber(q.used, 1),
                      limit: formatNumber(q.limit, 1),
                      unit: q.unit ?? '',
                    })}
                  </Text>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </section>
  )
}
