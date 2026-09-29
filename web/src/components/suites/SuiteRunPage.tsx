import { suiteRunMutations, suiteRunQueries } from '@api/queries/suites'
import type { SuiteRun } from '@api/types'
import { AppLink } from '@app/AppLink'
import { ErrorState } from '@app/ErrorState'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { ConfirmAction } from '@components/ConfirmAction'
import { KeyValueList } from '@components/KeyValueList'
import { RelativeTime } from '@components/RelativeTime'
import { StatusBadge } from '@components/StatusBadge'
import { TagsView } from '@components/TagsEditor'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Icon, Stack, Tab, TabContent, TabsBar, Text, useStyles2 } from '@grafana/ui'
import { elapsedSince, formatDuration } from '@helpers/format'
import { isTerminal } from '@helpers/run-status'
import { formatDateTime } from '@helpers/time'
import { useTenant } from '@hooks/useTenant'
import { useTopic } from '@hooks/useTopic'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { SuiteRunCellsTab } from './SuiteRunCellsTab'
import { SuiteRunProgressBar } from './SuiteRunProgressBar'
import { SuiteRunShareDialog } from './SuiteRunShareDialog'
import { SuiteRunSummaryTab } from './SuiteRunSummaryTab'

export type SuiteRunTab = 'summary' | 'cells'
export const SUITE_RUN_TABS: SuiteRunTab[] = ['summary', 'cells']

const getStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) 280px',
    gap: theme.spacing(3),
    marginTop: theme.spacing(2),
    [theme.breakpoints.down('md')]: { gridTemplateColumns: '1fr' },
  }),
  progress: css({
    padding: theme.spacing(2),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1),
    marginBottom: theme.spacing(2),
  }),
  big: css({
    fontSize: theme.typography.h3.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
  }),
})

// Ticks once a second while the run is live so elapsed time moves.
function useLiveDuration(sr: SuiteRun | undefined): string {
  const live = !!sr && !isTerminal(sr.status)
  const [, tick] = useState(0)
  useEffect(() => {
    if (!live) return
    const id = window.setInterval(() => tick((n) => n + 1), 1000)
    return () => window.clearInterval(id)
  }, [live])
  if (!sr) return '—'
  if (sr.duration) return formatDuration(sr.duration)
  return formatDuration(elapsedSince(sr.started_at ?? sr.created_at))
}

