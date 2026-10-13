import { AppLink } from '@app/AppLink'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, useStyles2 } from '@grafana/ui'
import { useSignedIn } from '@hooks/useSignedIn'
import {
  AnimatePresence,
  type MotionValue,
  motion,
  useMotionValueEvent,
  useReducedMotion,
  useTransform,
} from 'motion/react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { brand } from '@/styles/brand'
import { HeroMark } from './HeroMark'
import { ReportStrip, RunChrome } from './RunCanvas'

// The run told by its own phases: machines come up (provisioning), the
// database is laid out (deploy), the load runs live (workload), the result
// lands in the rating (results). Provisioning and results play on a clock
// when entered; deploy and load follow `scrub` (0…1), which the deck moves
// with the wheel, so those two run as fast as the reader scrolls.

export const PHASES = ['provision', 'deploy', 'load', 'result'] as const
/** Phases whose picture follows the reader's scroll instead of a clock. */
export const SCRUBBED: ReadonlySet<number> = new Set([1, 2])
/** Seconds a phase plays when entered by click or key. */
export const PLAY = [1.1, 2.2, 3, 1.2] as const

// A TPC-C-ish throughput trace: ramp, plateau with jitter, a checkpoint dip.
const TRACE = [
  2, 6, 14, 26, 38, 47, 52, 55, 54, 57, 56, 58, 55, 57, 59, 58, 46, 41, 52, 57, 58, 56, 59, 58, 57,
  59, 58, 60, 58, 59,
]
const W = 560
const H = 200
const at = (i: number) => i / (TRACE.length - 1)
const tracePath = TRACE.map((v, i) => `${i ? 'L' : 'M'} ${at(i) * W} ${H - (v / 64) * H}`).join(' ')
const areaPath = `${tracePath} L ${W} ${H} L 0 ${H} Z`
// What happens during the load, pinned to the trace.
const EVENTS = [
  { key: 'warmup', x: at(1) },
  { key: 'steady', x: at(9) },
  { key: 'checkpoint', x: at(16) },
  { key: 'vacuum', x: at(23) },
] as const

// How loaded each machine gets relative to the throughput (CPU share), and
// its CPU over the run — the throughput trace, scaled and a little ragged.
const CPU_SHARE = [0.92, 0.5, 0.47, 0.68] as const
const CPU_TRACE = CPU_SHARE.map((share, i) =>
  TRACE.map((v, k) => Math.min(0.97, (v / 60) * share + ((k * 7 + i * 13) % 9) / 100))
)
const SPARK_H = 20
const sparkPath = (series: readonly number[]) =>
  series
    .map((v, k) => `${k ? 'L' : 'M'} ${(k / (series.length - 1)) * 100} ${SPARK_H - v * SPARK_H}`)
    .join(' ')

const MACHINES = [
  { id: 'db-1', role: 'db', icon: 'database', engine: 'PostgreSQL 17', ip: '10.0.1.11' },
  { id: 'db-2', role: 'db-replica', icon: 'database', engine: 'PostgreSQL 17', ip: '10.0.1.12' },
  { id: 'db-3', role: 'db-replica', icon: 'database', engine: 'PostgreSQL 17', ip: '10.0.1.13' },
  { id: 'runner-1', role: 'runner', icon: 'bolt', engine: 'stroppy 6.1', ip: '10.0.1.21' },
] as const

// The deploy log: each line shows once the scrub passes its mark.
// One transient failure is in there on purpose: it is retried, not fatal.
const DEPLOY_LOG = [
  { at: 0.04, who: 'db-1…3', key: 'install', tone: 'ok' },
  { at: 0.13, who: 'db-3', key: 'mirrorTimeout', tone: 'warn' },
  { at: 0.21, who: 'db-3', key: 'retryOk', tone: 'ok' },
  { at: 0.29, who: 'db-1', key: 'leader', tone: 'ok' },
  { at: 0.37, who: 'runner-1', key: 'runner', tone: 'ok' },
] as const
// Then the stand is checked before any load goes in.
const CHECKS_AT = 0.45
const CHECKS = [
  { at: 0.5, key: 'patroni' },
  { at: 0.6, key: 'lag' },
  { at: 0.7, key: 'proxy' },
  { at: 0.8, key: 'spec' },
  { at: 0.9, key: 'selfcheck' },
] as const

