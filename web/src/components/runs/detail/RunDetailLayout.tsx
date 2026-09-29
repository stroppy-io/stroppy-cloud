import type { Run } from '@api/types'
import { AppLink } from '@app/AppLink'
import { ErrorState } from '@app/ErrorState'
import { Page, PageFill } from '@app/Page'
import { PageHeader, type TabDef } from '@app/PageHeader'
import { RelativeTime } from '@components/RelativeTime'
import { RunProgress } from '@components/runs/RunProgress'
import { StatusBadge } from '@components/StatusBadge'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Icon, LoadingPlaceholder, Stack, Text, Tooltip, useStyles2 } from '@grafana/ui'
import { formatDuration } from '@helpers/format'
import { isTerminal } from '@helpers/run-status'
import { formatDateTime } from '@helpers/time'
import { useRun } from '@hooks/useRun'
import { useTenant } from '@hooks/useTenant'
import { useSearch } from '@tanstack/react-router'
import { type ReactNode, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { KeepBadge } from './KeepBadge'
import { RunActions } from './RunActions'
import { pickTelemetrySearch } from './telemetry-search'

const getStyles = (theme: GrafanaTheme2) => ({
  meta: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    flexWrap: 'wrap',
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    marginTop: theme.spacing(0.5),
  }),
  metaItem: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(0.5) }),
  progress: css({ maxWidth: 720, marginTop: theme.spacing(1.5) }),
  reason: css({ marginTop: theme.spacing(1.5) }),
})

function LiveDuration({ run }: { run: Run }) {
  const [, tick] = useState(0)
  const live = !isTerminal(run.status) && !!run.started_at
  useEffect(() => {
    if (!live) return
    const id = window.setInterval(() => tick((n) => n + 1), 1000)
    return () => window.clearInterval(id)
  }, [live])
  if (run.duration) return <span>{formatDuration(run.duration)}</span>
  if (!run.started_at) return <span>—</span>
  return <span>{formatDuration((Date.now() - new Date(run.started_at).getTime()) / 1000)}</span>
}

