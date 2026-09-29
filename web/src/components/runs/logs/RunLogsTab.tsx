import { type LogQuery, runQueries } from '@api/queries/runs'
import type { LogLine, LogPage } from '@api/types'
import { ErrorState } from '@app/ErrorState'
import { toast } from '@app/Toaster'
import { TelemetryLayout } from '@components/runs/detail/TelemetryLayout'
import { telemetrySearchSchema } from '@components/runs/detail/telemetry-search'
import { useTheme2 } from '@grafana/ui'
import {
  formatLogLine,
  type LogLineFilter,
  logLineMatches,
  mergeLogLines,
  sortChronological,
} from '@helpers/log-lines'
import { useTelemetryRange } from '@hooks/useTelemetryRange'
import { useTenant } from '@hooks/useTenant'
import { useTopic } from '@hooks/useTopic'
import { downloadText } from '@lib/download'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { LogsRawDrawer } from './LogsRawDrawer'
import { LOG_LIMIT_DEFAULT, LOG_LIMITS, LogsToolbar } from './LogsToolbar'
import { LogView, type LogViewHandle } from './LogView'

const arr = () => z.array(z.string()).optional().catch(undefined)
const str = () => z.string().optional().catch(undefined)
// URL = source of truth: every filter, the search phrase, the page size, the deep-linked line
// (`line` = server `seq`) and the shared telemetry window (`from`/`to`/`refresh`).
export const runLogsSearchSchema = telemetrySearchSchema.extend({
  role: arr(),
  machine: arr(),
  container: arr(),
  stream: arr(),
  phase: arr(),
  level: arr(),
  segment: str(),
  q: str(),
  // Line `seq` (< 2^53, exact as a JS number). Kept numeric so the URL stays `?line=<seq>`:
  // a digit-only string would be re-serialized by the router as `?line="<seq>"`.
  line: z.coerce.number().int().nonnegative().optional().catch(undefined),
  limit: z
    .union([z.literal(LOG_LIMITS[0]), z.literal(LOG_LIMITS[1]), z.literal(LOG_LIMITS[2])])
    .optional()
    .catch(undefined),
})
export type RunLogsSearch = z.infer<typeof runLogsSearchSchema>

const MAX_LINES = 5000
const NEWER_RETRY_MS = 3000
// Stable per-machine colors from the theme palette (assigned in order of appearance).
const MACHINE_PALETTE = [
  'blue',
  'green',
  'orange',
  'purple',
  'red',
  'yellow',
  'semi-dark-blue',
  'semi-dark-green',
  'semi-dark-orange',
  'semi-dark-purple',
  'light-blue',
  'light-green',
]

function toFilter(s: RunLogsSearch, start: string, end: string | undefined): LogLineFilter {
  return {
    role: s.role,
    machine: s.machine,
    container: s.container,
    stream: s.stream,
    phase: s.phase,
    level: s.level,
    segment: s.segment,
    q: s.q,
    start,
    end,
  }
}

// The stream delivers one line per event on the server and `{ data: [...] }` in the mock.
function linesOf(payload: unknown): LogLine[] {
  if (!payload || typeof payload !== 'object') return []
  if ('data' in payload && Array.isArray((payload as { data: unknown }).data))
    return (payload as { data: LogLine[] }).data
  if ('message' in payload && 'time' in payload) return [payload as LogLine]
  return []
}

