import { runQueries } from '@api/queries/runs'
import type { TimeOption, TimeRange } from '@grafana/data'
import { isTerminal } from '@helpers/run-status'
import {
  isRangeLive,
  isRunWindow,
  moveRange,
  REFRESH_INTERVALS,
  type RefreshInterval,
  rangeToApi,
  rawFromSearch,
  rawToSearch,
  refreshToMs,
  resolveRange,
  zoomOutRange,
} from '@helpers/time-range'
import { useQuery } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useRun } from './useRun'

export interface TelemetryRangeSearch {
  from?: string
  to?: string
  refresh?: RefreshInterval
}

function asRefresh(value: string): RefreshInterval | undefined {
  return (REFRESH_INTERVALS as readonly string[]).includes(value)
    ? (value as RefreshInterval)
    : undefined
}

const LIVE_TICK_MS = 10_000

// The telemetry window of a run tab. Reads `from`/`to`/`refresh` from the URL, resolves them
// against the run (default = the whole run), and gives the picker its handlers. Live windows
// (`to: now`) re-resolve on a tick so the picker label and chart axes keep moving.
export function useTelemetryRange(
  slug: string,
  id: string,
  search: TelemetryRangeSearch,
  onSearchChange: (next: Partial<TelemetryRangeSearch>) => void
) {
  const { t } = useTranslation()
  const run = useRun(slug, id).data
  const terminal = isTerminal(run?.status)
  const overview = useQuery({ ...runQueries.overview(slug, id), enabled: !!run })
  const window = useMemo(
    () => ({ startedAt: run?.started_at ?? run?.created_at, finishedAt: run?.finished_at }),
    [run?.started_at, run?.created_at, run?.finished_at]
  )
  const raw = useMemo(() => rawFromSearch(search, window), [search, window])
  const refreshMs = terminal ? false : refreshToMs(search.refresh)

  const [tick, setTick] = useState(0)
  const liveRaw = raw.to === 'now'
  useEffect(() => {
    if (!liveRaw || terminal) return
    const every = refreshMs || LIVE_TICK_MS
    const h = globalThis.setInterval(() => setTick((n) => n + 1), every)
    return () => globalThis.clearInterval(h)
  }, [liveRaw, terminal, refreshMs])
  // biome-ignore lint/correctness/useExhaustiveDependencies: `tick` re-resolves "now"
  const range: TimeRange = useMemo(() => resolveRange(raw), [raw, tick])
  const api = useMemo(() => rangeToApi(range), [range])

  const setRange = useCallback(
    (next: TimeRange) => {
      if (isRunWindow(next.raw, window)) onSearchChange({ from: undefined, to: undefined })
      else onSearchChange(rawToSearch(next))
    },
    [onSearchChange, window]
  )
  const handleZoom = useCallback(() => setRange(zoomOutRange(range)), [range, setRange])
  const handleMoveBackward = useCallback(() => setRange(moveRange(range, -1)), [range, setRange])
  const handleMoveForward = useCallback(() => setRange(moveRange(range, 1)), [range, setRange])
  const setRefresh = useCallback(
    (value: string) => onSearchChange({ refresh: asRefresh(value) }),
    [onSearchChange]
  )

  // Quick ranges: the run itself, its workload phase and each segment, then the relative set
  // (live runs only — "last 15 min" of a finished run is empty).
  const quickRanges = useMemo<TimeOption[]>(() => {
    const ms = (iso: string | null | undefined) =>
      iso ? String(new Date(iso).getTime()) : undefined
    const opts: TimeOption[] = []
    if (run?.started_at)
      opts.push({
        from: ms(run.started_at) ?? 'now-1h',
        to: ms(run.finished_at) ?? 'now',
        display: t('runs.telemetry.ranges.wholeRun'),
      })
    const wl = overview.data?.phases.find((p) => p.id === 'workload')
    if (wl?.started_at)
      opts.push({
        from: ms(wl.started_at) ?? 'now-1h',
        to: ms(wl.finished_at) ?? 'now',
        display: t('runs.telemetry.ranges.workload'),
      })
    for (const s of overview.data?.workload_segments ?? [])
      if (s.started_at)
        opts.push({
          from: ms(s.started_at) ?? 'now-1h',
          to: ms(s.finished_at) ?? 'now',
          display: t('runs.telemetry.ranges.segment', { name: s.name }),
        })
    if (!terminal)
      for (const [from, key] of [
        ['now-5m', 'last5m'],
        ['now-15m', 'last15m'],
        ['now-30m', 'last30m'],
        ['now-1h', 'last1h'],
        ['now-3h', 'last3h'],
        ['now-6h', 'last6h'],
      ] as const)
        opts.push({ from, to: 'now', display: t(`runs.telemetry.ranges.${key}`) })
    return opts
  }, [run, overview.data, terminal, t])

  return {
    run,
    terminal,
    raw,
    range,
    // ISO bounds for queries; `end` is absent while the window ends at "now".
    start: api.start,
    end: api.end,
    isLive: !terminal && isRangeLive(range),
    isDefault: !search.from && !search.to,
    refresh: search.refresh ?? '',
    refreshMs,
    quickRanges,
    setRange,
    setRefresh,
    handleZoom,
    handleMoveBackward,
    handleMoveForward,
  }
}

export type TelemetryRange = ReturnType<typeof useTelemetryRange>
