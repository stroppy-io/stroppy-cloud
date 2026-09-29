import { runQueries } from '@api/queries/runs'
import type { Run, RunOverview } from '@api/types'
import { AppLink } from '@app/AppLink'
import { ErrorState } from '@app/ErrorState'
import { SectionTitle } from '@app/PageHeader'
import { RelativeTime } from '@components/RelativeTime'
import { TopologyMini } from '@components/runs/topology/TopologyView'
import { StatusBadge } from '@components/StatusBadge'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Badge, LoadingPlaceholder, Stack, Text, useStyles2 } from '@grafana/ui'
import { formatMetric } from '@helpers/format'
import { isTerminal } from '@helpers/run-status'
import { useRun } from '@hooks/useRun'
import { useTenant } from '@hooks/useTenant'
import { useTopic } from '@hooks/useTopic'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { PhaseTimeline } from './PhaseTimeline'
import { RunSidebar } from './RunSidebar'
import { ComponentsTable, MachinesTable, SegmentsTable } from './StandTables'

const getStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) 320px',
    gap: theme.spacing(3),
    alignItems: 'start',
    [theme.breakpoints.down('lg')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  main: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3), minWidth: 0 }),
  section: css({ minWidth: 0, '& > *': { minWidth: 0 } }),
  side: css({
    position: 'sticky',
    top: 48 + 16,
    [theme.breakpoints.down('lg')]: { position: 'static' },
  }),
  panel: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(2),
  }),
  stats: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))',
    gridAutoRows: theme.spacing(10),
    gap: theme.spacing(1.5),
  }),
  stat: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(1.5, 2),
    minWidth: 0,
    overflow: 'hidden',
    display: 'flex',
    flexDirection: 'column',
    justifyContent: 'space-between',
  }),
  statLabel: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    textTransform: 'uppercase',
    letterSpacing: 0.4,
  }),
  statValue: css({
    fontSize: theme.typography.h3.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    fontVariantNumeric: 'tabular-nums',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  reasons: css({ margin: theme.spacing(0.5, 0, 0), paddingLeft: theme.spacing(2) }),
  sourceRow: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
})

function ResultStats({ run }: { run: Run }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const m = run.result?.metrics
  const h = run.summary?.headline
  // Run-level qps (the segments weighted by their measured time); old results only carry tps.
  const qps = h?.qps
  const tps = m?.tps?.value ?? h?.tps
  const throughput =
    qps !== undefined
      ? { label: 'QPS', value: formatMetric(qps, 'count') }
      : { label: 'TPS', value: formatMetric(tps, 'tps') }
  const p99 = m?.latency_p99_ms?.value ?? h?.latency_p99_ms
  const p95 = m?.latency_p95_ms?.value
  const p50 = m?.latency_p50_ms?.value
  const errors = m?.errors?.value ?? h?.errors
  if (qps === undefined && tps === undefined && p99 === undefined) return null
  const cells: { label: string; value: string }[] = [
    throughput,
    { label: 'p99', value: formatMetric(p99, 'ms') },
    ...(p95 !== undefined ? [{ label: 'p95', value: formatMetric(p95, 'ms') }] : []),
    ...(p50 !== undefined ? [{ label: 'p50', value: formatMetric(p50, 'ms') }] : []),
    { label: t('runs.overview.result.errors'), value: formatMetric(errors, 'count') },
  ]
  return (
    <div className={styles.stats}>
      {cells.map((c) => (
        <div key={c.label} className={styles.stat}>
          <div className={styles.statLabel}>{c.label}</div>
          <div className={styles.statValue}>{c.value}</div>
        </div>
      ))}
      {run.result?.baseline && (
        <div className={styles.stat}>
          <div className={styles.statLabel}>{t('runs.overview.result.baseline')}</div>
          <div className={styles.statValue}>
            <StatusBadge
              status={run.result.baseline.ok ? 'ok' : 'failed'}
              label={run.result.baseline.ok ? t('common.status.ok') : t('common.status.failed')}
            />
          </div>
        </div>
      )}
    </div>
  )
}

