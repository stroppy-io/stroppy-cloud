import { testQueries } from '@api/queries/library'
import type { Schemas } from '@api/types'
import { AppLink } from '@app/AppLink'
import { useDensity } from '@components/DataTable/prefs'
import { RelativeTime } from '@components/RelativeTime'
import { StatusBadge } from '@components/StatusBadge'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { TpsTrend } from '../shared/TpsTrend'

type Test = Schemas['Test']

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({ display: 'flex', flexDirection: 'column', minWidth: 0 }),
  top: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(0.75), minWidth: 0 }),
  sub: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.75),
    minWidth: 0,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  never: css({ color: theme.colors.text.secondary }),
})

const SPARK_W = 56
const SPARK_H = 14

// Throughput of the test's completed runs; loaded lazily per row and only when there is a trend
// to draw (two runs or more).
function TrendSpark({ testId }: { testId: string }) {
  const { slug } = useTenant()
  const runs = useQuery({ ...testQueries.runs(slug, testId, 20), staleTime: 60_000 })
  if ((runs.data?.trend?.points?.length ?? 0) < 2) return null
  return <TpsTrend trend={runs.data?.trend} width={SPARK_W} height={SPARK_H} />
}

// «Последний запуск»: status icon + when (link to the run) over the trend sparkline + run count.
export function TestLastRunCell({ test }: { test: Test }) {
  const styles = useStyles2(getStyles)
  const density = useDensity()
  const { t } = useTranslation()
  const { slug } = useTenant()
  const last = test.summary?.last_run
  const count = test.summary?.run_count ?? 0
  if (!last) return <span className={styles.never}>{t('library.tests.neverRun')}</span>
  return (
    <div className={styles.root}>
      <div className={styles.top} title={last.name ?? last.id}>
        <StatusBadge status={last.status} iconOnly />
        <AppLink to="/t/$slug/runs/$id" params={{ slug, id: last.id }} plain>
          {last.started_at ? <RelativeTime value={last.started_at} /> : (last.name ?? last.id)}
        </AppLink>
      </div>
      {density !== 'compact' && (
        <div className={styles.sub}>
          {count >= 2 && <TrendSpark testId={test.id} />}
          <span>{t('library.tests.runCount', { count })}</span>
        </div>
      )}
    </div>
  )
}
