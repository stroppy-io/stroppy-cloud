import { runQueries } from '@api/queries/runs'
import type { RunEvent } from '@api/types'
import { AppLink } from '@app/AppLink'
import { ErrorState } from '@app/ErrorState'
import { TelemetryLayout } from '@components/runs/detail/TelemetryLayout'
import { type ActiveFilter, TelemetryToolbar } from '@components/runs/detail/TelemetryToolbar'
import { telemetrySearchSchema } from '@components/runs/detail/telemetry-search'
import { StatusBadge } from '@components/StatusBadge'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Alert,
  Badge,
  Button,
  Icon,
  IconButton,
  Input,
  LoadingPlaceholder,
  MultiSelect,
  Stack,
  Text,
  Tooltip,
  useStyles2,
} from '@grafana/ui'
import { runStatusMeta } from '@helpers/run-status'
import { formatDate, formatDateTime, formatTime, relativeTime } from '@helpers/time'
import { selectedValues } from '@helpers/time-range'
import { useTelemetryRange } from '@hooks/useTelemetryRange'
import { useTenant } from '@hooks/useTenant'
import { useTopic } from '@hooks/useTopic'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { eventKindLabel, eventSubject, eventTitle } from './eventTitle'

export const runEventsSearchSchema = telemetrySearchSchema.extend({
  kind: z.array(z.string()).optional().catch(undefined),
  status: z.array(z.string()).optional().catch(undefined),
  q: z.string().optional().catch(undefined),
})
export type RunEventsSearch = z.infer<typeof runEventsSearchSchema>

type EventsPage = { data: RunEvent[]; meta?: { next_cursor?: string | null; has_more?: boolean } }

const TIME_COL = 128
const TIME_COL_NARROW = 76
const RAIL_COL = 28
const DOT = 20
const LOG_CONTEXT_MS = 30_000

