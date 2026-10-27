import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, useStyles2 } from '@grafana/ui'
import { formatMetric, formatNumber, formatPercent } from '@helpers/format'
import { formatTime } from '@helpers/time'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { SimpleTable, SubTitle } from './ReportPrimitives'
import { clientWait, errorKinds, latencyRows, type ResultSegment } from './segment-metrics'

const getStyles = (theme: GrafanaTheme2) => ({
  hint: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    margin: theme.spacing(0, 0, 1),
  }),
  segment: css({ margin: theme.spacing(2, 0, 0.5) }),
  lead: css({ fontWeight: theme.typography.fontWeightMedium }),
  kindsHead: css({
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'baseline',
    gap: theme.spacing(2),
    margin: theme.spacing(2, 0, 0.5),
    fontWeight: theme.typography.fontWeightMedium,
  }),
  warn: css({ color: theme.colors.warning.text }),
  sub: css({ color: theme.colors.text.secondary, fontSize: theme.typography.bodySmall.fontSize }),
  sql: css({
    display: 'block',
    marginTop: theme.spacing(0.5),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-word',
  }),
  when: css({ whiteSpace: 'nowrap', color: theme.colors.text.secondary }),
})

/** Latency per segment and transaction type, with the time the client spent outside the database. */
export function LatencyPanel({ segments }: { segments?: ResultSegment[] }) {
  const { t, i18n } = useTranslation()
  const styles = useStyles2(getStyles)
  const withRows = (segments ?? [])
    .map((s) => ({ s, rows: latencyRows(s) }))
    .filter((x) => x.rows.length > 0)
  if (!withRows.length) return null
  const label = (name: string) =>
    i18n.exists(`report.latency.names.${name}`)
      ? t(`report.latency.names.${name}`)
      : name.replace(/^tpcc_/, '')
  return (
    <>
      <SubTitle>{t('report.latency.title')}</SubTitle>
      <p className={styles.hint}>{t('report.latency.hint')}</p>
      {withRows.map(({ s, rows }) => {
        const wait = clientWait(s.metrics)
        return (
          <div key={s.name}>
            {withRows.length > 1 && <div className={styles.segment}>{s.name}</div>}
            {wait && (
              <Alert severity="warning" title="">
                {t('report.latency.clientWait', {
                  ms: formatMetric(wait.ms, 'ms'),
                  share: formatPercent(wait.share * 100, 0),
                })}
              </Alert>
            )}
            <SimpleTable>
              <thead>
                <tr>
                  <th>{t('report.latency.name')}</th>
                  <th className="num">{t('report.latency.count')}</th>
                  <th className="num">{t('report.latency.avg')}</th>
                  <th className="num">p50</th>
                  <th className="num">p95</th>
                  <th className="num">p99</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={r.name}>
                    <td
                      className={r.name === 'iteration' || r.name === 'tx_total' ? styles.lead : ''}
                    >
                      {label(r.name)}
                    </td>
                    <td className="num">{formatNumber(r.count)}</td>
                    <td className="num">{formatMetric(r.avg, 'ms')}</td>
                    <td className="num">{formatMetric(r.p50, 'ms')}</td>
                    <td className="num">{formatMetric(r.p95, 'ms')}</td>
                    <td className="num">{formatMetric(r.p99, 'ms')}</td>
                  </tr>
                ))}
              </tbody>
            </SimpleTable>
          </div>
        )
      })}
    </>
  )
}

/**
 * Failed transactions, retries and the rollbacks TPC-C makes on purpose,
 * per segment, then which errors those were. logsLink renders a link to
 * the segment's error lines where the viewer can read logs (the run page).
 */
export function ErrorsPanel({
  segments,
  logsLink,
}: {
  segments?: ResultSegment[]
  logsLink?: (segment: string) => ReactNode
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const rows = (segments ?? []).filter((s) => s.errors || s.metrics?.tpcc_rollback_done)
  if (!rows.length) return null
  const kinds = rows.map((s) => ({ s, kinds: errorKinds(s) })).filter((x) => x.kinds.length > 0)
  return (
    <>
      <SubTitle>{t('report.errors.title')}</SubTitle>
      <p className={styles.hint}>{t('report.errors.hint')}</p>
      <SimpleTable>
        <thead>
          <tr>
            <th>{t('report.errors.segment')}</th>
            <th className="num">{t('report.errors.failed')}</th>
            <th className="num">{t('report.errors.failedQueries')}</th>
            <th className="num">{t('report.errors.retries')}</th>
            <th className="num">{t('report.errors.rollbacks')}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((s) => (
            <tr key={s.name}>
              <td className="mono">{s.name}</td>
              <td className="num">{formatNumber(s.errors?.terminal_errors)}</td>
              <td className="num">{formatNumber(s.errors?.failed_queries)}</td>
              <td className="num">{formatNumber(s.errors?.retry_attempts)}</td>
              <td className="num">{formatNumber(s.metrics?.tpcc_rollback_done?.value)}</td>
            </tr>
          ))}
        </tbody>
      </SimpleTable>
      {kinds.map(({ s, kinds: list }) => (
        <div key={s.name}>
          <div className={styles.kindsHead}>
            <span>{t('report.errors.kinds', { segment: s.name })}</span>
            {logsLink?.(s.name)}
          </div>
          {!s.error_log?.length && <p className={styles.hint}>{t('report.errors.noLog')}</p>}
          <SimpleTable>
            <thead>
              <tr>
                <th className="num">{t('report.errors.count')}</th>
                <th>{t('report.errors.error')}</th>
                <th>{t('report.errors.class')}</th>
                <th>{t('report.errors.when')}</th>
              </tr>
            </thead>
            <tbody>
              {list.map((k) => (
                <tr key={`${k.level}/${k.message}/${k.error}/${k.class}`}>
                  <td className="num">{formatNumber(k.count)}</td>
                  <td>
                    <div className={k.level === 'error' ? undefined : styles.warn}>
                      {k.error || k.message || '—'}
                    </div>
                    {k.error && k.message && <div className={styles.sub}>{k.message}</div>}
                    {k.sql && <code className={styles.sql}>{k.sql}</code>}
                  </td>
                  <td className="mono">{k.class || '—'}</td>
                  <td className={styles.when}>
                    {k.first_at
                      ? k.last_at && k.last_at !== k.first_at
                        ? `${formatTime(k.first_at)} – ${formatTime(k.last_at)}`
                        : formatTime(k.first_at)
                      : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </SimpleTable>
        </div>
      ))}
    </>
  )
}
