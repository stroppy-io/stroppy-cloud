import { AppLink } from '@app/AppLink'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import { Badge, Icon, Tooltip, useStyles2 } from '@grafana/ui'
import type { ComponentProps, ReactNode } from 'react'
import { useDensity } from './prefs'

// Cell kit (web/docs/tables-guide.md §2). Every list renders its cells from these so a status,
// a number or a timestamp reads the same in every table. Pages do not hand-roll cell markup.

// Router target of a cell link; kept loose so any typed route fits (`AppLink` checks at runtime).
export interface CellLink {
  to: string
  params?: Record<string, string>
  search?: Record<string, unknown>
}

// `AppLink` is typed per route; a cell link targets whichever route the column passes in.
function CellAppLink({
  link,
  plain,
  className,
  title,
  children,
}: {
  link: CellLink
  plain?: boolean
  className?: string
  title?: string
  children: ReactNode
}) {
  const props = {
    to: link.to,
    params: link.params,
    search: link.search,
    plain,
    className,
    title,
    children,
  } as unknown as ComponentProps<typeof AppLink>
  return <AppLink {...props} />
}

const getStyles = (theme: GrafanaTheme2) => ({
  // Text wraps onto a second line before it is cut (tables-guide §3): long names, reasons and
  // topology labels stay readable; the full value is always in the native tooltip.
  ellipsis: css({
    display: '-webkit-box',
    WebkitBoxOrient: 'vertical',
    WebkitLineClamp: 2,
    minWidth: 0,
    overflow: 'hidden',
    whiteSpace: 'normal',
    overflowWrap: 'anywhere',
    '[data-density="compact"] &': { WebkitLineClamp: 1 },
  }),
  dash: css({ color: theme.colors.text.disabled }),
  identity: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1), minWidth: 0 }),
  identityBody: css({ minWidth: 0, flex: 1 }),
  identityTitle: css({ fontWeight: theme.typography.fontWeightMedium }),
  sub: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    lineHeight: theme.typography.bodySmall.lineHeight,
  }),
  badges: css({ display: 'inline-flex', gap: theme.spacing(0.5), flexShrink: 0 }),
  leadIcon: css({ color: theme.colors.text.secondary, flexShrink: 0 }),
  progress: css({
    height: 3,
    borderRadius: 2,
    background: theme.colors.background.secondary,
    overflow: 'hidden',
    marginTop: 3,
    div: { height: '100%', background: theme.colors.info.main },
  }),
  stack: css({ minWidth: 0 }),
  pair: css({ display: 'flex', alignItems: 'baseline', gap: theme.spacing(0.5), minWidth: 0 }),
  primary: css({ flexShrink: 0, maxWidth: '100%' }),
  secondary: css({ color: theme.colors.text.secondary }),
  num: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    fontVariantNumeric: 'tabular-nums',
    whiteSpace: 'nowrap',
  }),
  numBar: css({
    height: 2,
    marginTop: 3,
    marginLeft: 'auto',
    borderRadius: 1,
    background: theme.colors.primary.main,
    opacity: 0.55,
  }),
  tags: css({ display: 'flex', gap: theme.spacing(0.5), minWidth: 0, overflow: 'hidden' }),
  link: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(0.75), minWidth: 0 }),
  ok: css({ color: theme.colors.success.text }),
})

// Missing value. Never an empty cell, `0` or «n/a» (§2).
export function Dash() {
  const styles = useStyles2(getStyles)
  return <span className={styles.dash}>—</span>
}

function isEmpty(v: ReactNode): boolean {
  return v === undefined || v === null || v === '' || v === false
}

// First column: bold name that links to the record, a secondary line under it (description,
// reason, current phase), optional trailing badges and a progress line for running records.
// The secondary line is dropped in compact density.
export function IdentityCell({
  title,
  link,
  subtitle,
  icon,
  lead,
  badges,
  progress,
}: {
  title: string
  link?: CellLink
  subtitle?: ReactNode
  icon?: IconName
  // Small marker before the name (trigger icon with its tooltip); `icon` is the plain variant.
  lead?: ReactNode
  badges?: ReactNode
  // 0..100; shows a thin progress line under the text.
  progress?: number
}) {
  const styles = useStyles2(getStyles)
  const density = useDensity()
  const name = link ? (
    <CellAppLink
      link={link}
      plain
      className={cx(styles.ellipsis, styles.identityTitle)}
      title={title}
    >
      {title}
    </CellAppLink>
  ) : (
    <span className={cx(styles.ellipsis, styles.identityTitle)} title={title}>
      {title}
    </span>
  )
  const sub =
    density !== 'compact' && !isEmpty(subtitle) ? (
      <span
        className={cx(styles.ellipsis, styles.sub)}
        title={typeof subtitle === 'string' ? subtitle : undefined}
      >
        {subtitle}
      </span>
    ) : null
  return (
    <div className={styles.identity}>
      {icon && <Icon name={icon} className={styles.leadIcon} />}
      {lead && <span className={styles.leadIcon}>{lead}</span>}
      <div className={styles.identityBody}>
        {name}
        {sub}
        {progress !== undefined && (
          <div
            className={styles.progress}
            role="progressbar"
            aria-valuenow={Math.round(progress)}
            aria-valuemin={0}
            aria-valuemax={100}
          >
            <div style={{ width: `${Math.max(0, Math.min(100, progress))}%` }} />
          </div>
        )}
      </div>
      {badges && <span className={styles.badges}>{badges}</span>}
    </div>
  )
}

