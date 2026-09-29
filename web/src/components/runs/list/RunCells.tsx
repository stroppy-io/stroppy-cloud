import type { Run } from '@api/types'
import { Dash } from '@components/DataTable/cells'
import { useDensity } from '@components/DataTable/prefs'
import { RelativeTime } from '@components/RelativeTime'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import { Icon, Sparkline, Tooltip, useStyles2, useTheme2 } from '@grafana/ui'
import { seriesToFrame } from '@helpers/dataframe'
import { durationSeconds, formatDuration, formatMetric } from '@helpers/format'
import { formatTime } from '@helpers/time'
import type { TFunction } from 'i18next'
import { type ReactNode, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

// Cells of the runs table (web/docs/tables-guide.md §14). Pure views over a `Run`; the column
// definitions in `RunListPage` only place them.

interface Segment {
  name?: string
  script?: string
  duration?: number
  warmup?: number
  vus?: number
}

// Workload segments from the run snapshot (the list payload carries it).
export function runSegments(r: Run): Segment[] {
  const segs = (r.snapshot?.workload?.segments ?? []) as {
    name?: string
    warmup?: string
    workload?: { script?: string }
    run?: { duration?: string; vus?: number }
  }[]
  return segs.map((s) => ({
    name: s.name,
    script: s.workload?.script,
    duration: durationSeconds(s.run?.duration),
    warmup: durationSeconds(s.warmup),
    vus: s.run?.vus,
  }))
}

export function workloadView(r: Run, t: TFunction) {
  const segs = runSegments(r)
  const scripts = [...new Set(segs.map((s) => s.script).filter(Boolean))] as string[]
  const main = r.summary?.workload_name ?? r.snapshot?.workload_name ?? scripts[0] ?? segs[0]?.name
  const primary = main ? `${main}${scripts.length > 1 ? ` +${scripts.length - 1}` : ''}` : undefined
  const vus = Math.max(0, ...segs.map((s) => s.vus ?? 0))
  const secondary = [
    r.summary?.protocol,
    r.summary?.stroppy_version ? `stroppy ${r.summary.stroppy_version}` : undefined,
    segs.length ? t('runs.list.segments', { count: segs.length }) : undefined,
    vus ? t('runs.list.vus', { count: vus }) : undefined,
  ]
    .filter(Boolean)
    .join(' · ')
  const title = segs
    .map(
      (s) =>
        `${s.name ?? '—'}: ${s.script ?? '—'}${s.duration ? `, ${formatDuration(s.duration)}` : ''}${s.vus ? `, ${s.vus} VU` : ''}`
    )
    .join('\n')
  return { primary, secondary, title }
}

export function databaseView(r: Run, t: TFunction) {
  const s = r.summary
  if (!s?.db_kind) return { primary: undefined }
  const primary = `${s.db_kind}${s.db_version ? ` ${s.db_version}` : ''}`
  const secondary = [
    s.topology_label,
    s.node_count ? t('runs.list.nodes', { count: s.node_count }) : undefined,
  ]
    .filter(Boolean)
    .join(' · ')
  return { primary, secondary, title: secondary ? `${primary}\n${secondary}` : primary }
}

export function providerView(r: Run) {
  const s = r.summary
  const sizes = Object.entries((s?.sizes ?? {}) as Record<string, { size?: string }>)
    .map(([role, v]) => `${role}:${v?.size ?? '?'}`)
    .join(' · ')
  const primary = s?.provider_profile?.name
  return {
    primary,
    secondary: sizes || undefined,
    title: [primary, sizes].filter(Boolean).join('\n') || undefined,
  }
}

// Throughput points for the sparkline. The server stores a short series in the summary;
// tolerate both `{ t, v }[]` and `[t, v][]`.
function qpsPoints(r: Run): number[][] {
  const raw = (r.summary as { qps_series?: unknown } | undefined)?.qps_series
  if (!Array.isArray(raw)) return []
  const out: number[][] = []
  for (const p of raw) {
    if (Array.isArray(p) && p.length >= 2) out.push([Number(p[0]), Number(p[1])])
    else if (p && typeof p === 'object' && 't' in p && 'v' in p)
      out.push([Number((p as { t: unknown }).t), Number((p as { v: unknown }).v)])
  }
  return out.filter((p) => Number.isFinite(p[0]) && Number.isFinite(p[1]))
}

function headline(r: Run, key: string): number | undefined {
  const v = r.summary?.headline?.[key]
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined
}

const getStyles = (theme: GrafanaTheme2) => ({
  metrics: css({ display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0 }),
  top: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1), minWidth: 0 }),
  spark: css({ flex: '0 0 auto', display: 'inline-flex' }),
  num: css({
    marginLeft: 'auto',
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    fontVariantNumeric: 'tabular-nums',
    whiteSpace: 'nowrap',
  }),
  sub: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    fontFamily: theme.typography.fontFamilyMonospace,
    fontVariantNumeric: 'tabular-nums',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    textAlign: 'right',
  }),
  err: css({ color: theme.colors.error.text }),
  empty: css({ textAlign: 'right' }),
  time: css({ display: 'flex', flexDirection: 'column', minWidth: 0 }),
  timeSub: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  eta: css({ color: theme.colors.info.text }),
  late: css({ color: theme.colors.warning.text }),
})