// The rating the run lands in. Illustrative rows, one engine family, so the
// picture compares configurations rather than vendors.
const BOARD = [
  { id: 'a', name: 'PostgreSQL 17 · Patroni HA', nodes: 5, qps: 61840, p99: 10.9 },
  { id: 'b', name: 'PostgreSQL 17 · Patroni HA', nodes: 3, qps: 52300, p99: 11.8 },
  { id: 'mine', name: 'PostgreSQL 17 · Patroni HA', nodes: 3, qps: 48210, p99: 12.4 },
  { id: 'c', name: 'PostgreSQL 16 · async replica', nodes: 2, qps: 39870, p99: 15.2 },
  { id: 'd', name: 'PostgreSQL 16 · single', nodes: 1, qps: 33120, p99: 16.7 },
]

const getStyles = (theme: GrafanaTheme2) => ({
  slide: css({
    position: 'absolute',
    inset: 0,
    maxWidth: 1360,
    margin: '0 auto',
    padding: theme.spacing(0, 8),
    [theme.breakpoints.down('md')]: { padding: theme.spacing(0, 2) },
  }),
  // --- headline ---
  intro: css({ display: 'flex', flexDirection: 'column', justifyContent: 'center' }),
  introText: css({ position: 'relative', zIndex: 1 }),
  h1: css({
    margin: 0,
    maxWidth: 940,
    fontSize: 'clamp(40px, 6.4vw, 88px)',
    lineHeight: 1.02,
    fontWeight: 700,
    letterSpacing: '-0.035em',
    color: brand.text,
  }),
  lead: css({
    marginTop: theme.spacing(3),
    maxWidth: 620,
    fontSize: 'clamp(17px, 1.5vw, 20px)',
    lineHeight: 1.55,
    color: brand.muted,
  }),
  actions: css({
    display: 'flex',
    gap: theme.spacing(2),
    marginTop: theme.spacing(5),
    flexWrap: 'wrap',
  }),
  primary: css({
    display: 'inline-flex',
    alignItems: 'center',
    height: 48,
    padding: theme.spacing(0, 3),
    borderRadius: theme.shape.radius.default,
    background: theme.colors.primary.main,
    color: theme.colors.primary.contrastText,
    fontWeight: theme.typography.fontWeightMedium,
    fontSize: theme.typography.h5.fontSize,
    '&:hover': {
      color: theme.colors.primary.contrastText,
      textDecoration: 'none',
      background: theme.colors.primary.shade,
    },
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: 3 },
  }),
  secondary: css({
    display: 'inline-flex',
    alignItems: 'center',
    height: 48,
    padding: theme.spacing(0, 3),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${brand.line}`,
    background: 'none',
    cursor: 'pointer',
    color: brand.text,
    fontSize: theme.typography.h5.fontSize,
    '&:hover': { color: brand.text, textDecoration: 'none', borderColor: brand.muted },
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: 3 },
  }),
  // --- run ---
  // Both columns start at the section heading; the pair is centred as a whole.
  run: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1.3fr)',
    alignItems: 'start',
    alignContent: 'center',
    gap: theme.spacing(8),
    [theme.breakpoints.down('lg')]: {
      gridTemplateColumns: '1fr',
      alignContent: 'center',
      gap: theme.spacing(3),
    },
  }),
  left: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(5) }),
  heading: css({
    margin: 0,
    fontSize: 'clamp(28px, 2.6vw, 38px)',
    lineHeight: 1.1,
    fontWeight: 700,
    letterSpacing: '-0.025em',
    color: brand.text,
  }),
  headingSub: css({
    marginTop: theme.spacing(1.5),
    color: brand.muted,
    maxWidth: 460,
    lineHeight: 1.5,
  }),
  rail: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(3),
    margin: 0,
    padding: 0,
    listStyle: 'none',
  }),
  step: css({
    display: 'grid',
    gridTemplateColumns: '28px minmax(0, 1fr)',
    gap: theme.spacing(2),
    width: '100%',
    padding: 0,
    border: 'none',
    background: 'none',
    textAlign: 'left',
    color: brand.muted,
    cursor: 'pointer',
    transition: 'color 200ms',
    '&:hover': { color: brand.text },
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: 4 },
  }),
  stepActive: css({ color: brand.text, cursor: 'default' }),
  num: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    lineHeight: '28px',
  }),
  stepHead: css({ display: 'flex', alignItems: 'baseline', gap: theme.spacing(1.5) }),
  stepTime: css({
    marginLeft: 'auto',
    color: brand.muted,
    fontSize: theme.typography.bodySmall.fontSize,
    fontFamily: theme.typography.fontFamilyMonospace,
    whiteSpace: 'nowrap',
  }),
  stepTitle: css({
    fontSize: theme.typography.h4.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: '28px',
    letterSpacing: '-0.01em',
  }),
  stepBody: css({ color: brand.muted, maxWidth: 440, lineHeight: 1.55, overflow: 'hidden' }),
  stepBodyText: css({ paddingTop: theme.spacing(0.5) }),
  bar: css({
    position: 'relative',
    height: 2,
    marginTop: theme.spacing(1),
    background: brand.line,
    overflow: 'hidden',
  }),
  barFill: css({ position: 'absolute', inset: 0, background: theme.colors.primary.main }),
  canvas: css({
    position: 'relative',
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(3),
    border: `1px solid ${brand.line}`,
    borderRadius: 10,
    background: brand.panel,
    padding: theme.spacing(3),
    height: 580,
    overflow: 'hidden',
    [theme.breakpoints.down('lg')]: { height: 500, padding: theme.spacing(2) },
  }),
  machines: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(4, minmax(0, 1fr))',
    gap: theme.spacing(1.5),
    flexShrink: 0,
  }),
  slot: css({ position: 'relative', minWidth: 0 }),
  // Where a machine will be: the stand before provisioning.
  placeholder: css({
    position: 'absolute',
    inset: 0,
    border: `1px dashed ${brand.line}`,
    borderRadius: 6,
  }),
  machine: css({
    position: 'relative',
    border: `1px solid ${brand.line}`,
    borderRadius: 6,
    background: brand.raised,
    padding: theme.spacing(1.25, 1.5),
    fontSize: theme.typography.bodySmall.fontSize,
    minWidth: 0,
  }),
  mHead: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.75),
    color: brand.text,
    fontFamily: theme.typography.fontFamilyMonospace,
  }),
  mDot: css({
    marginLeft: 'auto',
    width: 6,
    height: 6,
    borderRadius: '50%',
    background: brand.good,
    flexShrink: 0,
  }),
  mRole: css({ color: brand.muted, marginTop: 2 }),
  mSpec: css({
    color: brand.muted,
    marginTop: theme.spacing(0.75),
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  mEngine: css({ color: brand.text, whiteSpace: 'nowrap' }),
  mBar: css({
    position: 'absolute',
    left: 0,
    right: 0,
    bottom: 0,
    height: 3,
    borderRadius: 2,
    background: brand.line,
    overflow: 'hidden',
  }),
  mBarFill: css({ position: 'absolute', inset: 0, background: brand.good }),
  // Fixed height: a deploy bar, then the CPU sparkline, without a jump.
  // A row: the sparkline fills it, the CPU figure sits at its right end.
  mGraph: css({
    position: 'relative',
    display: 'flex',
    alignItems: 'flex-end',
    gap: theme.spacing(0.75),
    height: SPARK_H,
    marginTop: 6,
  }),
  mSpark: css({ flex: 1, minWidth: 0, height: '100%', overflow: 'visible' }),
  // Two lines reserved, so cards keep one height whatever the text.
  mLink: css({
    marginTop: theme.spacing(0.5),
    minHeight: '2.6em',
    lineHeight: 1.3,
    color: brand.muted,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  mEngineRow: css({
    display: 'flex',
    alignItems: 'baseline',
    gap: theme.spacing(1),
    marginTop: theme.spacing(0.75),
  }),
  mLoad: css({
    lineHeight: 1,
    color: brand.muted,
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  // The lower part of the canvas: a log, the load chart, the rating board.
  lower: css({ position: 'relative', flex: 1, minHeight: 0 }),
  pane: css({ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column' }),
  log: css({
    margin: 0,
    padding: 0,
    listStyle: 'none',
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    color: brand.muted,
  }),
  logWho: css({ display: 'inline-block', minWidth: 80, color: brand.text }),
  logOk: css({ color: brand.good }),
  logWarn: css({ color: theme.colors.warning.text }),
  checksTitle: css({
    marginTop: theme.spacing(1.5),
    color: brand.text,
    fontFamily: theme.typography.fontFamily,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  check: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1) }),
  checkMark: css({ position: 'relative', width: 16, height: 16, flexShrink: 0 }),
  checkPending: css({
    position: 'absolute',
    inset: 3,
    borderRadius: '50%',
    border: `1px solid ${brand.muted}`,
  }),
  checkDone: css({ position: 'absolute', inset: 0, color: brand.good }),
  readout: css({
    display: 'flex',
    gap: theme.spacing(4),
    marginBottom: theme.spacing(1.5),
    fontFamily: theme.typography.fontFamilyMonospace,
  }),
  big: css({ fontSize: 34, lineHeight: 1, color: brand.text, letterSpacing: '-0.02em' }),
  unit: css({ fontSize: theme.typography.bodySmall.fontSize, color: brand.muted, marginLeft: 6 }),
  small: css({ fontSize: theme.typography.h5.fontSize, color: brand.text }),
  smallLabel: css({ fontSize: theme.typography.bodySmall.fontSize, color: brand.muted }),
  plot: css({ position: 'relative', flex: 1, minHeight: 0, marginTop: theme.spacing(3) }),
  plotSvg: css({ position: 'absolute', inset: 0, overflow: 'visible' }),
  event: css({
    position: 'absolute',
    top: 0,
    bottom: 0,
    borderLeft: `1px dashed ${brand.muted}`,
    pointerEvents: 'none',
  }),
  eventLabel: css({
    position: 'absolute',
    bottom: '100%',
    left: 0,
    transform: 'translate(-50%, -4px)',
    whiteSpace: 'nowrap',
    padding: theme.spacing(0.25, 0.75),
    borderRadius: 4,
    background: brand.raised,
    border: `1px solid ${brand.line}`,
    color: brand.text,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  boardHead: css({
    display: 'flex',
    alignItems: 'baseline',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    marginBottom: theme.spacing(1.5),
    flexWrap: 'wrap',
  }),
  boardTitle: css({ color: brand.text, fontWeight: theme.typography.fontWeightMedium }),
  compare: css({
    color: brand.good,
    fontSize: theme.typography.bodySmall.fontSize,
    fontFamily: theme.typography.fontFamilyMonospace,
  }),
  rows: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.75) }),
  row: css({
    display: 'grid',
    gridTemplateColumns: '32px minmax(0, 1fr) auto auto',
    alignItems: 'center',
    gap: theme.spacing(2),
    padding: theme.spacing(1.25, 1.5),
    borderRadius: 6,
    border: `1px solid ${brand.line}`,
    background: brand.raised,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  rowMine: css({ borderColor: theme.colors.primary.main }),
  rank: css({ fontFamily: theme.typography.fontFamilyMonospace, color: brand.muted }),
  rankMine: css({ color: theme.colors.primary.text }),
  rowMain: css({
    minWidth: 0,
    color: brand.text,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  rowSub: css({ color: brand.muted }),
  rowNum: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    color: brand.text,
    textAlign: 'right',
  }),
})

function Counter({ value, format }: { value: MotionValue<number>; format: (n: number) => string }) {
  const [text, setText] = useState(format(value.get()))
  useMotionValueEvent(value, 'change', (v) => setText(format(v)))
  return <>{text}</>
}

/** `value` as a 0…1 ramp over [from, to]. */
function useRamp(value: MotionValue<number>, from: number, to: number) {
  return useTransform(value, [from, to], [0, 1], { clamp: true })
}

/**
 * The opening screen: what the platform does, over the large logo that
 * opens up as the reader scrolls (`scrub`).
 */
export function Headline({
  scrub,
  onRating,
}: {
  scrub: MotionValue<number>
  onRating: () => void
}) {
  const { t } = useTranslation()
  const signedIn = useSignedIn()
  const styles = useStyles2(getStyles)
  const y = useTransform(scrub, [0, 1], [0, -24])
  const opacity = useTransform(scrub, [0, 1], [1, 0.9])
  return (
    <div className={cx(styles.slide, styles.intro)}>
      <HeroMark open={scrub} />
      <motion.div className={styles.introText} style={{ y, opacity }}>
        <h1 className={styles.h1}>{t('landing.hero.title')}</h1>
        <p className={styles.lead}>{t('landing.hero.lead')}</p>
        <div className={styles.actions}>
          <AppLink to={signedIn ? '/' : '/register'} plain className={styles.primary}>
            {signedIn ? t('landing.nav.console') : t('landing.hero.start')}
          </AppLink>
          <button type="button" className={styles.secondary} onClick={onRating}>
            {t('landing.hero.rating')}
          </button>
        </div>
      </motion.div>
    </div>
  )
}

function Machine({
  m,
  i,
  phase,
  scrub,
  draw,
}: {
  m: (typeof MACHINES)[number]
  i: number
  phase: number
  scrub: MotionValue<number>
  draw: MotionValue<number>
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const reduce = useReducedMotion() ?? false
  // Deploy lays the machines out one after another along the first part of
  // the scrub, in step with the log; the rest is the checks.
  const own = useRamp(scrub, 0.04 + i * 0.09, 0.14 + i * 0.09)
  const fill = useTransform(own, (v) => (phase > 1 ? 1 : phase < 1 ? 0 : v))
  // Its link to the rest of the stand shows once the machine is laid out.
  const linked = useTransform(fill, [0.85, 1], [0, 1])
  // Under load it works in step with the throughput.
  const cpu = useTransform(draw, (d) =>
    d < 0.02 ? 0 : CPU_TRACE[i][Math.min(TRACE.length - 1, Math.round(d * (TRACE.length - 1)))]
  )
  const sparkW = useTransform(draw, (d) => d * 100)
  const loading = phase === 2
  const clip = `spark-${m.id}`
  return (
    <div className={styles.slot}>
      <div className={styles.placeholder} />
      <motion.div
        className={styles.machine}
        initial={{ opacity: 0, y: 10, scale: 0.97 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={reduce ? { duration: 0 } : { duration: 0.25, delay: 0.12 + i * 0.16 }}
      >
        <div className={styles.mHead}>
          <Icon name={m.icon} size="sm" />
          {m.id}
          <span className={styles.mDot} />
        </div>
        <div className={styles.mRole}>
          {t('landing.story.vm')} · {t(`landing.story.roles.${m.role}`)}
        </div>
        <div className={styles.mSpec}>4 vCPU · 16 GB</div>
        <div className={styles.mEngineRow}>
          <motion.span className={styles.mEngine} style={{ opacity: fill }}>
            {m.engine}
          </motion.span>
        </div>
        <motion.div className={styles.mLink} style={{ opacity: linked }}>
          {t(`landing.link.${m.role}`, { leader: 'db-1', count: 2 })}
        </motion.div>
        <div className={styles.mGraph}>
          {loading ? (
            <>
              <svg
                className={styles.mSpark}
                viewBox={`0 0 100 ${SPARK_H}`}
                preserveAspectRatio="none"
                aria-hidden="true"
              >
                <defs>
                  <clipPath id={clip}>
                    <motion.rect x="0" y="-2" height={SPARK_H + 4} style={{ width: sparkW }} />
                  </clipPath>
                </defs>
                <path
                  d={sparkPath(CPU_TRACE[i])}
                  fill="none"
                  stroke={brand.signal}
                  strokeWidth={1.5}
                  vectorEffect="non-scaling-stroke"
                  clipPath={`url(#${clip})`}
                />
              </svg>
              <span className={styles.mLoad}>
                <Counter value={cpu} format={(v) => `${Math.round(v * 100)}%`} />
              </span>
            </>
          ) : (
            <div className={styles.mBar}>
              <motion.div className={styles.mBarFill} style={{ scaleX: fill, originX: 0 }} />
            </div>
          )}
        </div>
      </motion.div>
    </div>
  )
}

