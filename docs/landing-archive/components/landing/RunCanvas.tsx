import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, useStyles2, useTheme2 } from '@grafana/ui'
import {
  animate,
  type MotionValue,
  motion,
  useMotionValue,
  useMotionValueEvent,
  useReducedMotion,
  useTransform,
} from 'motion/react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { brand } from '@/styles/brand'

// Parts of the run canvas that make it read as the product: the window bar
// (run, status, clock, cloud) and the report strip under the rating.

const PHASE_KEYS = ['provision', 'deploy', 'load', 'result'] as const
// The run's clock, seconds: where each phase starts and how long it lasts.
const CLOCK = [
  { from: 0, span: 72 },
  { from: 72, span: 148 },
  { from: 220, span: 600 },
  { from: 832, span: 0 },
] as const
const mmss = (s: number) =>
  `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(Math.floor(s % 60)).padStart(2, '0')}`

const getStyles = (theme: GrafanaTheme2) => ({
  chrome: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    margin: theme.spacing(-3, -3, 0),
    padding: theme.spacing(1.25, 3),
    borderBottom: `1px solid ${brand.line}`,
    fontSize: theme.typography.bodySmall.fontSize,
    color: brand.muted,
    flexShrink: 0,
    [theme.breakpoints.down('lg')]: {
      margin: theme.spacing(-2, -2, 0),
      padding: theme.spacing(1, 2),
    },
  }),
  status: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.75),
    color: brand.text,
  }),
  dot: css({ width: 8, height: 8, borderRadius: '50%', flexShrink: 0 }),
  runId: css({ fontFamily: theme.typography.fontFamilyMonospace }),
  clock: css({ fontFamily: theme.typography.fontFamilyMonospace, color: brand.text }),
  cloud: css({
    marginLeft: 'auto',
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    padding: theme.spacing(0.25, 1),
    borderRadius: theme.shape.radius.pill,
    border: `1px solid ${brand.line}`,
    whiteSpace: 'nowrap',
  }),
  // An <svg> does not stretch from `inset` alone (it keeps its viewBox ratio).
  topo: css({
    position: 'absolute',
    inset: 0,
    width: '100%',
    height: '100%',
    overflow: 'visible',
    pointerEvents: 'none',
  }),
  report: css({
    marginTop: 'auto',
    display: 'flex',
    gap: theme.spacing(1),
    flexWrap: 'wrap',
    alignItems: 'center',
  }),
  chip: css({
    display: 'flex',
    alignItems: 'baseline',
    gap: theme.spacing(0.75),
    padding: theme.spacing(0.75, 1.25),
    borderRadius: 6,
    border: `1px solid ${brand.line}`,
    background: brand.raised,
    fontSize: theme.typography.bodySmall.fontSize,
    color: brand.muted,
  }),
  chipValue: css({ color: brand.text, fontFamily: theme.typography.fontFamilyMonospace }),
  share: css({
    marginLeft: 'auto',
    alignItems: 'center',
    color: brand.text,
    borderColor: theme.colors.primary.border,
  }),
  link: css({ fontFamily: theme.typography.fontFamilyMonospace, color: theme.colors.primary.text }),
})

/** The window bar over the canvas: run id, status of the phase, clock, cloud. */
export function RunChrome({ phase, scrub }: { phase: number; scrub: MotionValue<number> }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const theme = useTheme2()
  const reduce = useReducedMotion() ?? false
  const phaseRef = useRef(phase)
  phaseRef.current = phase
  // Provisioning has no scrub: its clock runs on its own.
  const prov = useMotionValue(0)
  useEffect(() => {
    if (phase !== 0) return
    if (reduce) {
      prov.set(CLOCK[0].span)
      return
    }
    prov.set(0)
    const ctl = animate(prov, CLOCK[0].span, { duration: 1.1, ease: 'linear' })
    return () => ctl.stop()
  }, [phase, prov, reduce])
  const elapsed = useTransform([scrub, prov], ([s, p]: number[]) => {
    const c = CLOCK[phaseRef.current]
    if (phaseRef.current === 0) return p
    return c.from + c.span * (phaseRef.current === 3 ? 0 : s)
  })
  const [text, setText] = useState(() => mmss(elapsed.get()))
  useMotionValueEvent(elapsed, 'change', (v) => setText(mmss(v)))
  // A phase change alone does not move scrub or the clock: recompute here.
  // biome-ignore lint/correctness/useExhaustiveDependencies: phase is the trigger
  useEffect(() => setText(mmss(elapsed.get())), [phase, elapsed])

  const done = phase === 3
  return (
    <div className={styles.chrome}>
      <span className={styles.status}>
        <motion.span
          className={styles.dot}
          style={{ background: done ? brand.good : theme.colors.primary.main }}
          animate={done || reduce ? { opacity: 1 } : { opacity: [1, 0.35, 1] }}
          transition={done || reduce ? { duration: 0 } : { duration: 1.4, repeat: Infinity }}
        />
        {t(`landing.status.${PHASE_KEYS[phase]}`)}
      </span>
      <span className={styles.runId}>run 2041</span>
      <span className={styles.clock}>{text}</span>
      <span className={styles.cloud}>
        <Icon name="cloud" size="xs" />
        {t('landing.story.cloud')}
      </span>
    </div>
  )
}

/** Under the rating: what the run left behind — its duration, errors, a link. */
export function ReportStrip({ show }: { show: boolean }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const reduce = useReducedMotion() ?? false
  return (
    <motion.div
      className={styles.report}
      initial={{ opacity: 0, y: 8 }}
      animate={show ? { opacity: 1, y: 0 } : { opacity: 0, y: 8 }}
      transition={reduce ? { duration: 0 } : { duration: 0.3, delay: 0.5 }}
    >
      <span className={styles.chip}>
        {t('landing.report.duration')}{' '}
        <span className={styles.chipValue}>{mmss(CLOCK[3].from)}</span>
      </span>
      <span className={styles.chip}>
        {t('landing.report.errors')} <span className={styles.chipValue}>0</span>
      </span>
      <span className={cx(styles.chip, styles.share)}>
        <Icon name="share-alt" size="sm" />
        <span className={styles.link}>stroppy.cloud/s/7f3k9q</span>
      </span>
    </motion.div>
  )
}
