import type { ShareSnapshot } from '@api/queries/results'
import { StatusBadge } from '@components/StatusBadge'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Text, useStyles2 } from '@grafana/ui'
import { formatMetric } from '@helpers/format'
import { useTranslation } from 'react-i18next'
import { ReportSection, SimpleTable, useReportStyles } from './ReportPrimitives'

type SuiteSnapshot = NonNullable<ShareSnapshot['suite_run']>

const getStyles = (theme: GrafanaTheme2) => ({
  hero: css({
    marginTop: theme.spacing(3),
    padding: theme.spacing(3),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    display: 'flex',
    alignItems: 'flex-start',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
  }),
  h1: css({
    margin: 0,
    fontSize: theme.typography.h1.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: 1.2,
  }),
})

export function SuiteReport({ suite, title }: { suite: SuiteSnapshot; title?: string }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const rs = useReportStyles()
  const rows = suite.summary?.rows ?? []
  const cells = suite.cells ?? []
  const done = rows.filter((r) => r.status === 'completed').length
  return (
    <>
      <div className={styles.hero} id="summary">
        <div style={{ flex: 1, minWidth: 0 }}>
          <h1 className={styles.h1}>{title ?? suite.name ?? ''}</h1>
          {title && <Text color="secondary">{suite.name ?? ''}</Text>}
          <div className={rs.muted}>{t('public.suite.progress', { done, total: rows.length })}</div>
        </div>
        <StatusBadge status={suite.status} />
      </div>
      <ReportSection id="results" index={1} title={t('public.suite.cells')}>
        <SimpleTable>
          <thead>
            <tr>
              <th>{t('public.suite.cell')}</th>
              <th>{t('public.suite.status')}</th>
              <th>{t('public.suite.database')}</th>
              <th>{t('public.suite.topology')}</th>
              <th className="num">{t('public.suite.tps')}</th>
              <th className="num">{t('public.suite.p99')}</th>
              <th className="num">{t('public.suite.errors')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r, i) => {
              const cell = cells.find((c) => c.name === r.run?.name) ?? cells[i]
              return (
                <tr key={r.cell_id}>
                  <td>
                    <Text weight="medium">{r.name ?? r.cell_id}</Text>
                    {r.run?.name && (
                      <div className={rs.muted}>
                        <Text variant="bodySmall" color="secondary">
                          {r.run.name}
                        </Text>
                      </div>
                    )}
                  </td>
                  <td>
                    <StatusBadge status={r.status} />
                  </td>
                  <td>
                    {cell?.summary.db_kind} {cell?.summary.db_version}
                  </td>
                  <td>
                    {cell?.summary.topology_label ?? '—'}
                    {cell?.summary.league ? ` · ${cell.summary.league}` : ''}
                  </td>
                  <td className="num">{formatMetric(r.metrics?.tps?.value, 'tps')}</td>
                  <td className="num">{formatMetric(r.metrics?.latency_p99_ms?.value, 'ms')}</td>
                  <td className="num">{formatMetric(r.metrics?.errors?.value, 'count')}</td>
                </tr>
              )
            })}
          </tbody>
        </SimpleTable>
      </ReportSection>
    </>
  )
}