// Two-column timeline: fixed time column | rail with status dots | flexible event card.
// The rail line is one absolutely positioned element per list so cards can be any height.
const getStyles = (theme: GrafanaTheme2) => ({
  scroll: css({ flex: '1 1 auto', minHeight: 0, minWidth: 0, overflow: 'auto' }),
  count: css({
    whiteSpace: 'nowrap',
    fontVariantNumeric: 'tabular-nums',
    display: 'inline-flex',
    alignItems: 'center',
    padding: theme.spacing(0, 0.5),
  }),
  list: css({
    margin: 0,
    padding: 0,
    listStyle: 'none',
    position: 'relative',
    minWidth: 0,
    '&::before': {
      content: '""',
      position: 'absolute',
      top: 0,
      bottom: 0,
      left: TIME_COL + RAIL_COL / 2 + parseFloat(theme.spacing(1.5)) - 1,
      width: 2,
      background: theme.colors.border.weak,
    },
    [theme.breakpoints.down('lg')]: {
      '&::before': {
        left: TIME_COL_NARROW + RAIL_COL / 2 + parseFloat(theme.spacing(1.5)) - 1,
      },
    },
  }),
  row: css({
    display: 'grid',
    gridTemplateColumns: `${TIME_COL}px ${RAIL_COL}px minmax(0, 1fr)`,
    columnGap: theme.spacing(1.5),
    alignItems: 'start',
    paddingBottom: theme.spacing(1.5),
    minWidth: 0,
    [theme.breakpoints.down('lg')]: {
      gridTemplateColumns: `${TIME_COL_NARROW}px ${RAIL_COL}px minmax(0, 1fr)`,
    },
  }),
  day: css({
    display: 'grid',
    gridTemplateColumns: `${TIME_COL}px ${RAIL_COL}px minmax(0, 1fr)`,
    columnGap: theme.spacing(1.5),
    alignItems: 'center',
    padding: theme.spacing(1, 0, 1.5),
    [theme.breakpoints.down('lg')]: {
      gridTemplateColumns: `${TIME_COL_NARROW}px ${RAIL_COL}px minmax(0, 1fr)`,
    },
  }),
  dayLabel: css({
    gridColumn: '1 / 2',
    textAlign: 'right',
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    whiteSpace: 'nowrap',
  }),
  dayPin: css({
    gridColumn: '2 / 3',
    justifySelf: 'center',
    width: 8,
    height: 8,
    borderRadius: theme.shape.radius.circle,
    background: theme.colors.border.medium,
    position: 'relative',
    zIndex: 1,
  }),
  dayRule: css({
    gridColumn: '3 / 4',
    height: 1,
    background: theme.colors.border.weak,
  }),
  time: css({
    textAlign: 'right',
    paddingTop: theme.spacing(1),
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    fontVariantNumeric: 'tabular-nums',
    color: theme.colors.text.primary,
    lineHeight: theme.typography.bodySmall.lineHeight,
    whiteSpace: 'nowrap',
  }),
  timeRel: css({
    display: 'block',
    color: theme.colors.text.secondary,
    [theme.breakpoints.down('lg')]: { display: 'none' },
  }),
  rail: css({
    display: 'grid',
    placeItems: 'center',
    paddingTop: theme.spacing(1),
  }),
  dot: css({
    width: DOT,
    height: DOT,
    borderRadius: theme.shape.radius.circle,
    display: 'grid',
    placeItems: 'center',
    background: theme.colors.background.primary,
    border: `2px solid ${theme.colors.border.medium}`,
    position: 'relative',
    zIndex: 1,
  }),
  dotTone: (tone: string) =>
    css({
      borderColor:
        tone === 'secondary'
          ? theme.colors.border.medium
          : theme.colors[tone as 'success' | 'error' | 'warning' | 'info'].border,
      color:
        tone === 'secondary'
          ? theme.colors.text.secondary
          : theme.colors[tone as 'success' | 'error' | 'warning' | 'info'].text,
    }),
  card: css({
    minWidth: 0,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(1, 1.5),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1),
  }),
  cardError: css({ borderColor: theme.colors.error.border }),
  head: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    flexWrap: 'wrap',
    minWidth: 0,
  }),
  title: css({
    fontWeight: theme.typography.fontWeightMedium,
    minWidth: 0,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  }),
  chip: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    padding: theme.spacing(0, 0.75),
    lineHeight: theme.typography.body.lineHeight,
    whiteSpace: 'nowrap',
    maxWidth: 320,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  kindChip: css({ fontFamily: theme.typography.fontFamily, background: 'transparent' }),
  logLink: css({
    marginLeft: 'auto',
    fontSize: theme.typography.bodySmall.fontSize,
    whiteSpace: 'nowrap',
  }),
  error: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    whiteSpace: 'pre-wrap',
    overflowWrap: 'anywhere',
  }),
  empty: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(4, 2),
  }),
})

function toneOf(ev: RunEvent): string {
  if (ev.error || ev.kind.endsWith('.failed') || ev.status === 'failed') return 'error'
  if (ev.kind.endsWith('.cancelled') || ev.status === 'cancelled' || ev.status === 'cancelling')
    return 'warning'
  if (ev.status) return runStatusMeta(ev.status).tone
  if (ev.kind.endsWith('.finished') || ev.kind.endsWith('.completed') || ev.kind.endsWith('.ready'))
    return 'success'
  if (ev.kind.endsWith('.started')) return 'info'
  return 'secondary'
}

function iconOf(ev: RunEvent) {
  const [group] = ev.kind.split('.')
  switch (group) {
    case 'run':
      return 'play'
    case 'phase':
      return 'layer-group'
    case 'segment':
      return 'rocket'
    case 'machine':
      return 'monitor'
    case 'container':
    case 'component':
      return 'cube'
    case 'activity':
      return 'process'
    default:
      return 'circle'
  }
}

