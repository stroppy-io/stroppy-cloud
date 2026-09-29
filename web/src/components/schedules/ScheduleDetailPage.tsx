import { scheduleMutations, scheduleQueries } from '@api/queries/suites'
import type { Schedule } from '@api/types'
import { AppLink } from '@app/AppLink'
import { ErrorState } from '@app/ErrorState'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { ConfirmAction } from '@components/ConfirmAction'
import { KeyValueList } from '@components/KeyValueList'
import { RelativeTime } from '@components/RelativeTime'
import { StatusBadge } from '@components/StatusBadge'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Badge, Button, Stack, Tab, TabContent, TabsBar, useStyles2 } from '@grafana/ui'
import { formatDateTime } from '@helpers/time'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { CronHuman } from './CronPreview'
import { ScheduleForm, scheduleFormDefaults } from './ScheduleForm'
import { ScheduleHistoryTab } from './ScheduleHistoryTab'
import { ScheduleTargetChip } from './ScheduleTargetChip'

export type ScheduleTab = 'settings' | 'history'
export const SCHEDULE_TABS: ScheduleTab[] = ['settings', 'history']

const getStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) 280px',
    gap: theme.spacing(3),
    marginTop: theme.spacing(2),
    [theme.breakpoints.down('md')]: { gridTemplateColumns: '1fr' },
  }),
  mono: css({ fontFamily: theme.typography.fontFamilyMonospace }),
})

