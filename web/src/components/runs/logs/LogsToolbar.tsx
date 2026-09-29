import type { Facet } from '@api/types'
import { type ActiveFilter, TelemetryToolbar } from '@components/runs/detail/TelemetryToolbar'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Button,
  ButtonGroup,
  Icon,
  IconButton,
  InlineSwitch,
  Input,
  MultiSelect,
  Select,
  Text,
  ToolbarButton,
  Tooltip,
  useStyles2,
} from '@grafana/ui'
import { selectedValues } from '@helpers/time-range'
import type { TelemetryRange } from '@hooks/useTelemetryRange'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { RunLogsSearch } from './RunLogsTab'

export const LOG_FACET_FIELDS = [
  'level',
  'role',
  'machine',
  'container',
  'phase',
  'stream',
] as const
export type LogFacetField = (typeof LOG_FACET_FIELDS)[number]
// Page sizes offered to the reader; the server caps `limit` at 200.
export const LOG_LIMITS = [50, 100, 200] as const
export const LOG_LIMIT_DEFAULT = 200
const FACET_WIDTH: Record<LogFacetField, number> = {
  level: 14,
  role: 14,
  machine: 18,
  container: 18,
  phase: 16,
  stream: 12,
}

const getStyles = (theme: GrafanaTheme2) => ({
  count: css({
    fontVariantNumeric: 'tabular-nums',
    whiteSpace: 'nowrap',
    display: 'inline-flex',
    alignItems: 'center',
    padding: theme.spacing(0, 0.5),
  }),
  paging: css({
    '& > div:first-of-type > div': {
      borderTopRightRadius: 0,
      borderBottomRightRadius: 0,
    },
  }),
})

export interface LogsToolbarProps {
  search: RunLogsSearch
  onSearchChange: (next: Partial<RunLogsSearch>) => void
  range: TelemetryRange
  facets: Facet[] | undefined
  segments: string[]
  lineCount: number
  truncated: boolean
  isLoading: boolean
  onRefresh: () => void
  live: boolean
  liveAllowed: boolean
  onLiveChange: (live: boolean) => void
  wrap: boolean
  onWrapChange: (wrap: boolean) => void
  canOlder: boolean
  canNewer: boolean
  onOlder: () => void
  onNewer: () => void
  onDownload: () => void
  onOpenRaw: () => void
}