export function RunEventsTab({
  id,
  search,
  onSearchChange,
}: {
  id: string
  search: RunEventsSearch
  onSearchChange: (next: Partial<RunEventsSearch>) => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const range = useTelemetryRange(slug, id, search, onSearchChange)
  const { run, terminal } = range
  const q = useQuery({
    ...runQueries.events(slug, id),
    refetchInterval: terminal ? false : range.refreshMs || 15_000,
  })
  useTopic<{ data: RunEvent[] }, EventsPage>({
    topic: `run.events/${id}`,
    queryKey: runQueries.events(slug, id).queryKey,
    enabled: !!run && !terminal,
    merge: (prev, payload) => {
      const seen = new Set((prev?.data ?? []).map((e) => e.id))
      const fresh = payload.data.filter((e) => !seen.has(e.id))
      return { ...(prev ?? { data: [] }), data: [...(prev?.data ?? []), ...fresh] }
    },
  })
  const all = q.data?.data ?? []
  const kinds = useMemo(() => [...new Set(all.map((e) => e.kind))].sort(), [all])
  const statuses = useMemo(
    () => [...new Set(all.map((e) => e.status).filter((s): s is string => !!s))].sort(),
    [all]
  )
  // The time window applies client-side: the whole event list is one page.
  const windowFrom = range.range.from.valueOf()
  const windowTo = range.range.to.valueOf()
  const filtered = useMemo(() => {
    const needle = search.q?.toLowerCase()
    return all
      .filter((e) => {
        if (range.isDefault) return true
        const at = new Date(e.at).getTime()
        return at >= windowFrom && at <= windowTo
      })
      .filter((e) => !search.kind?.length || search.kind.includes(e.kind))
      .filter((e) => !search.status?.length || (e.status && search.status.includes(e.status)))
      .filter(
        (e) =>
          !needle ||
          `${eventTitle(e, t)} ${e.title} ${e.subject ?? ''} ${e.error ?? ''}`
            .toLowerCase()
            .includes(needle)
      )
      .slice()
      .sort((a, b) => (a.at < b.at ? 1 : -1))
  }, [all, search, t, range.isDefault, windowFrom, windowTo])

  const [draft, setDraft] = useState(search.q ?? '')
  useEffect(() => setDraft(search.q ?? ''), [search.q])

  const filters: ActiveFilter[] = [
    ...(search.kind ?? []).map((v) => ({
      key: `k:${v}`,
      label: `${t('runs.events.kind')}: ${eventKindLabel(v, t)}`,
      onRemove: () => {
        const rest = (search.kind ?? []).filter((x) => x !== v)
        onSearchChange({ kind: rest.length ? rest : undefined })
      },
    })),
    ...(search.status ?? []).map((v) => ({
      key: `s:${v}`,
      label: `${t('runs.events.status')}: ${t(`common.status.${v}`, { defaultValue: v })}`,
      onRemove: () => {
        const rest = (search.status ?? []).filter((x) => x !== v)
        onSearchChange({ status: rest.length ? rest : undefined })
      },
    })),
    ...(search.q
      ? [
          {
            key: 'q',
            label: `${t('common.actions.search')}: ${search.q}`,
            onRemove: () => onSearchChange({ q: undefined }),
          },
        ]
      : []),
  ]
  const clear = () => onSearchChange({ kind: undefined, status: undefined, q: undefined })

  const toolbar = (
    <TelemetryToolbar
      range={range}
      isLoading={q.isFetching}
      onRefresh={() => void q.refetch()}
      filters={filters}
      onResetFilters={clear}
      actions={
        <Text color="secondary" variant="bodySmall">
          <span className={styles.count}>
            {t('runs.events.count', { count: filtered.length, total: all.length })}
          </span>
        </Text>
      }
    >
      <MultiSelect
        aria-label={t('runs.events.kind')}
        placeholder={t('runs.events.kind')}
        width={24}
        closeMenuOnSelect={false}
        options={kinds.map((k) => ({ label: eventKindLabel(k, t), value: k }))}
        value={search.kind ?? []}
        onChange={(o) => {
          const v = selectedValues(o)
          onSearchChange({ kind: v.length ? v : undefined })
        }}
      />
      <MultiSelect
        aria-label={t('runs.events.status')}
        placeholder={t('runs.events.status')}
        width={20}
        closeMenuOnSelect={false}
        options={statuses.map((s) => ({
          label: t(`common.status.${s}`, { defaultValue: s }),
          value: s,
        }))}
        value={search.status ?? []}
        onChange={(o) => {
          const v = selectedValues(o)
          onSearchChange({ status: v.length ? v : undefined })
        }}
      />
      <form
        onSubmit={(e) => {
          e.preventDefault()
          onSearchChange({ q: draft.trim() || undefined })
        }}
      >
        <Input
          width={30}
          prefix={<Icon name="search" />}
          placeholder={t('runs.events.search')}
          value={draft}
          onChange={(e) => setDraft(e.currentTarget.value)}
          suffix={
            draft ? (
              <IconButton
                name="times"
                size="sm"
                tooltip={t('common.actions.clear')}
                onClick={() => {
                  setDraft('')
                  onSearchChange({ q: undefined })
                }}
              />
            ) : undefined
          }
        />
      </form>
    </TelemetryToolbar>
  )

  let lastDay = ''
  return (
    <TelemetryLayout toolbar={toolbar}>
      <div className={styles.scroll}>
        {q.isPending ? (
          <LoadingPlaceholder text={t('common.misc.loading')} />
        ) : q.isError ? (
          <ErrorState error={q.error} onRetry={() => void q.refetch()} compact />
        ) : filtered.length === 0 ? (
          <div className={styles.empty}>
            <Stack direction="column" alignItems="center" gap={1}>
              <Text weight="medium">
                {filters.length || !range.isDefault
                  ? t('common.empty.filtered')
                  : t('runs.events.empty')}
              </Text>
              {filters.length > 0 && (
                <Button size="sm" variant="secondary" onClick={clear}>
                  {t('common.empty.clearFilters')}
                </Button>
              )}
            </Stack>
          </div>
        ) : (
          <ol className={styles.list}>
            {filtered.map((ev) => {
              const at = new Date(ev.at)
              const day = at.toDateString()
              const showDay = day !== lastDay
              lastDay = day
              const tone = toneOf(ev)
              const from = String(at.getTime() - LOG_CONTEXT_MS)
              const to = String(at.getTime() + LOG_CONTEXT_MS)
              const subject = eventSubject(ev, t)
              return (
                <li key={ev.id}>
                  {showDay && (
                    <div className={styles.day}>
                      <span className={styles.dayLabel}>{formatDate(ev.at)}</span>
                      <span className={styles.dayPin} aria-hidden />
                      <span className={styles.dayRule} aria-hidden />
                    </div>
                  )}
                  <div className={styles.row}>
                    <div className={styles.time}>
                      <Tooltip content={formatDateTime(ev.at)}>
                        <span>
                          {formatTime(ev.at)}
                          <span className={styles.timeRel}>{relativeTime(ev.at)}</span>
                        </span>
                      </Tooltip>
                    </div>
                    <div className={styles.rail}>
                      <div className={cx(styles.dot, styles.dotTone(tone))}>
                        <Icon name={iconOf(ev)} size="xs" />
                      </div>
                    </div>
                    <div className={cx(styles.card, ev.error && styles.cardError)}>
                      <div className={styles.head}>
                        <span className={styles.title} title={eventTitle(ev, t)}>
                          {eventTitle(ev, t)}
                        </span>
                        {subject && (
                          <span className={styles.chip} title={ev.subject}>
                            {subject}
                          </span>
                        )}
                        <span className={cx(styles.chip, styles.kindChip)} title={ev.kind}>
                          {eventKindLabel(ev.kind, t)}
                        </span>
                        {ev.status && <StatusBadge status={ev.status} />}
                        {ev.attempt && ev.attempt > 1 && (
                          <Badge
                            text={t('runs.events.attempt', { n: ev.attempt })}
                            color="orange"
                            icon="repeat"
                          />
                        )}
                        <span className={styles.logLink}>
                          <AppLink
                            to="/t/$slug/runs/$id/logs"
                            params={{ slug, id }}
                            search={
                              {
                                from,
                                to,
                                ...(ev.log_ref?.filter ?? {}),
                              } as never
                            }
                          >
                            <Icon name="file-alt" size="sm" /> {t('runs.events.viewLogs')}
                          </AppLink>
                        </span>
                      </div>
                      {ev.error && (
                        <Alert severity="error" title={t('runs.events.error')} bottomSpacing={0}>
                          <span className={styles.error}>{ev.error}</span>
                        </Alert>
                      )}
                    </div>
                  </div>
                </li>
              )
            })}
          </ol>
        )}
      </div>
    </TelemetryLayout>
  )
}
