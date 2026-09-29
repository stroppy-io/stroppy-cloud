import { dashboardQueries } from '@api/queries/results'
import type { Run, TenantDashboard } from '@api/types'
import { ErrorState } from '@app/ErrorState'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, LoadingPlaceholder, useStyles2 } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useTopic } from '@hooks/useTopic'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Onboarding } from './Onboarding'
import { ProvidersHealth } from './ProvidersHealth'
import { LiveNow, RecentRuns, RecentSuiteRuns, TopResults, Upcoming } from './RunLists'
import { StatTiles } from './StatTiles'

const getStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 2fr) minmax(0, 1fr)',
    gap: theme.spacing(2),
    marginTop: theme.spacing(2),
    alignItems: 'start',
    [theme.breakpoints.down('md')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  col: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2), minWidth: 0 }),
})

const ACTIVE = new Set(['running', 'pending', 'cancelling'])

// Merge run deltas from `tenant.runs/{slug}` into the dashboard payload and recount tiles.
function mergeRuns(prev: TenantDashboard | undefined, payload: { data: Run[] }): TenantDashboard {
  if (!prev) return prev as never
  const byId = new Map(prev.recent_runs.map((r) => [r.id, r]))
  for (const r of payload.data) byId.set(r.id, r)
  const runs = [...byId.values()]
  const running = runs.filter((r) => r.status === 'running' || r.status === 'cancelling').length
  const pending = runs.filter((r) => r.status === 'pending').length
  return {
    ...prev,
    run_counts: { ...prev.run_counts, running, pending },
    recent_runs: runs,
  }
}

export function DashboardPage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug, tenant } = useTenant()
  const navigate = useNavigate()
  const q = useQuery(dashboardQueries.get(slug))
  useTopic<{ data: Run[] }, TenantDashboard>({
    topic: `tenant.runs/${slug}`,
    queryKey: dashboardQueries.get(slug).queryKey,
    batchMs: 250,
    merge: mergeRuns,
  })
  const data = q.data
  const { live, recent } = useMemo(() => {
    const all = data?.recent_runs ?? []
    return {
      live: all
        .filter((r) => ACTIVE.has(r.status))
        .sort((a, b) => (b.started_at ?? b.created_at).localeCompare(a.started_at ?? a.created_at)),
      recent: all
        .filter((r) => !ACTIVE.has(r.status))
        .sort((a, b) => (b.finished_at ?? '').localeCompare(a.finished_at ?? ''))
        .slice(0, 8),
    }
  }, [data])

  const actions = (
    <>
      <Button
        variant="secondary"
        icon="book-open"
        onClick={() => void navigate({ to: '/examples' })}
      >
        {t('dashboard.quick.example')}
      </Button>
      <Button
        variant="secondary"
        icon="import"
        onClick={() => void navigate({ to: '/t/$slug/library/tests', params: { slug } })}
      >
        {t('dashboard.quick.import')}
      </Button>
      <Button
        icon="plus"
        onClick={() => void navigate({ to: '/t/$slug/library/tests/new', params: { slug } })}
      >
        {t('dashboard.quick.newTest')}
      </Button>
    </>
  )

  return (
    <Page width="wide">
      <PageHeader
        title={tenant?.name ?? t('dashboard.title')}
        subtitle={tenant?.description}
        icon="apps"
        actions={actions}
      />
      {q.isError ? (
        <ErrorState error={q.error} onRetry={() => void q.refetch()} />
      ) : !data ? (
        <LoadingPlaceholder text={t('common.misc.loading')} />
      ) : (data.run_counts.total ?? 0) === 0 ? (
        <Onboarding
          slug={slug}
          tenantName={tenant?.name ?? slug}
          hasProvider={(data.providers ?? []).some((p) => p.status === 'ready')}
        />
      ) : (
        <>
          <StatTiles data={data} slug={slug} />
          <div className={styles.grid}>
            <div className={styles.col}>
              <LiveNow runs={live} slug={slug} />
              <RecentRuns runs={recent} slug={slug} />
              <RecentSuiteRuns items={data.recent_suite_runs} slug={slug} />
            </div>
            <div className={styles.col}>
              <TopResults items={data.top_results ?? []} slug={slug} />
              <Upcoming items={data.upcoming} slug={slug} />
              <ProvidersHealth providers={data.providers ?? []} limits={data.limits} slug={slug} />
            </div>
          </div>
        </>
      )}
    </Page>
  )
}