function LogLine({
  mark,
  scrub,
  who,
  text,
  tone,
}: {
  mark: number
  scrub: MotionValue<number>
  who: string
  text: string
  tone: 'ok' | 'warn'
}) {
  const styles = useStyles2(getStyles)
  const show = useRamp(scrub, mark, mark + 0.05)
  const x = useTransform(show, [0, 1], [-8, 0])
  return (
    <motion.li style={{ opacity: show, x }}>
      <span className={styles.logWho}>{who}</span>
      {tone === 'warn' ? (
        <span className={styles.logWarn}>{text}</span>
      ) : (
        <>
          {text} <span className={styles.logOk}>ok</span>
        </>
      )}
    </motion.li>
  )
}

/** A check before the load: shows up pending, then passes a beat later. */
function CheckLine({
  mark,
  scrub,
  text,
}: {
  mark: number
  scrub: MotionValue<number>
  text: string
}) {
  const styles = useStyles2(getStyles)
  const show = useRamp(scrub, mark - 0.04, mark)
  const passed = useRamp(scrub, mark, mark + 0.04)
  const pending = useTransform(passed, [0, 1], [1, 0])
  return (
    <motion.li className={styles.check} style={{ opacity: show }}>
      <span className={styles.checkMark}>
        <motion.span className={styles.checkPending} style={{ opacity: pending }} />
        <motion.span className={styles.checkDone} style={{ opacity: passed, scale: passed }}>
          <Icon name="check" size="md" />
        </motion.span>
      </span>
      {text}
    </motion.li>
  )
}

