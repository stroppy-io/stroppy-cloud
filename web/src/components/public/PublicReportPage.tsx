import { publicQueries } from '@api/queries/results'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { LoadingPlaceholder, Text, useStyles2 } from '@grafana/ui'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ComparisonTable } from './ComparisonTable'
import { PublicHeader } from './PublicHeader'
import { ReportConfigs } from './ReportConfigs'
import { ReportResults } from './ReportResults'
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
  nav: css({
    display: 'flex',
    gap: theme.spacing(0.5),
    marginTop: theme.spacing(3),
    flexWrap: 'wrap',
    '@media print': { display: 'none' },
    a: {
      padding: theme.spacing(0.5, 1.25),
      borderRadius: 999,
      border: `1px solid ${theme.colors.border.weak}`,
      color: theme.colors.text.secondary,
      fontSize: theme.typography.bodySmall.fontSize,
      textDecoration: 'none',
      '&:hover': { color: theme.colors.text.primary, borderColor: theme.colors.border.medium },
    },
  }),
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

// Public lab-report: no AppShell, no auth. Works for run / suite_run / comparison targets.
export function PublicReportPage({ token }: { token: string }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const q = useQuery(publicQueries.share(token))
  const snap = q.data
  const run = snap?.run
  const withConfigs = snap?.scope === 'configs'
  const sections = run
    ? [
        { id: 'summary', label: t('public.nav.summary') },
        { id: 'timeline', label: t('public.nav.timeline') },
        { id: 'setup', label: t('public.nav.setup') },
        { id: 'results', label: t('public.nav.results') },
        ...(withConfigs ? [{ id: 'configs', label: t('public.nav.configs') }] : []),
      ]
    : []
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
            {sections.length > 0 && (
              <nav className={styles.nav} aria-label={t('public.sharedReport')}>
                {sections.map((s) => (
                  <a key={s.id} href={`#${s.id}`}>
                    {s.label}
                  </a>
                ))}
              </nav>
            )}
            {run && (
              <>
                <ReportSummary run={run} title={snap.title} />
                <ReportTimeline run={run} index={1} />
                <ReportSetup run={run} index={2} />
                <ReportResults run={run} token={token} scope={snap.scope} index={3} />
                {withConfigs && <ReportConfigs run={run} index={4} />}
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
