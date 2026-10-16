import { publicQueries } from '@api/queries/results'
import type { ExplorerSearch } from '@components/metrics/explorer-search'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { LoadingPlaceholder, Tab, TabsBar, Text, useStyles2 } from '@grafana/ui'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ComparisonTable } from './ComparisonTable'
import { PublicHeader } from './PublicHeader'
import { ReportConfigs } from './ReportConfigs'
import { ReportExplorer, ReportResults } from './ReportResults'
import { ReportSetup } from './ReportSetup'
import { ReportSummary } from './ReportSummary'
import { ReportTimeline } from './ReportTimeline'
import { ShareGone } from './ShareGone'
import { SuiteReport } from './SuiteReport'

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    minHeight: '100vh',
    background: theme.colors.background.canvas,
    color: theme.colors.text.primary,
    '@media print': { background: 'transparent' },
  }),
  main: css({
    maxWidth: 1100,
    margin: '0 auto',
    padding: theme.spacing(0, 3, 8),
    [theme.breakpoints.down('md')]: { padding: theme.spacing(0, 2, 6) },
  }),
  tabs: css({
    marginTop: theme.spacing(3),
    '@media print': { display: 'none' },
  }),
  // Inactive tabs stay in the page for print: paper gets the whole report.
  inactive: css({ display: 'none', '@media print': { display: 'block' } }),
  footer: css({
    marginTop: theme.spacing(6),
    paddingTop: theme.spacing(2),
    borderTop: `1px solid ${theme.colors.border.weak}`,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    display: 'flex',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
  }),
})

export const reportTabs = ['results', 'explorer', 'setup', 'timeline', 'configs'] as const
export type ReportTab = (typeof reportTabs)[number]
export type ReportSearch = ExplorerSearch & { tab?: ReportTab }

// Public lab-report: no AppShell, no auth. Works for run / suite_run / comparison targets.
// A run report opens on its results; the stand, the run's course and the configs are tabs a
// reader goes to when the numbers raise a question.
export function PublicReportPage({
  token,
  search,
  onSearchChange,
}: {
  token: string
  search: ReportSearch
  onSearchChange: (next: Partial<ReportSearch>) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const q = useQuery(publicQueries.share(token))
  const snap = q.data
  const run = snap?.run
  const withConfigs = snap?.scope === 'configs'
  const withMetrics = snap !== undefined && snap.scope !== 'overview'
  const tabs = reportTabs.filter(
    (id) => (id !== 'explorer' || withMetrics) && (id !== 'configs' || withConfigs)
  )
  const tab = search.tab && tabs.includes(search.tab) ? search.tab : 'results'
  const panel = (id: ReportTab) => (id === tab ? undefined : styles.inactive)
  return (
    <div className={styles.root}>
      <PublicHeader token={token} snapshot={snap} />
      <main className={styles.main}>
        {q.isPending ? (
          <LoadingPlaceholder text={t('common.misc.loading')} />
        ) : q.isError || !snap ? (
          <ShareGone error={q.error} />
        ) : (
          <>
            {run && (
              <>
                <ReportSummary run={run} title={snap.title} />
                <TabsBar className={styles.tabs}>
                  {tabs.map((id) => (
                    <Tab
                      key={id}
                      label={t(`public.tabs.${id}`)}
                      active={id === tab}
                      onChangeTab={() => onSearchChange({ tab: id === 'results' ? undefined : id })}
                    />
                  ))}
                </TabsBar>
                <div className={panel('results')}>
                  <ReportResults run={run} token={token} scope={snap.scope} index={1} tabbed />
                </div>
                {tab === 'explorer' && (
                  <ReportExplorer
                    run={run}
                    token={token}
                    index={5}
                    tabbed
                    search={search}
                    onSearchChange={onSearchChange}
                  />
                )}
                <div className={panel('setup')}>
                  <ReportSetup run={run} index={2} tabbed />
                </div>
                <div className={panel('timeline')}>
                  <ReportTimeline run={run} index={3} tabbed />
                </div>
                {withConfigs && (
                  <div className={panel('configs')}>
                    <ReportConfigs run={run} index={4} tabbed />
                  </div>
                )}
              </>
            )}
            {snap.suite_run && <SuiteReport suite={snap.suite_run} title={snap.title} />}
            {snap.comparison && (
              <ComparisonTable comparison={snap.comparison} index={1} title={snap.title} />
            )}
            <footer className={styles.footer}>
              <span>{t('public.poweredBy')}</span>
              <Text color="secondary" variant="bodySmall">
                {token}
              </Text>
            </footer>
          </>
        )}
      </main>
    </div>
  )
}
