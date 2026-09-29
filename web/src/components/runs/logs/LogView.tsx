// biome-ignore-all lint/a11y/noNoninteractiveTabindex: the log body is a keyboard-scrollable region (Home/End/PgUp/PgDn)
import type { LogLine } from '@api/types'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { IconButton, Spinner, useStyles2 } from '@grafana/ui'
import { logLineKey } from '@helpers/log-lines'
import { useVirtualizer } from '@tanstack/react-virtual'
import {
  type KeyboardEvent,
  type ReactNode,
  type Ref,
  useCallback,
  useEffect,
  useImperativeHandle,
  useLayoutEffect,
  useRef,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'
import { LOG_ROW_HEIGHT, LogLineRow } from './LogLineRow'

const EDGE_PX = 80
const BOTTOM_PX = 40

const getStyles = (theme: GrafanaTheme2) => ({
  scroll: css({
    overflow: 'auto',
    flex: '1 1 auto',
    minHeight: 0,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.canvas,
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    position: 'relative',
    outline: 'none',
    '&:focus-visible': { boxShadow: `inset 0 0 0 1px ${theme.colors.primary.border}` },
  }),
  inner: css({ position: 'relative', width: '100%' }),
  edge: css({
    position: 'sticky',
    left: 0,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(0.5),
    fontFamily: theme.typography.fontFamily,
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    background: theme.colors.background.secondary,
    zIndex: 1,
  }),
  edgeTop: css({ top: 0, borderBottom: `1px solid ${theme.colors.border.weak}` }),
  edgeBottom: css({ bottom: 0, borderTop: `1px solid ${theme.colors.border.weak}` }),
  empty: css({
    padding: theme.spacing(4),
    textAlign: 'center',
    color: theme.colors.text.secondary,
    fontFamily: theme.typography.fontFamily,
  }),
  jump: css({
    position: 'sticky',
    bottom: theme.spacing(1),
    display: 'flex',
    justifyContent: 'flex-end',
    paddingRight: theme.spacing(2),
    pointerEvents: 'none',
    '& > *': { pointerEvents: 'auto' },
  }),
})

export interface LogViewHandle {
  pageBy: (dir: -1 | 1) => void
  scrollToTop: () => void
  scrollToBottom: () => void
  focus: () => void
}

export interface LogViewProps {
  ref?: Ref<LogViewHandle>
  lines: LogLine[]
  wrap: boolean
  // Auto-scroll to the newest line when lines arrive.
  follow: boolean
  onFollowChange: (follow: boolean) => void
  // Can `follow` be (re)enabled by scrolling to the bottom? (live tail available)
  canFollow?: boolean
  height?: number | string
  emptyText?: ReactNode
  highlight?: string
  // `seq` of the deep-linked line: scrolled into view once, highlighted while set.
  anchorSeq?: string
  machineColors?: Map<string, string>
  onReachTop?: () => void
  onReachBottom?: () => void
  loadingOlder?: boolean
  loadingNewer?: boolean
  onHome?: () => void
  onEnd?: () => void
  onCopyLink?: (line: LogLine) => void
}

// Virtualized log body on @tanstack/react-virtual. Keeps the reader's position when older lines
// are prepended, follows the tail when asked, and handles Home/End/PageUp/PageDown when focused.
export function LogView({
  ref,
  lines,
  wrap,
  follow,
  onFollowChange,
  canFollow = false,
  height = '100%',
  emptyText,
  highlight,
  anchorSeq,
  machineColors,
  onReachTop,
  onReachBottom,
  loadingOlder,
  loadingNewer,
  onHome,
  onEnd,
  onCopyLink,
}: LogViewProps) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const scrollRef = useRef<HTMLDivElement>(null)
  const virtualizer = useVirtualizer({
    count: lines.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => LOG_ROW_HEIGHT,
    overscan: 30,
    getItemKey: (i) => logLineKey(lines[i]),
  })
  // Programmatic scrolls must not be read as "the user scrolled away".
  const suppress = useRef(false)
  const programmatic = useCallback((fn: () => void) => {
    suppress.current = true
    fn()
    window.setTimeout(() => {
      suppress.current = false
    }, 60)
  }, [])
  const [copied, setCopied] = useState<string | null>(null)

  // ── prepend detection: hold the viewport on the same line ──
  const firstKey = useRef<string | undefined>(undefined)
  const snap = useRef({ height: 0, top: 0 })
  useLayoutEffect(() => {
    const el = scrollRef.current
    const newFirst = lines.length ? logLineKey(lines[0]) : undefined
    if (el && firstKey.current && newFirst && newFirst !== firstKey.current) {
      const idx = lines.findIndex((l) => logLineKey(l) === firstKey.current)
      if (idx > 0) {
        programmatic(() => {
          el.scrollTop = snap.current.top + (el.scrollHeight - snap.current.height)
        })
      }
    }
    firstKey.current = newFirst
    if (el) snap.current = { height: el.scrollHeight, top: el.scrollTop }
  })

  // ── follow the tail ──
  useEffect(() => {
    if (!follow || !lines.length) return
    programmatic(() => virtualizer.scrollToIndex(lines.length - 1, { align: 'end' }))
  }, [lines.length, follow, virtualizer, programmatic])

  // ── first fill: a tail buffer opens at its newest line (a deep link centers instead) ──
  const filled = useRef(false)
  useLayoutEffect(() => {
    if (!lines.length) {
      filled.current = false
      return
    }
    if (filled.current) return
    filled.current = true
    if (anchorSeq) return
    programmatic(() => virtualizer.scrollToIndex(lines.length - 1, { align: 'end' }))
  }, [lines.length, anchorSeq, virtualizer, programmatic])

  // wrap toggles row heights → remeasure
  // biome-ignore lint/correctness/useExhaustiveDependencies: `wrap` is the trigger, not a value used inside
  useEffect(() => {
    virtualizer.measure()
  }, [wrap, virtualizer])

  // ── deep link: center the anchored line once it is in the buffer ──
  const anchored = useRef<string | null>(null)
  useEffect(() => {
    if (!anchorSeq || anchored.current === anchorSeq) return
    const idx = lines.findIndex((l) => String(l.seq) === anchorSeq)
    if (idx < 0) return
    anchored.current = anchorSeq
    programmatic(() => virtualizer.scrollToIndex(idx, { align: 'center' }))
  }, [anchorSeq, lines, virtualizer, programmatic])

  const handleScroll = useCallback(() => {
    const el = scrollRef.current
    if (!el) return
    snap.current = { height: el.scrollHeight, top: el.scrollTop }
    if (suppress.current) return
    const fromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
    const atBottom = fromBottom < BOTTOM_PX
    if (follow && !atBottom) onFollowChange(false)
    if (!follow && atBottom && canFollow) onFollowChange(true)
    if (el.scrollTop < EDGE_PX) onReachTop?.()
    if (!follow && fromBottom < EDGE_PX) onReachBottom?.()
  }, [follow, canFollow, onFollowChange, onReachTop, onReachBottom])

  const pageBy = useCallback((dir: -1 | 1) => {
    const el = scrollRef.current
    if (!el) return
    el.scrollTop += dir * Math.max(el.clientHeight - LOG_ROW_HEIGHT, LOG_ROW_HEIGHT)
  }, [])
  const scrollToTop = useCallback(() => {
    const el = scrollRef.current
    if (el) programmatic(() => (el.scrollTop = 0))
  }, [programmatic])
  const scrollToBottom = useCallback(() => {
    if (lines.length)
      programmatic(() => virtualizer.scrollToIndex(lines.length - 1, { align: 'end' }))
  }, [lines.length, virtualizer, programmatic])
  useImperativeHandle(
    ref,
    () => ({ pageBy, scrollToTop, scrollToBottom, focus: () => scrollRef.current?.focus() }),
    [pageBy, scrollToTop, scrollToBottom]
  )

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.target !== e.currentTarget) return
    switch (e.key) {
      case 'Home':
        e.preventDefault()
        onHome ? onHome() : scrollToTop()
        break
      case 'End':
        e.preventDefault()
        onEnd ? onEnd() : scrollToBottom()
        break
      case 'PageUp':
        e.preventDefault()
        pageBy(-1)
        break
      case 'PageDown':
        e.preventDefault()
        pageBy(1)
        break
    }
  }

  const handleCopy = useCallback(
    (l: LogLine) => {
      onCopyLink?.(l)
      const k = logLineKey(l)
      setCopied(k)
      window.setTimeout(() => setCopied((c) => (c === k ? null : c)), 1500)
    },
    [onCopyLink]
  )

  const items = virtualizer.getVirtualItems()
  return (
    <div
      ref={scrollRef}
      className={styles.scroll}
      style={{ height }}
      onScroll={handleScroll}
      onKeyDown={handleKeyDown}
      tabIndex={0}
      role="log"
      aria-live={follow ? 'polite' : 'off'}
    >
      {loadingOlder && (
        <div className={cx(styles.edge, styles.edgeTop)}>
          <Spinner size="sm" inline /> {t('runs.logs.loadingOlder')}
        </div>
      )}
      {lines.length === 0 ? (
        <div className={styles.empty}>{emptyText ?? t('runs.logs.empty')}</div>
      ) : (
        <div className={styles.inner} style={{ height: virtualizer.getTotalSize() }}>
          {items.map((v) => {
            const l = lines[v.index]
            return (
              <LogLineRow
                key={v.key}
                line={l}
                index={v.index}
                start={v.start}
                wrap={wrap}
                highlight={highlight}
                isAnchor={!!anchorSeq && String(l.seq) === anchorSeq}
                isCopied={copied === logLineKey(l)}
                machineColor={l.machine ? machineColors?.get(l.machine) : undefined}
                measureRef={wrap ? virtualizer.measureElement : undefined}
                onCopyLink={onCopyLink ? handleCopy : undefined}
              />
            )
          })}
        </div>
      )}
      {loadingNewer && (
        <div className={cx(styles.edge, styles.edgeBottom)}>
          <Spinner size="sm" inline /> {t('runs.logs.loadingNewer')}
        </div>
      )}
      {!follow && canFollow && lines.length > 0 && (
        <div className={styles.jump}>
          <IconButton
            name="arrow-down"
            size="lg"
            variant="primary"
            tooltip={t('runs.logs.jumpToEnd')}
            onClick={() => {
              onFollowChange(true)
              scrollToBottom()
            }}
          />
        </div>
      )}
    </div>
  )
}