export function SuiteRunPage({
  id,
  tab,
  onTabChange,
}: {
  id: string
  tab: SuiteRunTab
  onTabChange: (t: SuiteRunTab) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug, can } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const q = useQuery({
    ...suiteRunQueries.detail(slug, id),
    refetchInterval: (query) =>
      query.state.data && !isTerminal(query.state.data.status) ? 5000 : false,
  })
  const sr = q.data
  useTopic<SuiteRun>({
    topic: `suite_run/${id}`,
    queryKey: suiteRunQueries.detail(slug, id).queryKey,
    enabled: !!sr && !isTerminal(sr.status),
  })
  const duration = useLiveDuration(sr)
  const [sharing, setSharing] = useState(false)
  const invalidate = () => {
    void qc.invalidateQueries({ queryKey: ['t', slug, 'suite-runs'] })
    void qc.invalidateQueries({ queryKey: ['t', slug, 'runs'] })
  }
  const onError = (e: unknown) => toast.error(e)
  const cancel = useMutation({
    mutationFn: () => suiteRunMutations.cancel(slug, id),
    onSuccess: (next) => {
      qc.setQueryData(suiteRunQueries.detail(slug, id).queryKey, next)
      toast.success(t('suites.suiteRuns.toasts.cancelled'))
      invalidate()
    },
    onError,
  })
  const retry = useMutation({
    mutationFn: () => suiteRunMutations.retryFailed(slug, id),
    onSuccess: (next) => {
      toast.success(t('suites.suiteRuns.toasts.retried'), { description: next.name })
      invalidate()
      void navigate({ to: '/t/$slug/suite-runs/$id', params: { slug, id: next.id } })
    },
    onError,
  })
  const remove = useMutation({
    mutationFn: () => suiteRunMutations.remove(slug, id),
    onSuccess: () => {
      toast.success(t('suites.suiteRuns.toasts.deleted'))
      invalidate()
      if (sr)
        void navigate({
          to: '/t/$slug/suites/$id',
          params: { slug, id: sr.suite.id },
          search: { tab: 'runs' } as never,
        })
      else void navigate({ to: '/t/$slug/suites', params: { slug } })
    },
    onError,
  })

  if (q.isError)
    return (
      <Page>
        <ErrorState error={q.error} onRetry={() => void q.refetch()} />
      </Page>
    )
  const terminal = !!sr && isTerminal(sr.status)
  const failedCount = sr ? sr.progress.failed + (sr.progress.cancelled ?? 0) : 0
  // done / failed / cancelled are disjoint counters (internal/domain/suite Progress).
  const okCount = sr ? sr.progress.done : 0

  return (
    <Page width="wide">
      <PageHeader
        breadcrumbs={[
          { label: t('suites.title'), to: '/t/$slug/suites', params: { slug } },
          ...(sr
            ? [
                {
                  label: sr.suite.name ?? sr.suite.id,
                  to: '/t/$slug/suites/$id' as const,
                  params: { slug, id: sr.suite.id },
                },
              ]
            : []),
          { label: sr?.name ?? '…' },
        ]}
        title={sr?.name ?? '…'}
        icon="layer-group"
        badge={sr ? <StatusBadge status={sr.status} /> : undefined}
        subtitle={
          sr ? (
            <Stack gap={2} wrap="wrap">
              <span>
                <Icon
                  name={
                    sr.trigger === 'schedule'
                      ? 'clock-nine'
                      : sr.trigger === 'api'
                        ? 'code-branch'
                        : 'user'
                  }
                  size="sm"
                />{' '}
                {t(`common.trigger.${sr.trigger}`)}
                {sr.trigger_ref?.schedule_id && (
                  <>
                    {' · '}
                    <AppLink
                      to="/t/$slug/schedules/$id"
                      params={{ slug, id: sr.trigger_ref.schedule_id }}
                    >
                      {t('suites.suiteRuns.meta.schedule')}
                    </AppLink>
                  </>
                )}
              </span>
              <span>
                {t('suites.suiteRuns.meta.concurrency')}: <strong>{sr.concurrency ?? '—'}</strong>
              </span>
              <span>
                {t('suites.suiteRuns.meta.started')}:{' '}
                <RelativeTime value={sr.started_at ?? sr.created_at} />
              </span>
              <span>
                {t('suites.suiteRuns.meta.duration')}: <strong>{duration}</strong>
              </span>
            </Stack>
          ) : undefined
        }
        actions={
          sr ? (
            <>
              {!terminal && (
                <ConfirmAction
                  title={t('suites.suiteRuns.confirm.cancelTitle', { name: sr.name })}
                  body={t('suites.suiteRuns.confirm.cancelBody')}
                  confirmText={t('suites.suiteRuns.confirm.yesCancel')}
                  onConfirm={() => cancel.mutateAsync()}
                >
                  {(open) => (
                    <Button
                      variant="destructive"
                      icon="times"
                      disabled={!can('cancel-any-run', sr.author.id) || sr.status === 'cancelling'}
                      onClick={open}
                    >
                      {t('suites.suiteRuns.actions.cancel')}
                    </Button>
                  )}
                </ConfirmAction>
              )}
              {terminal && failedCount > 0 && (
                <Button
                  icon="repeat"
                  variant="secondary"
                  disabled={!can('run') || retry.isPending}
                  onClick={() => retry.mutate()}
                >
                  {t('suites.suiteRuns.actions.retryFailed')} ({failedCount})
                </Button>
              )}
              <Button
                icon="share-alt"
                variant="secondary"
                disabled={!can('share')}
                onClick={() => setSharing(true)}
              >
                {t('suites.suiteRuns.actions.share')}
              </Button>
              {terminal && (
                <ConfirmAction
                  title={t('common.confirm.deleteTitle', { name: sr.name })}
                  body={t('suites.suiteRuns.confirm.deleteBody')}
                  onConfirm={() => remove.mutateAsync()}
                >
                  {(open) => (
                    <Button
                      variant="secondary"
                      icon="trash-alt"
                      disabled={!can('cancel-any-run', sr.author.id)}
                      onClick={open}
                      aria-label={t('suites.suiteRuns.actions.delete')}
                    />
                  )}
                </ConfirmAction>
              )}
            </>
          ) : undefined
        }
      />
      {sr && (
        <div className={styles.progress}>
          <Stack justifyContent="space-between" alignItems="baseline">
            <span className={styles.big}>
              {sr.progress.done}/{sr.progress.total}
              <Text color="secondary" variant="body">
                {' '}
                · {sr.progress.pct ?? 0}%
              </Text>
            </span>
            <Text color="secondary" variant="bodySmall">
              {okCount} {t('suites.suiteRuns.progress.done')} · {sr.progress.failed}{' '}
              {t('suites.suiteRuns.progress.failed')} · {sr.progress.running}{' '}
              {t('suites.suiteRuns.progress.running')} · {sr.progress.pending}{' '}
              {t('suites.suiteRuns.progress.pending')}
            </Text>
          </Stack>
          <SuiteRunProgressBar progress={sr.progress} size="lg" legend />
        </div>
      )}
      <TabsBar>
        {SUITE_RUN_TABS.map((x) => (
          <Tab
            key={x}
            label={t(`suites.suiteRuns.tabs.${x}`)}
            active={tab === x}
            counter={x === 'cells' ? sr?.cells.length : undefined}
            onChangeTab={() => onTabChange(x)}
          />
        ))}
      </TabsBar>
      <TabContent>
        <div className={styles.grid}>
          <div style={{ minWidth: 0 }}>
            {sr && tab === 'summary' && <SuiteRunSummaryTab suiteRun={sr} />}
            {sr && tab === 'cells' && <SuiteRunCellsTab suiteRun={sr} />}
          </div>
          {sr && (
            <Stack direction="column" gap={3}>
              <KeyValueList
                title={t('suites.suiteRuns.one')}
                items={[
                  {
                    label: t('suites.suiteRuns.meta.suite'),
                    value: (
                      <AppLink to="/t/$slug/suites/$id" params={{ slug, id: sr.suite.id }}>
                        {sr.suite.name ?? sr.suite.id}
                      </AppLink>
                    ),
                  },
                  {
                    label: t('suites.suiteRuns.meta.trigger'),
                    value: t(`common.trigger.${sr.trigger}`),
                  },
                  ...(sr.trigger_ref?.retry_of
                    ? [
                        {
                          label: t('suites.suiteRuns.meta.retryOf'),
                          value: (
                            <AppLink
                              to="/t/$slug/suite-runs/$id"
                              params={{ slug, id: sr.trigger_ref.retry_of }}
                            >
                              {sr.trigger_ref.retry_of.slice(0, 8)}
                            </AppLink>
                          ),
                        },
                      ]
                    : []),
                  { label: t('suites.suiteRuns.meta.concurrency'), value: sr.concurrency },
                  { label: t('suites.suiteRuns.meta.author'), value: sr.author.display_name },
                  {
                    label: t('suites.suiteRuns.meta.started'),
                    value: formatDateTime(sr.started_at ?? sr.created_at),
                  },
                  {
                    label: t('common.fields.finished'),
                    value: sr.finished_at ? formatDateTime(sr.finished_at) : '—',
                  },
                  { label: t('suites.suiteRuns.meta.duration'), value: duration },
                  ...(sr.labels && Object.keys(sr.labels).length
                    ? [
                        {
                          label: t('suites.suiteRuns.meta.labels'),
                          value: <TagsView value={sr.labels} />,
                        },
                      ]
                    : []),
                ]}
              />
              <Button
                variant="secondary"
                size="sm"
                icon="list-ul"
                onClick={() =>
                  void navigate({
                    to: '/t/$slug/runs',
                    params: { slug },
                    search: { suiteRun: sr.id } as never,
                  })
                }
              >
                {t('suites.suiteRuns.actions.allRuns')}
              </Button>
            </Stack>
          )}
        </div>
      </TabContent>
      {sharing && sr && <SuiteRunShareDialog suiteRun={sr} onClose={() => setSharing(false)} />}
    </Page>
  )
}