export function ScheduleDetailPage({
  id,
  tab,
  onTabChange,
}: {
  id: string
  tab: ScheduleTab
  onTabChange: (t: ScheduleTab) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug, can } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const q = useQuery(scheduleQueries.detail(slug, id))
  const s = q.data
  const invalidate = () => qc.invalidateQueries({ queryKey: ['t', slug, 'schedules'] })
  const onError = (e: unknown) => toast.error(e)
  const toggle = useMutation({
    mutationFn: (sch: Schedule) =>
      sch.enabled ? scheduleMutations.pause(slug, sch.id) : scheduleMutations.resume(slug, sch.id),
    onSuccess: (next) => {
      qc.setQueryData(scheduleQueries.detail(slug, id).queryKey, next)
      toast.success(next.enabled ? t('schedules.toasts.resumed') : t('schedules.toasts.paused'))
      void invalidate()
    },
    onError,
  })
  const runNow = useMutation({
    mutationFn: () => scheduleMutations.runNow(slug, id),
    onSuccess: (ref) => {
      toast.success(t('schedules.toasts.ran', { name: ref.name ?? ref.id }), {
        action: {
          label: t('schedules.toasts.ranOpen'),
          onClick: () =>
            ref.kind === 'run'
              ? void navigate({ to: '/t/$slug/runs/$id', params: { slug, id: ref.id } })
              : void navigate({ to: '/t/$slug/suite-runs/$id', params: { slug, id: ref.id } }),
        },
      })
      void invalidate()
      void qc.invalidateQueries({ queryKey: ['t', slug, 'runs'] })
      void qc.invalidateQueries({ queryKey: ['t', slug, 'suite-runs'] })
      onTabChange('history')
    },
    onError,
  })
  const remove = useMutation({
    mutationFn: () => scheduleMutations.remove(slug, id),
    onSuccess: () => {
      toast.success(t('schedules.toasts.deleted'))
      void invalidate()
      void navigate({ to: '/t/$slug/schedules', params: { slug } })
    },
    onError,
  })
  const canEdit = can('edit-library')

  if (q.isError)
    return (
      <Page>
        <ErrorState error={q.error} onRetry={() => void q.refetch()} />
      </Page>
    )

  return (
    <Page width="wide">
      <PageHeader
        breadcrumbs={[
          { label: t('schedules.title'), to: '/t/$slug/schedules', params: { slug } },
          { label: s?.name ?? '…' },
        ]}
        title={s?.name ?? '…'}
        icon="clock-nine"
        badge={
          s ? (
            <Badge
              text={s.enabled ? t('schedules.enabled') : t('schedules.paused')}
              color={s.enabled ? 'green' : 'darkgrey'}
              icon={s.enabled ? 'check' : 'pause'}
            />
          ) : undefined
        }
        subtitle={
          s ? (
            <Stack gap={2} wrap="wrap" alignItems="center">
              <ScheduleTargetChip target={s.target} />
              <span>
                <span className={styles.mono}>{s.cron}</span> · <CronHuman cron={s.cron} /> ·{' '}
                {s.timezone}
              </span>
            </Stack>
          ) : undefined
        }
        actions={
          s ? (
            <>
              <ConfirmAction
                title={t('common.confirm.deleteTitle', { name: s.name })}
                body={t('schedules.confirm.deleteBody')}
                onConfirm={() => remove.mutateAsync()}
              >
                {(open) => (
                  <Button
                    variant="secondary"
                    icon="trash-alt"
                    disabled={!canEdit}
                    onClick={open}
                    aria-label={t('schedules.actions.delete')}
                  />
                )}
              </ConfirmAction>
              <Button
                variant="secondary"
                icon={s.enabled ? 'pause' : 'play'}
                disabled={!canEdit || toggle.isPending}
                onClick={() => toggle.mutate(s)}
              >
                {s.enabled ? t('schedules.actions.pause') : t('schedules.actions.resume')}
              </Button>
              <Button
                icon="play"
                disabled={!can('run') || runNow.isPending}
                onClick={() => runNow.mutate()}
              >
                {t('schedules.actions.runNow')}
              </Button>
            </>
          ) : undefined
        }
      />
      <TabsBar>
        {SCHEDULE_TABS.map((x) => (
          <Tab
            key={x}
            label={t(`schedules.tabs.${x}`)}
            active={tab === x}
            onChangeTab={() => onTabChange(x)}
          />
        ))}
      </TabsBar>
      <TabContent>
        {s && tab === 'settings' && (
          <div className={styles.grid}>
            <div style={{ marginTop: 16 }}>
              <ScheduleForm
                key={s.updated_at ?? s.created_at}
                initial={scheduleFormDefaults(s)}
                submitLabel={t('schedules.actions.save')}
                onSubmit={async (w) => {
                  const next = await scheduleMutations.patch(slug, s.id, w)
                  qc.setQueryData(scheduleQueries.detail(slug, id).queryKey, next)
                  toast.success(t('schedules.toasts.saved'))
                  await invalidate()
                }}
              />
            </div>
            <div style={{ marginTop: 16 }}>
              <KeyValueList
                title={t('schedules.about.title')}
                items={[
                  { label: t('schedules.about.author'), value: s.author?.display_name },
                  { label: t('schedules.about.created'), value: formatDateTime(s.created_at) },
                  {
                    label: t('schedules.about.updated'),
                    value: s.updated_at ? <RelativeTime value={s.updated_at} /> : '—',
                  },
                  {
                    label: t('schedules.about.nextRun'),
                    value:
                      s.enabled && s.next_run_at ? (
                        <RelativeTime value={s.next_run_at} />
                      ) : (
                        t('schedules.paused')
                      ),
                  },
                  {
                    label: t('schedules.about.lastRun'),
                    value: s.last_run ? (
                      <Stack alignItems="center" gap={1}>
                        <StatusBadge status={s.last_run.status} iconOnly />
                        {s.last_run.kind === 'run' ? (
                          <AppLink to="/t/$slug/runs/$id" params={{ slug, id: s.last_run.id }}>
                            {s.last_run.name ?? s.last_run.id}
                          </AppLink>
                        ) : (
                          <AppLink
                            to="/t/$slug/suite-runs/$id"
                            params={{ slug, id: s.last_run.id }}
                          >
                            {s.last_run.name ?? s.last_run.id}
                          </AppLink>
                        )}
                      </Stack>
                    ) : (
                      t('schedules.neverRan')
                    ),
                  },
                ]}
              />
            </div>
          </div>
        )}
        {s && tab === 'history' && <ScheduleHistoryTab scheduleId={s.id} />}
      </TabContent>
    </Page>
  )
}