// Layout for /t/$slug/runs/$id: header (status, phase, progress, keep, actions) + tabs.
// Children = the active tab route.
export function RunDetailLayout({ id, children }: { id: string; children: ReactNode }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug } = useTenant()
  const q = useRun(slug, id)
  const run = q.data
  const params = { slug, id }
  // The telemetry window (`from`/`to`/`refresh`) survives switching between Events / Logs /
  // Metrics; tab-specific filters do not.
  const currentSearch = useSearch({ strict: false }) as Record<string, unknown>
  const telemetry = pickTelemetrySearch(currentSearch)

  if (q.isPending)
    return (
      <Page width="wide">
        <LoadingPlaceholder text={t('common.misc.loading')} />
      </Page>
    )
  if (q.isError || !run)
    return (
      <Page width="wide">
        <ErrorState error={q.error} onRetry={() => void q.refetch()} />
      </Page>
    )

  const tabs: TabDef[] = [
    {
      id: 'overview',
      label: t('runs.tabs.overview'),
      icon: 'apps',
      to: '/t/$slug/runs/$id',
      params,
    },
    {
      id: 'topology',
      label: t('runs.tabs.topology'),
      icon: 'sitemap',
      to: '/t/$slug/runs/$id/topology',
      params,
    },
    {
      id: 'events',
      label: t('runs.tabs.events'),
      icon: 'list-ul',
      to: '/t/$slug/runs/$id/events',
      params,
      search: telemetry,
    },
    {
      id: 'logs',
      label: t('runs.tabs.logs'),
      icon: 'file-alt',
      to: '/t/$slug/runs/$id/logs',
      params,
      search: telemetry,
    },
    {
      id: 'metrics',
      label: t('runs.tabs.metrics'),
      icon: 'chart-line',
      to: '/t/$slug/runs/$id/metrics',
      params,
      search: telemetry,
    },
    {
      id: 'artifacts',
      label: t('runs.tabs.artifacts'),
      icon: 'download-alt',
      to: '/t/$slug/runs/$id/artifacts',
      params,
      counter: run.result?.artifacts?.length || undefined,
    },
    {
      id: 'spec',
      label: t('runs.tabs.spec'),
      icon: 'brackets-curly',
      to: '/t/$slug/runs/$id/spec',
      params,
    },
  ]

  return (
    <Page width="wide" fill>
      <PageHeader
        breadcrumbs={[
          { label: t('runs.title'), to: '/t/$slug/runs', params: { slug } },
          { label: run.name },
        ]}
        title={run.name}
        badge={
          <Stack gap={1} alignItems="center">
            <StatusBadge status={run.status} />
            <KeepBadge run={run} />
            {run.trigger !== 'manual' && (
              <Tooltip content={t('runs.detail.triggeredBy')}>
                <span>
                  <Text color="secondary" variant="bodySmall">
                    <Icon
                      name={
                        run.trigger === 'schedule'
                          ? 'clock-nine'
                          : run.trigger === 'suite'
                            ? 'layer-group'
                            : 'plug'
                      }
                      size="sm"
                    />{' '}
                    {t(`common.trigger.${run.trigger}`)}
                  </Text>
                </span>
              </Tooltip>
            )}
          </Stack>
        }
        subtitle={
          <>
            <div className={styles.meta}>
              <span className={styles.metaItem}>
                <Icon name="database" size="sm" />
                {run.summary?.db_kind} {run.summary?.db_version}
                {run.summary?.topology_label ? ` · ${run.summary.topology_label}` : ''}
                {run.summary?.node_count ? ` · ${run.summary.node_count}n` : ''}
              </span>
              <span className={styles.metaItem}>
                <Icon name="rocket" size="sm" />
                {run.summary?.workload_name ?? run.snapshot.workload_name}
              </span>
              <span className={styles.metaItem}>
                <Icon name="cloud" size="sm" />
                {run.summary?.provider_profile?.name ?? run.snapshot.provider_profile.name}
              </span>
              <span className={styles.metaItem}>
                <Icon name="user" size="sm" />
                {run.author.display_name}
              </span>
              <span className={styles.metaItem}>
                <Icon name="clock-nine" size="sm" />
                {run.started_at ? (
                  <Tooltip content={formatDateTime(run.started_at)}>
                    <span>
                      {t('common.fields.started')} <RelativeTime value={run.started_at} />
                    </span>
                  </Tooltip>
                ) : (
                  <span>
                    {t('common.fields.created')} <RelativeTime value={run.created_at} />
                  </span>
                )}
                {' · '}
                <LiveDuration run={run} />
              </span>
              {run.test_ref.name && (
                <span className={styles.metaItem}>
                  <Icon name="vial" size="sm" />
                  <AppLink
                    to="/t/$slug/library/tests/$id"
                    params={{ slug, id: run.test_ref.id }}
                    plain
                  >
                    {run.test_ref.name}
                  </AppLink>
                </span>
              )}
            </div>
            {!isTerminal(run.status) && (
              <div className={styles.progress}>
                <RunProgress run={run} />
              </div>
            )}
            {run.status_reason && (run.status === 'failed' || run.status === 'cancelled') && (
              <div className={styles.reason}>
                <Alert
                  severity={run.status === 'failed' ? 'error' : 'info'}
                  title={
                    run.status === 'failed'
                      ? t('runs.detail.failedReason')
                      : t('runs.detail.cancelledReason')
                  }
                  bottomSpacing={0}
                >
                  {run.status_reason}
                </Alert>
              </div>
            )}
          </>
        }
        actions={<RunActions run={run} />}
        tabs={tabs}
      />
      <PageFill scroll>{children}</PageFill>
    </Page>
  )
}
