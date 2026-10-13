import { BAND, BODY, RAYS } from '@components/Logo'
import { css, cx, keyframes } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2, useTheme2 } from '@grafana/ui'
import { type MotionValue, motion, useTransform } from 'motion/react'
import type { ReactNode } from 'react'
import { brand } from '@/styles/brand'

// The logo, large and muted, behind the headline.
//
// At rest the rays fire one after another, left to right, like a signal; the
// band breathes. On the wheel, `open` (0…1) lifts the cylinder's layers apart
// and throws the rays out, brighter, as if the database were being opened up
// for the test.
//
// Every layer and every ray is its own <svg> over the same viewBox, so all
// motion is opacity and transform on separate elements — composited, never a
// repaint of one large drawing per frame.

// The drawing area; layers translate in % of it, so units convert exactly.
const VB = { x: 14, y: -2, w: 100, h: 132 }
const ux = (u: number) => `${(u / VB.w) * 100}%`
const uy = (u: number) => `${(u / VB.h) * 100}%`

// Rays leave from the top of the cylinder.
const ORIGIN = { x: 64, y: 42 }
const RAY_CENTRES = [
  [35, 27],
  [47, 22],
  [64, 22],
  [80, 23],
  [92, 27],
] as const
const RAY_DIRS = RAY_CENTRES.map(([x, y]) => {
  const dx = x - ORIGIN.x
  const dy = y - ORIGIN.y
  const len = Math.hypot(dx, dy)
  return [dx / len, dy / len] as const
})
// Open state: how far each layer travels (drawing units), how far rays fly.
const LIFT = { top: -16, middle: -6, band: 3, bottom: 12 }
const THROW = 14
// Idle: one wave across the rays per period; each ray nudges this far out.
const WAVE_PERIOD = 2.8
const WAVE_STEP = 0.18
const NUDGE = 2.5
// Each ray's pulse: brighten and nudge out along its own direction.
const PULSES = RAY_DIRS.map(([dx, dy]) =>
  keyframes({
    '0%, 40%, 100%': { opacity: 0.4, transform: 'translate(0, 0)' },
    '14%': { opacity: 1, transform: `translate(${ux(dx * NUDGE)}, ${uy(dy * NUDGE)})` },
  })
)
const breathe = keyframes({
  '0%, 100%': { opacity: 0.75 },
  '50%': { opacity: 1 },
})

const getStyles = (theme: GrafanaTheme2) => ({
  wrap: css({
    position: 'absolute',
    // Centred by margins: `transform` belongs to the scroll (scale, rotate).
    top: 0,
    bottom: 0,
    margin: 'auto 0',
    right: '3vw',
    height: '82%',
    aspectRatio: `${VB.w} / ${VB.h}`,
    pointerEvents: 'none',
    zIndex: 0,
    [theme.breakpoints.down('lg')]: { right: '-12vw', opacity: 0.6 },
  }),
  layer: css({ position: 'absolute', inset: 0, willChange: 'transform, opacity' }),
  svg: css({ position: 'absolute', inset: 0, width: '100%', height: '100%', overflow: 'visible' }),
  reduced: css({ '@media (prefers-reduced-motion: reduce)': { animation: 'none !important' } }),
})

function Layer({ children, style }: { children: ReactNode; style?: object }) {
  const styles = useStyles2(getStyles)
  return (
    <motion.div className={styles.layer} style={style}>
      <svg
        className={styles.svg}
        viewBox={`${VB.x} ${VB.y} ${VB.w} ${VB.h}`}
        preserveAspectRatio="xMidYMid meet"
        aria-hidden="true"
      >
        {children}
      </svg>
    </motion.div>
  )
}

function Ray({
  d,
  i,
  open,
  fill,
}: {
  d: string
  i: number
  open: MotionValue<number>
  fill: string
}) {
  const styles = useStyles2(getStyles)
  const [dx, dy] = RAY_DIRS[i]
  const x = useTransform(open, (v) => ux(dx * THROW * v))
  const y = useTransform(open, (v) => uy(dy * THROW * v))
  // Scaled by the pulse (0.4–1): idle 0.22–0.55, open up to 0.8.
  const opacity = useTransform(open, [0, 1], [0.55, 0.8])
  return (
    <motion.div className={styles.layer} style={{ x, y, opacity }}>
      <div
        className={`${styles.layer} ${styles.reduced}`}
        style={{
          animation: `${PULSES[i]} ${WAVE_PERIOD}s ease-in-out ${i * WAVE_STEP}s infinite both`,
        }}
      >
        <svg
          className={styles.svg}
          viewBox={`${VB.x} ${VB.y} ${VB.w} ${VB.h}`}
          preserveAspectRatio="xMidYMid meet"
          aria-hidden="true"
        >
          <path d={d} fill={fill} />
        </svg>
      </div>
    </motion.div>
  )
}

/** `className` places and sizes the mark (default: the landing's right half). */
export function HeroMark({ open, className }: { open: MotionValue<number>; className?: string }) {
  const styles = useStyles2(getStyles)
  const theme = useTheme2()
  const accent = theme.colors.primary.main
  const top = useTransform(open, (v) => uy(LIFT.top * v))
  const middle = useTransform(open, (v) => uy(LIFT.middle * v))
  const band = useTransform(open, (v) => uy(LIFT.band * v))
  const bottom = useTransform(open, (v) => uy(LIFT.bottom * v))
  const body = useTransform(open, [0, 1], [0.05, 0.09])
  const bandOpacity = useTransform(open, [0, 1], [0.16, 0.3])
  const scale = useTransform(open, [0, 1], [1, 1.05])
  const rotate = useTransform(open, [0, 1], [0, -3])
  return (
    <motion.div className={cx(styles.wrap, className)} style={{ scale, rotate }} aria-hidden>
      <motion.div className={styles.layer} style={{ opacity: body }}>
        <Layer style={{ y: top }}>
          <path d={BODY[0]} fill={brand.text} />
        </Layer>
        <Layer style={{ y: middle }}>
          <path d={BODY[1]} fill={brand.text} />
        </Layer>
        <Layer style={{ y: bottom }}>
          <path d={BODY[2]} fill={brand.text} />
        </Layer>
      </motion.div>
      <div
        className={`${styles.layer} ${styles.reduced}`}
        style={{ animation: `${breathe} 6s ease-in-out infinite` }}
      >
        <Layer style={{ y: band, opacity: bandOpacity }}>
          <path d={BAND} fill={accent} />
        </Layer>
      </div>
      {RAYS.map((d, i) => (
        <Ray key={d} d={d} i={i} open={open} fill={accent} />
      ))}
    </motion.div>
  )
}