// One entity described by a main value and a qualifier: «postgres 17», «YC live · M».
// The whole pair ellipsizes together; the full text is in the native tooltip.
export function PairCell({
  primary,
  secondary,
  icon,
}: {
  primary: string | undefined
  secondary?: string
  icon?: IconName
}) {
  const styles = useStyles2(getStyles)
  if (!primary) return <Dash />
  const full = secondary ? `${primary} ${secondary}` : primary
  return (
    <span className={styles.ellipsis} title={full}>
      {icon && <Icon name={icon} size="sm" className={styles.leadIcon} />}
      {primary}
      {secondary && <span className={styles.secondary}> {secondary}</span>}
    </span>
  )
}

// Right-aligned tabular number (the column sets `align: 'right'`). With `max`, a thin bar under
// the value shows it against the largest value on the page, so a column compares at a glance.
export function NumberCell({
  value,
  text,
  max,
}: {
  value: number | undefined | null
  // Pre-formatted text (unit included); defaults to the plain number.
  text?: string
  max?: number
}) {
  const styles = useStyles2(getStyles)
  if (value === undefined || value === null || Number.isNaN(value)) return <Dash />
  const pct = max && max > 0 ? Math.max(0, Math.min(1, value / max)) : undefined
  return (
    <div>
      <span className={styles.num}>{text ?? String(value)}</span>
      {pct !== undefined && (
        <div className={styles.numBar} style={{ width: `${Math.max(pct * 100, 2)}%` }} />
      )}
    </div>
  )
}

// Two lines of one entity: the value that identifies it and a secondary qualifier under it
// («postgres 17» over «patroni · 3 узла»). Compact density keeps only the first line.
export function StackCell({
  primary,
  secondary,
  title,
}: {
  primary: ReactNode
  secondary?: ReactNode
  // Native tooltip with the full text when the lines are truncated.
  title?: string
}) {
  const styles = useStyles2(getStyles)
  const density = useDensity()
  if (isEmpty(primary)) return <Dash />
  return (
    <div className={styles.stack} title={title}>
      <span className={styles.ellipsis}>{primary}</span>
      {density !== 'compact' && !isEmpty(secondary) && (
        <span className={cx(styles.ellipsis, styles.sub)}>{secondary}</span>
      )}
    </div>
  )
}

// Up to `max` neutral badges plus «+N» whose tooltip lists the rest (§2: colour is for signals).
export function TagsCell({ items, max = 2 }: { items: string[]; max?: number }) {
  const styles = useStyles2(getStyles)
  if (!items.length) return <Dash />
  const shown = items.slice(0, max)
  const rest = items.slice(max)
  return (
    <span className={styles.tags} title={items.join(', ')}>
      {shown.map((s) => (
        <Badge key={s} text={s} color="darkgrey" />
      ))}
      {rest.length > 0 && (
        <Tooltip content={rest.join(', ')}>
          <span>
            <Badge text={`+${rest.length}`} color="darkgrey" />
          </span>
        </Tooltip>
      )}
    </span>
  )
}

// Reference to another record (test of a run, database of a test): icon of its kind + name.
export function LinkCell({
  text,
  link,
  icon,
  secondary,
}: {
  text: string | undefined
  link?: CellLink
  icon?: IconName
  secondary?: string
}) {
  const styles = useStyles2(getStyles)
  if (!text) return <Dash />
  const label = (
    <>
      {text}
      {secondary && <span className={styles.secondary}> {secondary}</span>}
    </>
  )
  return (
    <span className={styles.link}>
      {icon && <Icon name={icon} size="sm" className={styles.leadIcon} />}
      {link ? (
        <CellAppLink link={link} className={styles.ellipsis} title={text}>
          {label}
        </CellAppLink>
      ) : (
        <span className={styles.ellipsis} title={text}>
          {label}
        </span>
      )}
    </span>
  )
}

// Read-only yes/no: a check for yes, a dash for no. Editable booleans use a `Switch` instead.
export function BoolCell({ value, label }: { value: boolean | undefined; label?: string }) {
  const styles = useStyles2(getStyles)
  if (!value) return <Dash />
  return <Icon name="check" className={styles.ok} aria-label={label} />
}

// Plain text that ellipsizes with the full value in the native tooltip.
export function TextCell({ value }: { value: string | undefined | null }) {
  const styles = useStyles2(getStyles)
  if (!value) return <Dash />
  return (
    <span className={styles.ellipsis} title={value}>
      {value}
    </span>
  )
}
