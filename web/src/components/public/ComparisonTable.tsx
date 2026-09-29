import type { Comparison } from '@api/types'
import { StatusBadge } from '@components/StatusBadge'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Badge, Text, useStyles2 } from '@grafana/ui'
import { formatDuration, formatMetric } from '@helpers/format'
import { formatDate } from '@helpers/time'
import { useTranslation } from 'react-i18next'
import {
  ReportSection,
  SimpleTable,
  SubTitle,
  useReportStyles,
  valueToText,
} from './ReportPrimitives'

const getStyles = (theme: GrafanaTheme2) => ({
  col: css({ minWidth: 160 }),
  colName: css({ fontWeight: theme.typography.fontWeightMedium, whiteSpace: 'normal' }),
  colSub: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    fontWeight: theme.typography.fontWeightRegular,
    whiteSpace: 'normal',
    marginTop: 2,
  }),
  group: css({
    background: theme.colors.background.secondary,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    textTransform: 'uppercase',
    letterSpacing: 0.4,
  }),
  diff: css({ fontSize: theme.typography.bodySmall.fontSize, marginLeft: theme.spacing(0.5) }),
  better: css({ color: theme.colors.success.text }),
  worse: css({ color: theme.colors.error.text }),
  same: css({ color: theme.colors.text.secondary }),
  baseline: css({ color: theme.colors.text.secondary, fontStyle: 'italic' }),
})

export function ComparisonTable({
  comparison,
  index,
  title,
}: {
  comparison: Comparison
  index: number
  title?: string
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const rs = useReportStyles()
  const groups = [...new Set(comparison.metrics.map((m) => m.group ?? ''))]
  const baseId = comparison.baseline_run_id ?? comparison.columns[0]?.run_id
  return (
    <ReportSection
      id="results"
      index={index}
      title={title ?? t('public.comparison.title')}
      hint={t('public.comparison.hint')}
    >
      <SimpleTable>
        <thead>
          <tr>
            <th>{t('public.comparison.metric')}</th>
            {comparison.columns.map((c) => (
              <th key={c.run_id} className={cx('num', styles.col)}>
                <div className={styles.colName}>
                  {c.name ?? c.run_id}{' '}
                  {c.run_id === baseId && (
                    <Badge text={t('public.comparison.baseline')} color="blue" />
                  )}
                </div>
                <div className={styles.colSub}>
                  {c.summary?.db_kind} {c.summary?.db_version} · {c.summary?.topology_label} ·{' '}
                  {c.summary?.league}
                  <br />
                  {formatDate(c.started_at)} · {formatDuration(c.duration)}{' '}
                  <StatusBadge status={c.status} iconOnly />
                </div>
                {c.verdict && c.run_id !== baseId && (
                  <div className={styles.colSub}>
                    <span className={styles.better}>{c.verdict.better ?? 0} ▲</span>{' '}
                    <span className={styles.worse}>{c.verdict.worse ?? 0} ▼</span>{' '}
                    <span className={styles.same}>{c.verdict.same ?? 0} =</span>
                  </div>
                )}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {groups.map((g) => (
            <GroupRows key={g} group={g} comparison={comparison} baseId={baseId} />
          ))}
        </tbody>
      </SimpleTable>

      <SubTitle>{t('public.comparison.specDiff')}</SubTitle>
      {Object.values(comparison.spec_diff).every((d) => d.changes.length === 0) ? (
        <span className={rs.muted}>{t('public.comparison.noDiff')}</span>
      ) : (
        <SimpleTable>
          <thead>
            <tr>
              <th>{t('public.comparison.path')}</th>
              <th>{t('public.comparison.baselineValue')}</th>
              {comparison.columns
                .filter((c) => c.run_id !== baseId)
                .map((c) => (
                  <th key={c.run_id}>{c.name ?? c.run_id}</th>
                ))}
            </tr>
          </thead>
          <tbody>
            {[
              ...new Set(
                Object.values(comparison.spec_diff).flatMap((d) => d.changes.map((c) => c.path))
              ),
            ]
              .sort()
              .map((path) => {
                const first = Object.values(comparison.spec_diff)
                  .flatMap((d) => d.changes)
                  .find((c) => c.path === path)
                return (
                  <tr key={path}>
                    <td className="mono">{path}</td>
                    <td className="mono">{valueToText(first?.a)}</td>
                    {comparison.columns
                      .filter((c) => c.run_id !== baseId)
                      .map((c) => {
                        const ch = comparison.spec_diff[c.run_id]?.changes.find(
                          (x) => x.path === path
                        )
                        return (
                          <td key={c.run_id} className="mono">
                            {ch ? (
                              <span className={styles.worse}>{valueToText(ch.b)}</span>
                            ) : (
                              <Text color="secondary">=</Text>
                            )}
                          </td>
                        )
                      })}
                  </tr>
                )
              })}
          </tbody>
        </SimpleTable>
      )}
    </ReportSection>
  )
}

function GroupRows({
  group,
  comparison,
  baseId,
}: {
  group: string
  comparison: Comparison
  baseId?: string
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const rows = comparison.metrics.filter((m) => (m.group ?? '') === group)
  return (
    <>
      {group && (
        <tr>
          <td colSpan={comparison.columns.length + 1} className={styles.group}>
            {group}
          </td>
        </tr>
      )}
      {rows.map((m) => (
        <tr key={m.key}>
          <td>
            {m.title ?? m.key}
            {m.unit && (
              <Text color="secondary" variant="bodySmall">
                {' '}
                · {m.unit}
              </Text>
            )}
          </td>
          {comparison.columns.map((c) => {
            const cell = m.cells.find((x) => x.run_id === c.run_id)
            if (!cell?.present)
              return (
                <td key={c.run_id} className="num">
                  <Text color="secondary">{t('public.comparison.missing')}</Text>
                </td>
              )
            const v = cell.verdict
            return (
              <td key={c.run_id} className="num">
                {formatMetric(cell.value, m.unit)}
                {c.run_id !== baseId && cell.diff_pct !== undefined && (
                  <span
                    className={cx(
                      styles.diff,
                      v === 'better' ? styles.better : v === 'worse' ? styles.worse : styles.same
                    )}
                  >
                    {cell.diff_pct > 0 ? '+' : ''}
                    {cell.diff_pct}%
                  </span>
                )}
              </td>
            )
          })}
        </tr>
      ))}
    </>
  )
}
