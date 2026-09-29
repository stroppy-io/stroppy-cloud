import { suiteRunQueries } from '@api/queries/suites'
import type { SuiteRun, SuiteRunSummary } from '@api/types'
import { AppLink } from '@app/AppLink'
import { Dash } from '@components/DataTable/cells'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import { useDensity } from '@components/DataTable/prefs'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, useStyles2 } from '@grafana/ui'
import { formatMetric } from '@helpers/format'
import { isTerminal } from '@helpers/run-status'
import { useTenant } from '@hooks/useTenant'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import type { TFunction } from 'i18next'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { runRefActions } from './run-like'

type Row = SuiteRunSummary['rows'][number]
type Metric = NonNullable<Row['metrics']>[string]

const getStyles = (theme: GrafanaTheme2) => ({
  metrics: css({ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', minWidth: 0 }),
  line: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontVariantNumeric: 'tabular-nums',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    maxWidth: '100%',
  }),
  top: css({ fontSize: theme.typography.bodySmall.fontSize }),
  sub: css({ color: theme.colors.text.secondary, fontSize: theme.typography.bodySmall.fontSize }),
  diff: css({ color: theme.colors.text.secondary }),
  better: css({ color: theme.colors.success.text }),
  worse: css({ color: theme.colors.error.text, fontWeight: theme.typography.fontWeightMedium }),
  worseRow: css({ td: { background: theme.colors.error.transparent } }),
  legend: css({
    display: 'flex',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
    margin: theme.spacing(2, 0, 1),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
})

// Headline metrics of a summary row, in display order: throughput first, then latency, errors.
const THROUGHPUT = [
  ['qps', 'QPS', 'count'],
  ['tps', 'TPS', 'count'],
] as const
const SECONDARY = [
  ['latency_p99_ms', 'p99', 'ms'],
  ['latency_p50_ms', 'p50', 'ms'],
  ['errors', 'errors', 'count'],
] as const

function diffText(m: Metric): string | undefined {
  if (m.diff_pct === undefined) return undefined
  return `${m.diff_pct > 0 ? '+' : ''}${m.diff_pct.toFixed(1)}%`
}

function describe(label: string, unit: string, m: Metric, t: TFunction): string {
  const diff = diffText(m)
  return [
    `${label} ${formatMetric(m.value, unit)}`,
    diff
      ? `${diff} ${t(`suites.suiteRuns.summary.verdict.${m.verdict ?? 'missing'}`)} (${formatMetric(m.previous, unit)})`
      : undefined,
  ]
    .filter(Boolean)
    .join(' · ')
}

// «Показатели» of a summary row: throughput with its diff over p99 · p50 · errors with theirs.
// Verdict colours only the diff (green better, red regression); the native tooltip spells every
// metric out with the previous value.
function SummaryMetricsCell({ row }: { row: Row }) {
  const styles = useStyles2(getStyles)
  const density = useDensity()
  const { t } = useTranslation()
  const m = row.metrics ?? {}
  const head = THROUGHPUT.find(([k]) => m[k]?.value !== undefined)
  const rest = SECONDARY.filter(([k]) => m[k]?.value !== undefined)
  if (!head && !rest.length)
    return (
      <div className={styles.metrics}>
        <Dash />
      </div>
    )
  const tone = (x: Metric) =>
    x.verdict === 'better' ? styles.better : x.verdict === 'worse' ? styles.worse : styles.diff
  const part = ([k, label, unit]: readonly [string, string, string], showLabel: boolean) => {
    const x = m[k] as Metric
    const diff = diffText(x)
    const name = label === 'errors' ? t('runs.sort.errors') : label
    return (
      <span key={k}>
        {showLabel ? `${name} ` : ''}
        {formatMetric(x.value, unit)}
        {diff && (
          <span className={tone(x)}>
            {' '}
            {x.verdict === 'better' && <Icon name="arrow-up" size="xs" />}
            {x.verdict === 'worse' && <Icon name="arrow-down" size="xs" />}
            {diff}
          </span>
        )}
      </span>
    )
  }
  const title = [head, ...rest]
    .filter((x): x is NonNullable<typeof x> => !!x)
    .map(([k, label, unit]) =>
      describe(label === 'errors' ? t('runs.sort.errors') : label, unit, m[k] as Metric, t)
    )
    .join('\n')
  return (
    <div className={styles.metrics} title={title}>
      {head && <span className={cx(styles.line, styles.top)}>{part(head, true)}</span>}
      {density !== 'compact' && rest.length > 0 && (
        <span className={cx(styles.line, styles.sub)}>
          {rest.map((r, i) => (
            <span key={r[0]}>
              {i > 0 ? ' · ' : ''}
              {part(r, true)}
            </span>
          ))}
        </span>
      )}
    </div>
  )
}

export function SuiteRunSummaryTab({ suiteRun }: { suiteRun: SuiteRun }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug } = useTenant()
  const navigate = useNavigate()
  const live = !isTerminal(suiteRun.status)
  const summary = useQuery({
    ...suiteRunQueries.summary(slug, suiteRun.id),
    refetchInterval: live ? 5000 : false,
  })

  const rowActions = (r: Row): RowAction[] =>
    runRefActions({
      runId: r.run?.id,
      slug,
      t,
      navigate: (to) => void navigate({ to: to as never }),
      notStartedReason: t('suites.suiteRuns.summary.noRun'),
    })

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure
  const columns = useMemo<DataTableColumn<Row>[]>(
    () => [
      col.status<Row>({
        id: 'status',
        title: t('runs.columns.status'),
        status: (r) => r.status,
      }),
      col.identity<Row>({
        id: 'cell',
        header: t('suites.suiteRuns.summary.columns.cell'),
        render: (r) => {
          const regress = Object.values(r.metrics ?? {}).filter((m) => m.verdict === 'worse')
          return {
            title: r.name ?? r.cell_id,
            link: r.run ? { to: '/t/$slug/runs/$id', params: { slug, id: r.run.id } } : undefined,
            subtitle: r.run ? (r.run.name ?? r.run.id) : t('suites.suiteRuns.summary.noRun'),
            badges:
              regress.length > 0 ? (
                <span className={styles.worse} title={t('suites.suiteRuns.summary.verdict.worse')}>
                  <Icon name="arrow-down" />
                </span>
              ) : undefined,
          }
        },
      }),
      col.custom<Row>({
        id: 'metrics',
        header: t('runs.columns.metrics'),
        width: 320,
        align: 'right',
        cell: (r) => <SummaryMetricsCell row={r} />,
      }),
      col.actions<Row>({ title: (r) => r.name ?? r.cell_id, actions: rowActions }),
    ],
    [t, slug, styles]
  )
  const prev = summary.data?.compared_to
  return (
    <>
      <div className={styles.legend}>
        <span>
          {prev ? (
            <>
              {t('suites.suiteRuns.summary.comparedTo')}{' '}
              <AppLink to="/t/$slug/suite-runs/$id" params={{ slug, id: prev }}>
                {prev.slice(0, 8)}
              </AppLink>
            </>
          ) : (
            t('suites.suiteRuns.summary.noPrevious')
          )}
        </span>
        {prev && <span>{t('suites.suiteRuns.summary.legend')}</span>}
      </div>
      <DataTable<Row>
        columns={columns}
        data={summary.data?.rows ?? []}
        getRowId={(r) => r.cell_id}
        loading={summary.isPending}
        error={summary.isError ? summary.error : undefined}
        onRetry={() => void summary.refetch()}
        rowHref={(r) => (r.run ? `/t/${slug}/runs/${r.run.id}` : undefined)}
        rowClassName={(r) =>
          Object.values(r.metrics ?? {}).some((m) => m.verdict === 'worse')
            ? styles.worseRow
            : undefined
        }
        empty={{ message: t('suites.suiteRuns.summary.empty') }}
      />
    </>
  )
}