// Logs toolbar on the shared telemetry toolbar: facet filters + search on the left, time range
// and refresh in the middle, paging / live tail / wrap / download / LogsQL on the right, active
// filters as chips below.
export function LogsToolbar({
  search,
  onSearchChange,
  range,
  facets,
  segments,
  lineCount,
  truncated,
  isLoading,
  onRefresh,
  live,
  liveAllowed,
  onLiveChange,
  wrap,
  onWrapChange,
  canOlder,
  canNewer,
  onOlder,
  onNewer,
  onDownload,
  onOpenRaw,
}: LogsToolbarProps) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const [draft, setDraft] = useState(search.q ?? '')
  useEffect(() => setDraft(search.q ?? ''), [search.q])

  const facetOptions = (field: LogFacetField) => {
    const values = facets?.find((f) => f.field === field)?.values ?? []
    const selected = search[field] ?? []
    const opts = values.map((v) => ({
      value: v.value,
      label: v.label ?? v.value,
      description: t('runs.logs.facetCount', { count: v.count }),
    }))
    // A selected value stays visible even when the facet no longer lists it.
    for (const s of selected)
      if (!values.some((v) => v.value === s))
        opts.push({ value: s, label: s, description: t('runs.logs.facetCount', { count: 0 }) })
    return opts
  }

  const filters: ActiveFilter[] = [
    ...LOG_FACET_FIELDS.flatMap((field) =>
      (search[field] ?? []).map((v) => ({
        key: `${field}:${v}`,
        label: `${t(`runs.logs.filters.${field}`)}: ${v}`,
        onRemove: () => {
          const rest = (search[field] ?? []).filter((x) => x !== v)
          onSearchChange({ [field]: rest.length ? rest : undefined } as Partial<RunLogsSearch>)
        },
      }))
    ),
    ...(search.segment
      ? [
          {
            key: 'segment',
            label: `${t('runs.logs.filters.segment')}: ${search.segment}`,
            onRemove: () => onSearchChange({ segment: undefined }),
          },
        ]
      : []),
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
  const resetAll = () =>
    onSearchChange({
      level: undefined,
      role: undefined,
      machine: undefined,
      container: undefined,
      phase: undefined,
      stream: undefined,
      segment: undefined,
      q: undefined,
    })

  const limit = search.limit ?? LOG_LIMIT_DEFAULT
  return (
    <TelemetryToolbar
      range={range}
      isLoading={isLoading}
      onRefresh={onRefresh}
      filters={filters}
      onResetFilters={resetAll}
      actions={
        <>
          <Text color="secondary" variant="bodySmall">
            <span className={styles.count}>
              {t('runs.logs.count', { count: lineCount })}
              {truncated && (
                <Tooltip content={t('runs.logs.truncated')}>
                  <span>+</span>
                </Tooltip>
              )}
            </span>
          </Text>
          <ButtonGroup className={styles.paging}>
            <Select
              width={10}
              aria-label={t('runs.logs.limit')}
              options={LOG_LIMITS.map((n) => ({ label: String(n), value: n }))}
              value={limit}
              onChange={(o) =>
                onSearchChange({ limit: o.value === LOG_LIMIT_DEFAULT ? undefined : o.value })
              }
            />
            <Button
              variant="secondary"
              size="md"
              icon="angle-up"
              disabled={!canOlder}
              onClick={onOlder}
              tooltip={t('runs.logs.olderHint')}
            >
              {t('runs.logs.older')}
            </Button>
            <Button
              variant="secondary"
              size="md"
              icon="angle-down"
              disabled={!canNewer}
              onClick={onNewer}
              tooltip={t('runs.logs.newerHint')}
            >
              {t('runs.logs.newer')}
            </Button>
          </ButtonGroup>
          {!range.terminal && (
            <Tooltip content={liveAllowed ? t('runs.logs.liveHint') : t('runs.logs.liveRange')}>
              <span>
                <InlineSwitch
                  showLabel
                  label={t('runs.logs.liveTail')}
                  value={live && liveAllowed}
                  disabled={!liveAllowed}
                  onChange={(e) => onLiveChange(e.currentTarget.checked)}
                />
              </span>
            </Tooltip>
          )}
          <ToolbarButton
            icon="wrap-text"
            variant={wrap ? 'active' : 'default'}
            tooltip={t('runs.logs.wrap')}
            aria-pressed={wrap}
            onClick={() => onWrapChange(!wrap)}
          />
          <ToolbarButton
            icon="download-alt"
            disabled={!lineCount}
            tooltip={t('runs.logs.downloadHint')}
            onClick={onDownload}
          />
          <ToolbarButton
            icon="brackets-curly"
            onClick={onOpenRaw}
            tooltip={t('runs.logs.raw.title')}
          >
            {t('runs.logs.raw.button')}
          </ToolbarButton>
        </>
      }
    >
      {LOG_FACET_FIELDS.map((field) => {
        const opts = facetOptions(field)
        if (!opts.length) return null
        return (
          <MultiSelect
            key={field}
            aria-label={t(`runs.logs.filters.${field}`)}
            placeholder={t(`runs.logs.filters.${field}`)}
            width={FACET_WIDTH[field]}
            options={opts}
            value={search[field] ?? []}
            closeMenuOnSelect={false}
            onChange={(o) => {
              const v = selectedValues(o)
              onSearchChange({ [field]: v.length ? v : undefined } as Partial<RunLogsSearch>)
            }}
          />
        )
      })}
      {segments.length > 1 && (
        <Select
          aria-label={t('runs.logs.filters.segment')}
          placeholder={t('runs.logs.filters.segment')}
          width={16}
          isClearable
          options={segments.map((s) => ({ label: s, value: s }))}
          value={search.segment ?? null}
          onChange={(o) => onSearchChange({ segment: o?.value ?? undefined })}
        />
      )}
      <form
        onSubmit={(e) => {
          e.preventDefault()
          onSearchChange({ q: draft.trim() || undefined })
        }}
      >
        <Input
          width={30}
          prefix={<Icon name="search" />}
          placeholder={t('runs.logs.search')}
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
}
