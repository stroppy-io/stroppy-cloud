import type { LogLine } from '@api/types'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, useStyles2 } from '@grafana/ui'
import { logLevelTone, logLineFields } from '@helpers/log-lines'
import { formatTime } from '@helpers/time'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'

// Row height when wrapping is off; the virtualizer's estimate when it is on.
export const LOG_ROW_HEIGHT = 22

const getStyles = (theme: GrafanaTheme2) => ({
  row: css({
    position: 'absolute',
    top: 0,
    left: 0,
    width: '100%',
    display: 'flex',
    alignItems: 'flex-start',
    gap: theme.spacing(1),
    padding: theme.spacing(0, 1, 0, 0.5),
    boxSizing: 'border-box',
    lineHeight: `${LOG_ROW_HEIGHT}px`,
    '&:hover': { background: theme.colors.action.hover },
    '&:hover .gutter-index': { visibility: 'hidden' },
    '&:hover .gutter-link': { visibility: 'visible' },
  }),
  rowError: css({ background: theme.colors.error.transparent }),
  rowAnchor: css({
    background: theme.colors.warning.transparent,
    boxShadow: `inset 0 0 0 1px ${theme.colors.warning.border}`,
  }),
  gutter: css({
    flex: '0 0 auto',
    width: '5.5ch',
    position: 'relative',
    textAlign: 'right',
    color: theme.colors.text.disabled,
    fontVariantNumeric: 'tabular-nums',
    userSelect: 'none',
    background: 'none',
    border: 'none',
    padding: 0,
    margin: 0,
    font: 'inherit',
    lineHeight: 'inherit',
    cursor: 'pointer',
    '&:hover': { color: theme.colors.text.link },
    '&:focus-visible': {
      outline: `1px solid ${theme.colors.primary.border}`,
      borderRadius: theme.shape.radius.default,
    },
  }),
  gutterLink: css({
    visibility: 'hidden',
    position: 'absolute',
    inset: 0,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'flex-end',
  }),
  gutterCopied: css({
    color: theme.colors.success.text,
    '&:hover': { color: theme.colors.success.text },
  }),
  time: css({
    flex: '0 0 auto',
    color: theme.colors.text.secondary,
    whiteSpace: 'nowrap',
    fontVariantNumeric: 'tabular-nums',
  }),
  level: css({
    flex: '0 0 auto',
    width: '5ch',
    textTransform: 'uppercase',
    fontWeight: theme.typography.fontWeightMedium,
    overflow: 'hidden',
    whiteSpace: 'nowrap',
  }),
  lvlError: css({ color: theme.colors.error.text }),
  lvlWarning: css({ color: theme.colors.warning.text }),
  lvlInfo: css({ color: theme.colors.info.text }),
  lvlDebug: css({ color: theme.colors.text.disabled }),
  lvlNone: css({ color: theme.colors.text.secondary }),
  machine: css({
    flex: '0 0 auto',
    maxWidth: 160,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    fontWeight: theme.typography.fontWeightMedium,
  }),
  container: css({
    flex: '0 0 auto',
    maxWidth: 180,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    color: theme.colors.text.secondary,
    background: theme.colors.background.secondary,
    borderRadius: theme.shape.radius.default,
    padding: theme.spacing(0, 0.5),
    lineHeight: `${LOG_ROW_HEIGHT - 4}px`,
    marginTop: 2,
  }),
  msg: css({
    flex: '1 1 auto',
    minWidth: 0,
    whiteSpace: 'pre',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  msgWrap: css({ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }),
  msgStderr: css({ color: theme.colors.error.text }),
  mark: css({
    background: theme.colors.warning.transparent,
    color: 'inherit',
    boxShadow: `0 0 0 1px ${theme.colors.warning.border}`,
    borderRadius: theme.shape.radius.default,
  }),
})

function Highlight({
  text,
  q,
  className,
}: {
  text: string
  q: string | undefined
  className: string
}) {
  if (!q) return <>{text}</>
  const idx = text.toLowerCase().indexOf(q.toLowerCase())
  if (idx < 0) return <>{text}</>
  return (
    <>
      {text.slice(0, idx)}
      <mark className={className}>{text.slice(idx, idx + q.length)}</mark>
      {text.slice(idx + q.length)}
    </>
  )
}

export interface LogLineRowProps {
  line: LogLine
  index: number
  start: number
  wrap: boolean
  highlight?: string
  isAnchor: boolean
  isCopied: boolean
  machineColor?: string
  measureRef?: (el: HTMLDivElement | null) => void
  onCopyLink?: (line: LogLine) => void
}

export const LogLineRow = memo(function LogLineRow({
  line,
  index,
  start,
  wrap,
  highlight,
  isAnchor,
  isCopied,
  machineColor,
  measureRef,
  onCopyLink,
}: LogLineRowProps) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const tone = logLevelTone(line.level)
  const lvlClass =
    tone === 'error'
      ? styles.lvlError
      : tone === 'warning'
        ? styles.lvlWarning
        : tone === 'info'
          ? styles.lvlInfo
          : tone === 'debug'
            ? styles.lvlDebug
            : styles.lvlNone
  const title = logLineFields(line)
    .map(([k, v]) => `${k}: ${v}`)
    .join('\n')
  const canLink = line.seq !== undefined && !!onCopyLink
  return (
    <div
      data-index={index}
      data-seq={line.seq}
      ref={measureRef}
      className={cx(styles.row, tone === 'error' && styles.rowError, isAnchor && styles.rowAnchor)}
      style={{ transform: `translateY(${start}px)`, height: wrap ? undefined : LOG_ROW_HEIGHT }}
    >
      <button
        type="button"
        className={cx(styles.gutter, isCopied && styles.gutterCopied)}
        title={canLink ? t('runs.logs.copyLink') : undefined}
        disabled={!canLink}
        onClick={() => onCopyLink?.(line)}
      >
        {isCopied ? (
          <Icon name="check" size="sm" />
        ) : (
          <>
            <span className="gutter-index">{index + 1}</span>
            {canLink && (
              <span className={cx('gutter-link', styles.gutterLink)}>
                <Icon name="link" size="sm" />
              </span>
            )}
          </>
        )}
      </button>
      <span className={styles.time} title={line.time}>
        {formatTime(line.time)}
      </span>
      <span className={cx(styles.level, lvlClass)} title={line.level}>
        {(line.level ?? '').slice(0, 5)}
      </span>
      {line.machine && (
        <span
          className={styles.machine}
          style={machineColor ? { color: machineColor } : undefined}
          title={line.role ? `${line.role} · ${line.machine}` : line.machine}
        >
          [{line.machine}]
        </span>
      )}
      {line.container && line.container !== line.machine && (
        <span className={styles.container} title={line.container}>
          {line.container}
        </span>
      )}
      <span
        className={cx(
          styles.msg,
          wrap && styles.msgWrap,
          line.stream === 'stderr' && styles.msgStderr
        )}
        title={title}
      >
        <Highlight text={line.message} q={highlight} className={styles.mark} />
      </span>
    </div>
  )
})
