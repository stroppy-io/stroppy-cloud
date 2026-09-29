import type { Schemas } from '@api/types'
import { toast } from '@app/Toaster'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, Tooltip, useStyles2 } from '@grafana/ui'
import { durationSeconds, formatDuration } from '@helpers/format'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'

// Cells and row helpers shared by the library lists (tests, databases, workloads). The column
// layout follows web/docs/tables-guide.md §13; these are pure views over the list payloads.

const getStyles = (theme: GrafanaTheme2) => ({
  star: css({ color: theme.colors.warning.text }),
  chips: css({
    display: 'inline-flex',
    gap: theme.spacing(0.5),
    verticalAlign: 'middle',
    marginRight: theme.spacing(0.75),
    maxWidth: '100%',
  }),
  chip: css({
    all: 'unset',
    boxSizing: 'border-box',
    display: 'inline-block',
    padding: theme.spacing(0, 0.75),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.secondary,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    lineHeight: theme.typography.bodySmall.lineHeight,
    whiteSpace: 'nowrap',
    maxWidth: theme.spacing(16),
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  clickable: css({
    cursor: 'pointer',
    '&:hover': { borderColor: theme.colors.border.medium, color: theme.colors.text.primary },
    '&:focus-visible': { outline: `2px solid ${theme.colors.primary.border}` },
  }),
  stale: css({ color: theme.colors.warning.text }),
})

// ☆ in the identity cell: an indicator only, the toggle lives in the row menu (tables-guide §14).
export function FavoriteMark() {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  return (
    <Tooltip content={t('library.list.favorite')}>
      <Icon name="favorite" className={styles.star} aria-label={t('library.list.favorite')} />
    </Tooltip>
  )
}

// «устарел» marker inside a secondary line (the referenced library record changed).
export function StaleMark() {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  return <span className={styles.stale}>{t('common.status.stale')}</span>
}

function tagLabel([k, v]: [string, string]): string {
  return v ? `${k}=${v}` : k
}

// `key=value` chips in front of the identity subtitle. A click filters the list by that tag;
// the full text of every chip (and of the hidden rest behind «+N») is in the native tooltip.
export function TagChips({
  tags,
  max = 2,
  onPick,
}: {
  tags: Record<string, string> | undefined
  max?: number
  onPick?: (tag: string) => void
}) {
  const styles = useStyles2(getStyles)
  const entries = Object.entries(tags ?? {})
  if (!entries.length) return null
  const shown = entries.slice(0, max)
  const rest = entries.slice(max)
  return (
    <span className={styles.chips} data-no-row-click>
      {shown.map((e) => {
        const label = tagLabel(e)
        return onPick ? (
          <button
            key={e[0]}
            type="button"
            className={`${styles.chip} ${styles.clickable}`}
            title={label}
            onClick={(ev) => {
              ev.stopPropagation()
              onPick(label)
            }}
          >
            {label}
          </button>
        ) : (
          <span key={e[0]} className={styles.chip} title={label}>
            {label}
          </span>
        )
      })}
      {rest.length > 0 && (
        <span className={styles.chip} title={rest.map(tagLabel).join(', ')}>
          +{rest.length}
        </span>
      )}
    </span>
  )
}

export function tagsTitle(tags: Record<string, string> | undefined): string {
  return Object.entries(tags ?? {})
    .map(tagLabel)
    .join(', ')
}

// ---- workload segments ----------------------------------------------------------------------

export interface SegmentView {
  name?: string
  script?: string
  duration?: number
  iterations?: number
  vus?: number
}

// The fields lists show out of `workload.segment` values.
export function segmentViews(segments: Schemas['SchemaValue'][] | undefined): SegmentView[] {
  return (segments ?? []).map((raw) => {
    const s = raw as {
      name?: string
      workload?: { script?: string }
      run?: { duration?: string; vus?: number; iterations?: number }
    }
    return {
      name: s.name,
      script: s.workload?.script,
      duration: durationSeconds(s.run?.duration),
      iterations: s.run?.iterations,
      vus: s.run?.vus,
    }
  })
}

// Main script of a workload: the first segment's script plus «+N» other scripts.
export function scriptsLine(segs: SegmentView[]): string | undefined {
  const scripts = [...new Set(segs.map((s) => s.script).filter(Boolean))] as string[]
  const main = scripts[0] ?? segs[0]?.name
  if (!main) return undefined
  return scripts.length > 1 ? `${main} +${scripts.length - 1}` : main
}

export function maxVus(segs: SegmentView[]): number {
  return Math.max(0, ...segs.map((s) => s.vus ?? 0))
}

// Total planned run time over the segments (iteration-bound segments do not add up).
export function totalDuration(segs: SegmentView[]): number | undefined {
  const d = segs.reduce((sum, s) => sum + (s.duration ?? 0), 0)
  return d > 0 ? d : undefined
}

// One line per segment for the native tooltip.
export function segmentsTitle(segs: SegmentView[]): string {
  return segs
    .map((s, i) =>
      [
        `#${i + 1} ${s.name ?? ''}`.trim(),
        s.script,
        s.vus ? `${s.vus} VU` : undefined,
        s.duration !== undefined
          ? formatDuration(s.duration)
          : s.iterations !== undefined
            ? `${s.iterations} it`
            : undefined,
      ]
        .filter(Boolean)
        .join(' · ')
    )
    .join('\n')
}

export function segmentsCount(segs: SegmentView[], t: TFunction): string | undefined {
  return segs.length ? t('library.list.segments', { count: segs.length }) : undefined
}

export function vusText(segs: SegmentView[], t: TFunction): string | undefined {
  const v = maxVus(segs)
  return v ? t('library.list.vus', { count: v }) : undefined
}

// ---- requirements ---------------------------------------------------------------------------

function roleReq(r: Schemas['RoleRequirement']): string {
  return [
    r.cpu !== undefined ? `${r.cpu} CPU` : undefined,
    r.memory_gb ? `${r.memory_gb} GB` : undefined,
    r.disk_gb ? `${r.disk_gb} GB disk` : undefined,
  ]
    .filter(Boolean)
    .join(' · ')
}

// «db: 4 CPU · 16 GB» per role, `; `-separated; `withReason` adds what drove the numbers.
export function requirementsLine(
  req: Schemas['Requirements'] | undefined,
  withReason = false
): string | undefined {
  const parts = Object.entries(req ?? {})
    .map(([role, r]) => {
      const body = roleReq(r)
      if (!body) return undefined
      return `${role}: ${body}${withReason && r.reason ? ` (${r.reason})` : ''}`
    })
    .filter(Boolean)
  return parts.length ? parts.join(withReason ? '\n' : '; ') : undefined
}

// ---- usages ---------------------------------------------------------------------------------

// «в N тестах» (the only records that reference databases/workloads), generic otherwise.
export function usageView(usages: Schemas['Usage'][] | undefined, t: TFunction) {
  const all = usages ?? []
  if (!all.length) return { primary: t('library.list.notUsed'), used: false, title: undefined }
  const tests = all.filter((u) => u.kind === 'test').length
  const others = all.length - tests
  const primary = !tests
    ? t('library.list.usedIn', { count: all.length })
    : others
      ? `${t('library.list.usedInTests', { count: tests })} ${t('library.list.andMore', { count: others })}`
      : t('library.list.usedInTests', { count: tests })
  const title = all.map((u) => `${t(`library.kind.${u.kind}`, u.kind)}: ${u.name}`).join('\n')
  return { primary, used: true, title }
}

// ---- row actions ----------------------------------------------------------------------------

export function copyToClipboard(text: string, t: TFunction) {
  void navigator.clipboard
    .writeText(text)
    .then(() => toast.success(t('common.actions.copied')))
    .catch((e) => toast.error(e))
}