const SPARK_W = 72
const SPARK_H = 20

// «Показатели»: throughput sparkline + QPS over p99 · p50 · errors.
export function RunMetricsCell({ run }: { run: Run }) {
  const styles = useStyles2(getStyles)
  const theme = useTheme2()
  const density = useDensity()
  const { t } = useTranslation()
  const points = useMemo(() => qpsPoints(run), [run])
  const frame = useMemo(
    () => (points.length > 1 ? seriesToFrame('qps', points, '1/s', theme) : undefined),
    [points, theme]
  )
  const qps = headline(run, 'qps') ?? headline(run, 'tps')
  const p99 = headline(run, 'latency_p99_ms')
  const p50 = headline(run, 'latency_p50_ms')
  const errors = headline(run, 'errors')
  if (qps === undefined && p99 === undefined && p50 === undefined)
    return (
      <div className={styles.empty}>
        <Dash />
      </div>
    )
  const unit = headline(run, 'qps') !== undefined ? 'qps' : 'tps'
  const sub = [
    p99 !== undefined ? `p99 ${formatMetric(p99, 'ms')}` : undefined,
    p50 !== undefined ? `p50 ${formatMetric(p50, 'ms')}` : undefined,
  ]
    .filter(Boolean)
    .join(' · ')
  return (
    <div className={styles.metrics}>
      <div className={styles.top}>
        {frame && (
          <span className={styles.spark} aria-hidden>
            <Sparkline
              width={SPARK_W}
              height={SPARK_H}
              theme={theme}
              sparkline={{ x: frame.fields[0], y: frame.fields[1] }}
              config={{
                ...frame.fields[1].config,
                custom: { drawStyle: 'line', lineWidth: 1, fillOpacity: 15 } as never,
                color: { mode: 'fixed' as never, fixedColor: theme.colors.primary.main },
              }}
            />
          </span>
        )}
        <span className={styles.num}>
          {qps !== undefined ? `${formatMetric(qps, 'count')} ${unit}` : '—'}
        </span>
      </div>
      {density !== 'compact' && (sub || errors) && (
        <div className={styles.sub}>
          {sub}
          {errors ? (
            <span className={styles.err}>
              {sub ? ' · ' : ''}
              {t('runs.list.errors', { count: errors })}
            </span>
          ) : null}
        </div>
      )}
    </div>
  )
}

function useNow(ms: number, enabled: boolean): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!enabled) return
    const id = window.setInterval(() => setNow(Date.now()), ms)
    return () => window.clearInterval(id)
  }, [ms, enabled])
  return now
}

// «Время»: start over (running) time left / (finished) finish · duration.
export function RunTimeCell({ run }: { run: Run }) {
  const styles = useStyles2(getStyles)
  const density = useDensity()
  const { t } = useTranslation()
  const running = run.status === 'running' || run.status === 'cancelling'
  const now = useNow(30_000, running)
  const start = run.started_at ?? undefined
  if (run.status === 'pending' || !start)
    return <span className={styles.timeSub}>{t('runs.list.queued')}</span>

  let sub: ReactNode
  if (running) {
    const eta = (run.summary as { expected_finish_at?: string | null } | undefined)
      ?.expected_finish_at
    const left = eta ? (new Date(eta).getTime() - now) / 1000 : undefined
    const elapsed = (now - new Date(start).getTime()) / 1000
    sub =
      left === undefined ? (
        t('runs.list.elapsed', { time: formatDuration(elapsed) })
      ) : left > 0 ? (
        <span className={styles.eta}>
          {t('runs.list.remaining', { time: formatDuration(left) })}
        </span>
      ) : (
        <span className={styles.late}>
          {t('runs.list.overdue', { time: formatDuration(-left) })}
        </span>
      )
  } else {
    const dur = durationSeconds(run.duration)
    sub = [
      run.finished_at
        ? t('runs.list.finishedAt', { time: formatTime(run.finished_at).slice(0, 5) })
        : undefined,
      dur !== undefined ? formatDuration(dur) : undefined,
    ]
      .filter(Boolean)
      .join(' · ')
  }
  return (
    <div className={styles.time}>
      <RelativeTime value={start} />
      {density !== 'compact' && sub && <span className={cx(styles.timeSub)}>{sub}</span>}
    </div>
  )
}

// Trigger marker before the run name; manual runs (the common case) get none.
export function TriggerMark({ trigger, icon }: { trigger: string; icon: IconName }) {
  const { t } = useTranslation()
  const label = t(`common.trigger.${trigger}`)
  return (
    <Tooltip content={label}>
      <Icon name={icon} aria-label={label} />
    </Tooltip>
  )
}