export function RunLogsTab({
  id,
  search,
  onSearchChange,
}: {
  id: string
  search: RunLogsSearch
  onSearchChange: (next: Partial<RunLogsSearch>) => void
}) {
  const theme = useTheme2()
  const { t } = useTranslation()
  const { slug } = useTenant()
  const qc = useQueryClient()
  const range = useTelemetryRange(slug, id, search, onSearchChange)
  const { run, terminal } = range
  const limit = search.limit ?? LOG_LIMIT_DEFAULT
  const filter = useMemo(
    () => toFilter(search, range.start, range.end),
    [search, range.start, range.end]
  )
  const query = useMemo(() => ({ ...filter, limit }) as LogQuery, [filter, limit])
  const anchor = search.line !== undefined ? String(search.line) : undefined
  const bufferOpts = runQueries.logBuffer(slug, id, query, anchor)
  const key = bufferOpts.queryKey
  const logs = useQuery(bufferOpts)
  const facets = useQuery({
    ...runQueries.logFacets(slug, id),
    refetchInterval: terminal ? false : range.refreshMs || 30_000,
  })
  // Live tail needs a window that ends at "now" and no anchored line.
  const liveAllowed = !!run && !terminal && range.isLive && !search.line
  const [live, setLive] = useState(true)
  const [follow, setFollow] = useState(true)
  const [wrap, setWrap] = useState(false)
  const [raw, setRaw] = useState(false)
  const isLive = live && liveAllowed
  const viewRef = useRef<LogViewHandle>(null)
  const lines = logs.data?.data ?? []

  // ── live tail into the same cache entry ──
  const filterRef = useRef(filter)
  filterRef.current = filter
  useTopic<unknown, LogPage>({
    topic: `run.logs/${id}`,
    queryKey: key,
    enabled: isLive && !!logs.data,
    batchMs: 150,
    merge: (prev, payload) => {
      const base: LogPage = prev ?? { data: [] }
      const fresh = linesOf(payload).filter((l) => logLineMatches(l, filterRef.current))
      if (!fresh.length) return base
      const data = mergeLogLines(base.data, fresh)
      if (data === base.data) return base
      let older = base.older
      if (data.length > MAX_LINES) {
        data.splice(0, data.length - MAX_LINES)
        // Trimmed history stays reachable: page it back from the first kept line.
        const first = data[0]?.seq
        older = first !== undefined ? String(first) : older
      }
      const last = data[data.length - 1]?.seq
      return { ...base, data, older, newer: last !== undefined ? String(last) : base.newer }
    },
  })

  // ── paging ──
  const fetchPage = (extra: LogQuery) =>
    qc.fetchQuery({
      ...runQueries.logs(slug, id, { ...query, ...extra } as LogQuery),
      staleTime: 0,
    }) as Promise<LogPage>
  const older = useMutation({
    mutationFn: async () => {
      const cursor = qc.getQueryData<LogPage>(key)?.older
      if (!cursor) return
      const page = await fetchPage({ cursor, direction: 'older' })
      qc.setQueryData<LogPage>(key, (prev) => ({
        ...(prev ?? page),
        data: mergeLogLines(prev?.data ?? [], page.data, true),
        older: page.data.length ? (page.older ?? null) : null,
      }))
    },
    onError: (e) => toast.error(e),
  })
  const newerAt = useRef(0)
  const newer = useMutation({
    mutationFn: async () => {
      const cursor = qc.getQueryData<LogPage>(key)?.newer
      if (!cursor) return
      newerAt.current = Date.now()
      const page = await fetchPage({ cursor, direction: 'newer' })
      qc.setQueryData<LogPage>(key, (prev) => ({
        ...(prev ?? page),
        data: mergeLogLines(prev?.data ?? [], page.data),
        newer: page.data.length ? (page.newer ?? prev?.newer) : prev?.newer,
      }))
    },
    onError: (e) => toast.error(e),
  })
  const handleReachTop = useCallback(() => {
    if (!older.isPending && logs.data?.older) older.mutate()
  }, [older, logs.data?.older])
  const handleReachBottom = useCallback(() => {
    if (isLive || newer.isPending || !logs.data?.newer) return
    if (Date.now() - newerAt.current < NEWER_RETRY_MS) return
    newer.mutate()
  }, [isLive, newer, logs.data?.newer])

  // ── auto-refresh (RefreshPicker): pull the newer page, or reload the tail without a cursor ──
  const handleRefresh = useCallback(() => {
    if (logs.data?.newer && !search.line) newer.mutate()
    else void logs.refetch()
    void facets.refetch()
  }, [logs, newer, facets, search.line])
  const refreshRef = useRef(handleRefresh)
  refreshRef.current = handleRefresh
  useEffect(() => {
    if (!range.refreshMs || isLive) return
    const h = window.setInterval(() => refreshRef.current(), range.refreshMs)
    return () => window.clearInterval(h)
  }, [range.refreshMs, isLive])

  // ── Home / End replace the buffer with the absolute oldest / newest page ──
  const jump = useRef<'top' | 'bottom' | null>(null)
  // biome-ignore lint/correctness/useExhaustiveDependencies: runs once the replaced buffer has rendered
  useLayoutEffect(() => {
    if (!jump.current) return
    if (jump.current === 'top') viewRef.current?.scrollToTop()
    else viewRef.current?.scrollToBottom()
    jump.current = null
  }, [lines])
  const home = useMutation({
    mutationFn: async () => {
      setLive(false)
      const page = await fetchPage({ direction: 'newer' })
      jump.current = 'top'
      qc.setQueryData<LogPage>(key, {
        data: sortChronological(page.data),
        older: null,
        newer: page.data.length ? page.newer : null,
        truncated: page.truncated,
      })
    },
    onError: (e) => toast.error(e),
  })
  const end = useMutation({
    mutationFn: async () => {
      if (search.line) {
        // Leaving the anchored view: the tail key loads and follows on its own.
        setLive(true)
        setFollow(true)
        onSearchChange({ line: undefined })
        return
      }
      const page = await fetchPage({ direction: 'older' })
      jump.current = 'bottom'
      qc.setQueryData<LogPage>(key, { ...page, data: sortChronological(page.data) })
      setLive(true)
      setFollow(true)
    },
    onError: (e) => toast.error(e),
  })

  // ── per-machine colors ──
  const machineColors = useMemo(() => {
    const names = new Set<string>()
    for (const v of facets.data?.data.find((f) => f.field === 'machine')?.values ?? [])
      names.add(v.value)
    for (const l of lines) if (l.machine) names.add(l.machine)
    const map = new Map<string, string>()
    let i = 0
    for (const n of names)
      map.set(n, theme.visualization.getColorByName(MACHINE_PALETTE[i++ % MACHINE_PALETTE.length]))
    return map
  }, [facets.data, lines, theme])

  const handleCopyLink = useCallback((l: LogLine) => {
    if (l.seq === undefined) return
    const url = new URL(window.location.href)
    url.searchParams.set('line', String(l.seq))
    void navigator.clipboard.writeText(url.toString()).catch(() => undefined)
  }, [])

  const handleDownload = () =>
    downloadText(`logs-${id}.txt`, lines.map(formatLogLine).join('\n'), 'text/plain')

  const isFiltered =
    Object.entries(filter).some(
      ([k, v]) => k !== 'start' && k !== 'end' && (Array.isArray(v) ? v.length > 0 : !!v)
    ) ||
    !!search.line ||
    !range.isDefault
  const emptyText = logs.isPending
    ? t('common.misc.loading')
    : isFiltered
      ? t('common.empty.filtered')
      : terminal
        ? t('runs.logs.empty')
        : t('runs.logs.waiting')

  return (
    <TelemetryLayout
      toolbar={
        <LogsToolbar
          search={search}
          onSearchChange={onSearchChange}
          range={range}
          facets={facets.data?.data}
          segments={
            (run?.snapshot.workload.segments as { name?: string }[] | undefined)
              ?.map((s) => s.name)
              .filter((n): n is string => !!n) ?? []
          }
          lineCount={lines.length}
          truncated={!!logs.data?.truncated || !!logs.data?.older}
          isLoading={logs.isFetching || newer.isPending || older.isPending}
          onRefresh={handleRefresh}
          live={live}
          liveAllowed={liveAllowed}
          onLiveChange={(v) => {
            setLive(v)
            if (v) {
              setFollow(true)
              viewRef.current?.scrollToBottom()
            }
          }}
          wrap={wrap}
          onWrapChange={setWrap}
          canOlder={!!logs.data?.older && !older.isPending}
          canNewer={!!logs.data?.newer && !newer.isPending && !isLive}
          onOlder={() => older.mutate()}
          onNewer={() => newer.mutate()}
          onDownload={handleDownload}
          onOpenRaw={() => setRaw(true)}
        />
      }
    >
      {logs.isError && (
        <ErrorState error={logs.error} onRetry={() => void logs.refetch()} compact />
      )}
      <LogView
        ref={viewRef}
        lines={lines}
        wrap={wrap}
        follow={isLive && follow}
        onFollowChange={setFollow}
        canFollow={isLive}
        height="100%"
        emptyText={emptyText}
        highlight={search.q}
        anchorSeq={anchor}
        machineColors={machineColors}
        onReachTop={handleReachTop}
        onReachBottom={handleReachBottom}
        loadingOlder={older.isPending}
        loadingNewer={newer.isPending || home.isPending || end.isPending}
        onHome={() => home.mutate()}
        onEnd={() => end.mutate()}
        onCopyLink={handleCopyLink}
      />
      {raw && (
        <LogsRawDrawer
          slug={slug}
          runId={id}
          start={range.start}
          end={range.end}
          onClose={() => setRaw(false)}
        />
      )}
    </TelemetryLayout>
  )
}
