import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Tooltip, useStyles2 } from '@grafana/ui'

const getStyles = (theme: GrafanaTheme2) => ({
  wrap: css({ display: 'flex', gap: theme.spacing(0.5), flexWrap: 'nowrap', overflow: 'hidden' }),
  tag: css({
    display: 'inline-block',
    padding: theme.spacing(0, 0.75),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.secondary,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    lineHeight: '18px',
    whiteSpace: 'nowrap',
    maxWidth: 140,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    cursor: 'default',
  }),
  clickable: css({
    cursor: 'pointer',
    font: 'inherit',
    '&:hover': { borderColor: theme.colors.border.medium, color: theme.colors.text.primary },
  }),
})

// Compact `key=value` chips; click filters the list by that tag when `onPick` is given.
export function TagsCell({
  tags,
  max = 3,
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
  const label = ([k, v]: [string, string]) => (v ? `${k}=${v}` : k)
  return (
    <div className={styles.wrap}>
      {shown.map((e) => (
        <Tooltip key={e[0]} content={label(e)}>
          {onPick ? (
            <button
              type="button"
              className={`${styles.tag} ${styles.clickable}`}
              onClick={() => onPick(label(e))}
              data-no-row-click
            >
              {label(e)}
            </button>
          ) : (
            <span className={styles.tag}>{label(e)}</span>
          )}
        </Tooltip>
      ))}
      {rest.length > 0 && (
        <Tooltip content={rest.map(label).join(', ')}>
          <span className={styles.tag}>+{rest.length}</span>
        </Tooltip>
      )}
    </div>
  )
}