function ChecksTitle({ scrub }: { scrub: MotionValue<number> }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const show = useRamp(scrub, CHECKS_AT, CHECKS_AT + 0.04)
  return (
    <motion.li className={styles.checksTitle} style={{ opacity: show }}>
      {t('landing.checks.title')}
    </motion.li>
  )
}

function EventMark({ x, draw, label }: { x: number; draw: MotionValue<number>; label: string }) {
  const styles = useStyles2(getStyles)
  const show = useRamp(draw, x, x + 0.04)
  return (
    <motion.div className={styles.event} style={{ left: `${x * 100}%`, opacity: show }}>
      <span
        className={styles.eventLabel}
        // Labels near the edges hang inward so the canvas does not cut them.
        style={x < 0.15 ? { transform: 'translate(-8px, -4px)' } : undefined}
      >
        {label}
      </span>
    </motion.div>
  )
}

/**
 * The run screen. `phase` is 0–3 (provision … result); `scrub` drives the
 * deploy and load pictures.
 */
export function RunStage({
  entry,
  phase,
  scrub,
  onPhase,
}: {
  /** Changes on every entry into provisioning; replays its animation. */
  entry: number
  phase: number
  scrub: MotionValue<number>
  onPhase: (p: number) => void
}) {
  const { t, i18n } = useTranslation()
  const styles = useStyles2(getStyles)
  const reduce = useReducedMotion() ?? false
  const fmt = new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 0 })
  const fmt1 = new Intl.NumberFormat(i18n.language, {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  })
  const tr = (duration: number, delay = 0) => (reduce ? { duration: 0 } : { duration, delay })

  const draw = useTransform(scrub, (v) => (phase === 2 ? v : phase > 2 ? 1 : 0))
  const clipW = useTransform(draw, (d) => d * W)
  const qps = useTransform(draw, (d) => {
    const i = Math.min(TRACE.length - 1, Math.round(d * (TRACE.length - 1)))
    return d < 0.02 ? 0 : (TRACE[i] / 60) * 48210
  })
  const p99 = useTransform(draw, (d) => (d < 0.04 ? 0 : 12.4 + (1 - d) * 5))
  const rail = useTransform(scrub, (v) => v)

  // Results: the run is placed into the board a beat after the board shows.
  const [placed, setPlaced] = useState(false)
  useEffect(() => {
    if (phase !== 3) {
      setPlaced(false)
      return
    }
    if (reduce) {
      setPlaced(true)
      return
    }
    const id = setTimeout(() => setPlaced(true), 450)
    return () => clearTimeout(id)
  }, [phase, reduce])
  const rows = BOARD.filter((r) => r.id !== 'mine' || placed)

  return (
    <section className={cx(styles.slide, styles.run)} aria-label={t('landing.story.label')}>
      <div className={styles.left}>
        <header>
          <h2 className={styles.heading}>{t('landing.story.heading')}</h2>
          <p className={styles.headingSub}>{t('landing.story.sub')}</p>
        </header>
        <ol className={styles.rail}>
          {PHASES.map((p, i) => (
            <li key={p}>
              <button
                type="button"
                className={cx(styles.step, phase === i && styles.stepActive)}
                aria-current={phase === i ? 'step' : undefined}
                onClick={() => onPhase(i)}
              >
                <span className={styles.num}>{i + 1}</span>
                <div>
                  <div className={styles.stepHead}>
                    <span className={styles.stepTitle}>{t(`landing.story.${p}.title`)}</span>
                    <span className={styles.stepTime}>{t(`landing.story.${p}.time`)}</span>
                  </div>
                  <AnimatePresence initial={false}>
                    {phase === i && (
                      <motion.div
                        className={styles.stepBody}
                        initial={{ height: 0, opacity: 0 }}
                        animate={{ height: 'auto', opacity: 1 }}
                        exit={{ height: 0, opacity: 0 }}
                        transition={tr(0.3)}
                      >
                        <div className={styles.stepBodyText}>{t(`landing.story.${p}.body`)}</div>
                      </motion.div>
                    )}
                  </AnimatePresence>
                  <div className={styles.bar}>
                    {i === phase && SCRUBBED.has(i) ? (
                      <motion.div className={styles.barFill} style={{ scaleX: rail, originX: 0 }} />
                    ) : (
                      <motion.div
                        className={styles.barFill}
                        initial={{ scaleX: i < phase ? 1 : 0 }}
                        animate={{ scaleX: i <= phase ? 1 : 0 }}
                        transition={i === phase ? tr(PLAY[i]) : { duration: 0 }}
                        style={{ originX: 0 }}
                      />
                    )}
                  </div>
                </div>
              </button>
            </li>
          ))}
        </ol>
      </div>

      <div className={styles.canvas} aria-hidden>
        <RunChrome phase={phase} scrub={scrub} />
        {/* No `initial={false}` here: it would also suppress the machines' own entrance. */}
        <AnimatePresence>
          {phase < 3 && (
            <motion.div
              key="machines"
              className={styles.machines}
              exit={{ opacity: 0, height: 0, marginBottom: -24 }}
              transition={tr(0.35)}
            >
              {MACHINES.map((m, i) => (
                <Machine
                  key={`${m.id}-${entry}`}
                  m={m}
                  i={i}
                  phase={phase}
                  scrub={scrub}
                  draw={draw}
                />
              ))}
            </motion.div>
          )}
        </AnimatePresence>

        <div className={styles.lower}>
          <AnimatePresence>
            {phase === 0 && (
              <motion.ul
                key={`provision-${entry}`}
                className={cx(styles.pane, styles.log)}
                exit={{ opacity: 0 }}
                transition={tr(0.2)}
              >
                {['network', ...MACHINES.map((m) => m.id)].map((who, i) => (
                  <motion.li
                    key={who}
                    initial={{ opacity: 0, x: -8 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={tr(0.2, 0.05 + i * 0.16)}
                  >
                    <span className={styles.logWho}>{who}</span>
                    {i
                      ? `${t('landing.log.created')} · ${MACHINES[i - 1].ip}`
                      : t('landing.log.network')}{' '}
                    <span className={styles.logOk}>ok</span>
                  </motion.li>
                ))}
              </motion.ul>
            )}
            {phase === 1 && (
              <motion.ul
                key="deploy"
                className={cx(styles.pane, styles.log)}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={tr(0.2)}
              >
                {DEPLOY_LOG.map((l) => (
                  <LogLine
                    key={l.key}
                    mark={l.at}
                    scrub={scrub}
                    who={l.who}
                    tone={l.tone}
                    text={t(`landing.log.${l.key}`)}
                  />
                ))}
                <ChecksTitle scrub={scrub} />
                {CHECKS.map((c) => (
                  <CheckLine
                    key={c.key}
                    mark={c.at}
                    scrub={scrub}
                    text={t(`landing.checks.${c.key}`, { leader: 'db-1' })}
                  />
                ))}
              </motion.ul>
            )}
            {phase === 2 && (
              <motion.div
                key="chart"
                className={styles.pane}
                initial={{ opacity: 0, y: 12 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -12 }}
                transition={tr(0.3)}
              >
                <div className={styles.readout}>
                  <div>
                    <span className={styles.big}>
                      <Counter value={qps} format={(n) => fmt.format(n)} />
                    </span>
                    <span className={styles.unit}>qps</span>
                  </div>
                  <div>
                    <div className={styles.small}>
                      <Counter value={p99} format={(n) => (n ? `${fmt1.format(n)} ms` : '—')} />
                    </div>
                    <div className={styles.smallLabel}>p99</div>
                  </div>
                  <div>
                    <div className={styles.small}>0</div>
                    <div className={styles.smallLabel}>{t('landing.story.errors')}</div>
                  </div>
                </div>
                <div className={styles.plot}>
                  <svg
                    className={styles.plotSvg}
                    viewBox={`0 0 ${W} ${H}`}
                    width="100%"
                    height="100%"
                    preserveAspectRatio="none"
                    aria-hidden="true"
                  >
                    <defs>
                      <clipPath id="trace-clip">
                        <motion.rect x="0" y="-4" height={H + 8} style={{ width: clipW }} />
                      </clipPath>
                    </defs>
                    {[0.25, 0.5, 0.75, 1].map((f) => (
                      <line
                        key={f}
                        x1="0"
                        x2={W}
                        y1={H * f - 0.5}
                        y2={H * f - 0.5}
                        stroke={brand.line}
                        strokeDasharray={f === 1 ? undefined : '3 5'}
                        vectorEffect="non-scaling-stroke"
                      />
                    ))}
                    <g clipPath="url(#trace-clip)">
                      <path d={areaPath} fill={brand.signalSoft} />
                      <path
                        d={tracePath}
                        fill="none"
                        stroke={brand.signal}
                        strokeWidth="2"
                        vectorEffect="non-scaling-stroke"
                      />
                    </g>
                  </svg>
                  {EVENTS.map((e) => (
                    <EventMark
                      key={e.key}
                      x={e.x}
                      draw={draw}
                      label={t(`landing.events.${e.key}`)}
                    />
                  ))}
                </div>
              </motion.div>
            )}
            {phase === 3 && (
              <motion.div
                key="board"
                className={styles.pane}
                initial={{ opacity: 0, y: 12 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: 12 }}
                transition={tr(0.3, 0.15)}
              >
                <div className={styles.boardHead}>
                  <span className={styles.boardTitle}>{t('landing.board.title')}</span>
                  <motion.span
                    className={styles.compare}
                    initial={{ opacity: 0 }}
                    animate={{ opacity: placed ? 1 : 0 }}
                    transition={tr(0.3, 0.3)}
                  >
                    {t('landing.board.compare', { qps: '+18%', p99: '−2.1 ms' })}
                  </motion.span>
                </div>
                <div className={styles.rows}>
                  <AnimatePresence initial={false}>
                    {rows.map((r, i) => {
                      const mine = r.id === 'mine'
                      return (
                        <motion.div
                          key={r.id}
                          layout={!reduce}
                          className={cx(styles.row, mine && styles.rowMine)}
                          initial={mine ? { opacity: 0, x: 40 } : false}
                          animate={{ opacity: 1, x: 0 }}
                          transition={tr(0.4)}
                        >
                          <span className={cx(styles.rank, mine && styles.rankMine)}>#{i + 1}</span>
                          <div className={styles.rowMain}>
                            {r.name}{' '}
                            <span className={styles.rowSub}>
                              · {t('landing.board.nodes', { count: r.nodes })}
                              {mine && ` · ${t('landing.board.yours')}`}
                            </span>
                          </div>
                          <span className={styles.rowNum}>{fmt.format(r.qps)} qps</span>
                          <span className={styles.rowNum}>{fmt1.format(r.p99)} ms</span>
                        </motion.div>
                      )
                    })}
                  </AnimatePresence>
                </div>
                <ReportStrip show={placed} />
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </div>
    </section>
  )
}
