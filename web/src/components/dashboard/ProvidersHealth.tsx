import { providerQueries } from '@api/queries/results'
import type { ProviderProfile, QuotaReport, TenantLimits } from '@api/types'
import { AppLink } from '@app/AppLink'
import { KeyValueList } from '@components/KeyValueList'
import { StatusBadge } from '@components/StatusBadge'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, Text, Tooltip, useStyles2 } from '@grafana/ui'
import { formatCompact, formatDuration } from '@helpers/format'
import { relativeTime } from '@helpers/time'
import { useQueries } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Panel, PanelEmpty } from './Panel'

const getStyles = (theme: GrafanaTheme2) => ({
  provider: css({
    padding: theme.spacing(1, 2),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.75),
    '& + &': { borderTop: `1px solid ${theme.colors.border.weak}` },
  }),
  head: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1), minWidth: 0 }),
  name: css({
    fontWeight: theme.typography.fontWeightMedium,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    flex: 1,
  }),
  quotas: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))',
    gap: theme.spacing(0.5, 1.5),
  }),
  quota: css({ fontSize: theme.typography.bodySmall.fontSize, minWidth: 0 }),
  quotaHead: css({
    display: 'flex',
    justifyContent: 'space-between',
    gap: theme.spacing(1),
    color: theme.colors.text.secondary,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    span: { overflow: 'hidden', textOverflow: 'ellipsis' },
  }),
  bar: css({
    height: 4,
    borderRadius: 2,
    background: theme.colors.background.secondary,
    overflow: 'hidden',
    marginTop: 2,
    div: { height: '100%' },
  }),
  ok: css({ background: theme.colors.success.main }),
  warn: css({ background: theme.colors.warning.main }),
  bad: css({ background: theme.colors.error.main }),
  note: css({
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    display: 'flex',
    gap: theme.spacing(0.5),
    alignItems: 'center',
  }),
  limits: css({ padding: theme.spacing(1, 2) }),
})

function QuotaBars({ report }: { report: QuotaReport | undefined }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  if (!report) return null
  if (report.unavailable_reason || !report.quotas.length)
    return (
      <div className={styles.note}>
        <Icon name="exclamation-triangle" size="sm" />
        {report.unavailable_reason ?? t('dashboard.providers.unavailable')}
      </div>
    )
  return (
    <>
      <div className={styles.quotas}>
        {report.quotas.slice(0, 6).map((q) => {
          const pct = q.limit > 0 ? Math.min(100, (q.used / q.limit) * 100) : 0
          return (
            <Tooltip
              key={`${q.name}-${q.zone ?? ''}`}
              content={t('dashboard.providers.quotaOf', {
                used: formatCompact(q.used),
                limit: formatCompact(q.limit),
                unit: q.unit ?? '',
              })}
            >
              <div className={styles.quota}>
                <div className={styles.quotaHead}>
                  <span>{q.title ?? q.name}</span>
                  <span>{Math.round(pct)}%</span>
                </div>
                <div className={styles.bar}>
                  <div
                    className={pct >= 90 ? styles.bad : pct >= 70 ? styles.warn : styles.ok}
                    style={{ width: `${pct}%` }}
                  />
                </div>
              </div>
            </Tooltip>
          )
        })}
      </div>
      <div className={styles.note}>
        {report.stale && <Icon name="history" size="sm" />}
        {report.stale ? `${t('dashboard.providers.stale')} · ` : ''}
        {t('dashboard.providers.observed', { when: relativeTime(report.observed_at) })}
      </div>
    </>
  )
}

export function ProvidersHealth({
  providers,
  limits,
  slug,
}: {
  providers: ProviderProfile[]
  limits?: TenantLimits
  slug: string
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const quotas = useQueries({
    queries: providers
      .filter((p) => p.status === 'ready')
      .map((p) => ({ ...providerQueries.quotas(slug, p.id), retry: false })),
  })
  const byId = new Map<string, QuotaReport>()
  providers
    .filter((p) => p.status === 'ready')
    .forEach((p, i) => {
      const d = quotas[i]?.data
      if (d) byId.set(p.id, d)
    })
  return (
    <Panel
      title={t('dashboard.providers.title')}
      right={
        <AppLink to="/t/$slug/settings/providers" params={{ slug }}>
          <Text variant="bodySmall">{t('dashboard.providers.manage')}</Text>
        </AppLink>
      }
      flush
    >
      {providers.length === 0 ? (
        <PanelEmpty>{t('dashboard.providers.empty')}</PanelEmpty>
      ) : (
        providers.map((p) => (
          <div key={p.id} className={styles.provider}>
            <div className={styles.head}>
              <Icon name="cloud" size="sm" />
              <span className={styles.name}>{p.name}</span>
              <Text color="secondary" variant="bodySmall">
                {p.kind}
              </Text>
              <StatusBadge status={p.status} />
            </div>
            {p.status_reason && p.status !== 'ready' && (
              <div className={styles.note}>{p.status_reason}</div>
            )}
            <QuotaBars report={byId.get(p.id)} />
          </div>
        ))
      )}
      {limits && (
        <div className={styles.limits}>
          <KeyValueList
            title={t('dashboard.limits.title')}
            items={[
              { label: t('dashboard.limits.concurrent'), value: limits.max_concurrent_runs },
              { label: t('dashboard.limits.machines'), value: limits.max_machines_per_run },
              { label: t('dashboard.limits.maxSize'), value: limits.max_size },
              { label: t('dashboard.limits.maxKeep'), value: formatDuration(limits.max_keep) },
              { label: t('dashboard.limits.retention'), value: limits.run_retention_max_days },
            ]}
          />
        </div>
      )}
    </Panel>
  )
}