export function RunOverviewTab({ id }: { id: string }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const runQ = useRun(slug, id)
  const run = runQ.data
  const terminal = isTerminal(run?.status)
  const ov = useQuery({
    ...runQueries.overview(slug, id),
    refetchInterval: terminal ? false : 10_000,
  })
  useTopic<RunOverview, RunOverview>({
    topic: `run.overview/${id}`,
    queryKey: runQueries.overview(slug, id).queryKey,
    enabled: !!run && !terminal,
    merge: (_prev, payload) => payload,
  })

  if (!run) return null
  if (ov.isPending) return <LoadingPlaceholder text={t('common.misc.loading')} />
  if (ov.isError) return <ErrorState error={ov.error} onRetry={() => void ov.refetch()} compact />
  const o = ov.data

  return (
    <div className={styles.grid}>
      <div className={styles.main}>
        {o.source !== 'live' && !terminal && (
          <Alert
            severity={o.source === 'synthetic' ? 'info' : 'warning'}
            title={t(`runs.overview.source.${o.source}Title`)}
          >
            {t(`runs.overview.source.${o.source}`)}
            {o.degraded_reasons?.length ? (
              <ul className={styles.reasons}>
                {o.degraded_reasons.map((r) => (
                  <li key={r}>{r}</li>
                ))}
              </ul>
            ) : null}
          </Alert>
        )}
        {o.source === 'live' && o.degraded_reasons?.length ? (
          <Alert severity="warning" title={t('runs.overview.source.degradedTitle')}>
            <ul className={styles.reasons}>
              {o.degraded_reasons.map((r) => (
                <li key={r}>{r}</li>
              ))}
            </ul>
          </Alert>
        ) : null}
        {o.pending_activity?.activity && !terminal && (
          <Alert severity="info" title={t('runs.overview.pending.title')}>
            <code>{o.pending_activity.activity}</code>
            {o.pending_activity.attempt && o.pending_activity.attempt > 1
              ? ` · ${t('runs.events.attempt', { n: o.pending_activity.attempt })}`
              : ''}
            {o.pending_activity.since ? (
              <>
                {' · '}
                {t('runs.overview.pending.since')}{' '}
                <RelativeTime value={o.pending_activity.since} refresh={5000} />
              </>
            ) : null}
            {o.pending_activity.last_failure && (
              <div>
                <Text color="error" variant="bodySmall">
                  {o.pending_activity.last_failure}
                </Text>
              </div>
            )}
          </Alert>
        )}
        <ResultStats run={run} />
        <section className={styles.panel}>
          <Stack direction="column" gap={2}>
            <SectionTitle
              right={
                <span className={styles.sourceRow}>
                  <Badge
                    text={t(`common.status.${o.source}`)}
                    color={
                      o.source === 'live'
                        ? 'green'
                        : o.source === 'persisted'
                          ? 'darkgrey'
                          : 'orange'
                    }
                    icon={o.source === 'live' ? 'sync' : undefined}
                  />
                  <span>
                    {t('runs.overview.observed')}{' '}
                    <RelativeTime value={o.observed_at} refresh={5000} />
                  </span>
                </span>
              }
            >
              {t('runs.overview.phases')}
            </SectionTitle>
            <PhaseTimeline phases={o.phases} />
          </Stack>
        </section>
        <section>
          <Stack direction="column" gap={1}>
            <SectionTitle
              right={
                <AppLink to="/t/$slug/runs/$id/topology" params={{ slug, id: run.id }}>
                  {t('runs.tabs.topology')}
                </AppLink>
              }
            >
              {t('runs.topology.title')}
            </SectionTitle>
            <TopologyMini id={run.id} height={280} />
          </Stack>
        </section>
        {(o.workload_segments?.length ?? 0) > 0 && (
          <Stack direction="column" gap={1}>
            <SectionTitle
              right={
                <Text color="secondary" variant="bodySmall">
                  {t('runs.overview.snapshot.segmentsCount', {
                    count: o.workload_segments?.length ?? 0,
                  })}
                </Text>
              }
            >
              {t('runs.overview.segments.title')}
            </SectionTitle>
            <div className={styles.section}>
              <SegmentsTable segments={o.workload_segments ?? []} />
            </div>
          </Stack>
        )}
        <Stack direction="column" gap={1}>
          <SectionTitle
            right={
              <Text color="secondary" variant="bodySmall">
                {t('runs.overview.components.count', { count: o.components.length })}
              </Text>
            }
          >
            {t('runs.overview.components.title')}
          </SectionTitle>
          <div className={styles.section}>
            <ComponentsTable components={o.components} />
          </div>
        </Stack>
        <Stack direction="column" gap={1}>
          <SectionTitle
            right={
              <Text color="secondary" variant="bodySmall">
                {t('runs.overview.machines.online', {
                  online: o.machines.filter((m) => m.presence === 'online').length,
                  total: o.machines.length,
                })}
              </Text>
            }
          >
            {t('runs.overview.machines.title')}
          </SectionTitle>
          <div className={styles.section}>
            <MachinesTable machines={o.machines} />
          </div>
        </Stack>
      </div>
      <aside className={styles.side}>
        <RunSidebar run={run} />
      </aside>
    </div>
  )
}
