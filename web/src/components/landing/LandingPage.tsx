import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import {
  AnimatePresence,
  type AnimationPlaybackControls,
  animate,
  motion,
  useMotionValue,
  useReducedMotion,
} from 'motion/react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { brand } from '@/styles/brand'
import { DeckNav } from './DeckNav'
import { EndPage } from './EndPage'
import { HEADER_HEIGHT, PublicHeader } from './PublicHeader'
import { Headline, PHASES, PLAY, RunStage, SCRUBBED } from './RunStory'

// The deck: the headline, one step per run phase, the closing screen. Nothing
// scrolls natively. A wheel gesture, a swipe or a key moves one step; on the
// scrubbed phases (deploy, load) the wheel first plays the phase forward or
// back, and only a fresh gesture at its end moves on. The headline has two
// states: the first gesture down opens its logo in one go, the next moves on.
const LAST = PHASES.length + 1
// Names of the steps for the side navigation, in deck order.
const STEP_KEYS = ['start', ...PHASES, 'end'] as const
const phaseOf = (step: number) => step - 1
const isScrubbed = (step: number) =>
  step === 0 || (step <= PHASES.length && SCRUBBED.has(phaseOf(step)))
// A wheel gesture ends after this much quiet (trackpad inertia included).
const GESTURE_GAP = 220
// Minimum time between two steps, so one fling never skips a step.
const STEP_LOCK = 600
// Wheel distance that plays a scrubbed phase start to end.
const SCRUB_DISTANCE = 1400
// How long the headline's logo takes to open or close.
const LOGO_TURN = 0.7

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    height: '100vh',
    overflow: 'hidden',
    background: brand.ink,
    color: brand.text,
    fontFamily: theme.typography.fontFamily,
    overscrollBehavior: 'none',
  }),
  stage: css({
    position: 'relative',
    height: `calc(100vh - ${HEADER_HEIGHT}px)`,
    overflow: 'hidden',
  }),
  screen: css({ position: 'absolute', inset: 0 }),
})

type Screen = 'hero' | 'run' | 'end'
const screenOf = (step: number): Screen => (step === 0 ? 'hero' : step === LAST ? 'end' : 'run')

/** The public front page: what Stroppy Cloud does, played as one benchmark run. */
export function LandingPage() {
  const styles = useStyles2(getStyles)
  const reduce = useReducedMotion() ?? false
  const [step, setStep] = useState(0)
  const [dir, setDir] = useState(1)
  const stepRef = useRef(0)
  // Bumped on every entry into provisioning, so its animation replays each time.
  const [entry, setEntry] = useState(0)
  const root = useRef<HTMLDivElement>(null)
  const lockedUntil = useRef(0)
  // Where a scrubbed phase stands, and where the wheel is taking it.
  const scrub = useMotionValue(0)
  const target = useRef(0)
  const playing = useRef<AnimationPlaybackControls | null>(null)

  const scrubTo = useCallback(
    (to: number, duration: number) => {
      playing.current?.stop()
      target.current = to
      if (reduce || duration === 0) {
        scrub.set(to)
        return
      }
      playing.current = animate(scrub, to, { duration, ease: duration > 1 ? 'linear' : 'easeOut' })
    },
    [scrub, reduce]
  )

  // `wheel` enters a scrubbed phase at the edge the reader comes from;
  // a click or a key plays it through.
  const go = useCallback(
    (next: number, how: 'wheel' | 'play') => {
      const from = stepRef.current
      const to = Math.max(0, Math.min(LAST, next))
      if (to === from) return
      stepRef.current = to
      setDir(to > from ? 1 : -1)
      setStep(to)
      if (to === 1) setEntry((n) => n + 1)
      if (!isScrubbed(to)) return
      if (to === 0) {
        // The headline never plays itself; back by wheel it opens unfolded.
        scrubTo(how === 'wheel' ? 1 : 0, 0)
        return
      }
      if (how === 'wheel') {
        scrubTo(to > from ? 0 : 1, 0)
      } else {
        scrub.set(0)
        scrubTo(1, PLAY[phaseOf(to)])
      }
    },
    [scrub, scrubTo]
  )
  const step1 = useCallback(
    (delta: 1 | -1, how: 'wheel' | 'play') => {
      const now = performance.now()
      if (now < lockedUntil.current) return
      lockedUntil.current = now + STEP_LOCK
      go(stepRef.current + delta, how)
    },
    [go]
  )

  useEffect(() => {
    const el = root.current
    if (!el) return
    let last = 0
    let usedByScrub = false
    // The gesture that moved a step ends there; it does not also play the next one.
    let flipped = false
    // Feeds a scrubbed phase; false when it is already at that edge.
    const feed = (dy: number) => {
      if (!isScrubbed(stepRef.current)) return false
      const t = target.current
      if ((dy > 0 && t >= 1) || (dy < 0 && t <= 0)) return false
      if (stepRef.current === 0) {
        // Not scrubbed: the gesture opens or closes the logo whole.
        scrubTo(dy > 0 ? 1 : 0, LOGO_TURN)
        return true
      }
      scrubTo(Math.max(0, Math.min(1, t + dy / SCRUB_DISTANCE)), 0.35)
      return true
    }
    const onWheel = (e: WheelEvent) => {
      if (e.ctrlKey) return // pinch zoom
      e.preventDefault()
      const now = performance.now()
      const fresh = now - last > GESTURE_GAP
      last = now
      if (fresh) {
        usedByScrub = false
        flipped = false
      }
      if (flipped) return
      if (feed(e.deltaY)) {
        usedByScrub = true
        return
      }
      if (!fresh || usedByScrub || Math.abs(e.deltaY) < 4) return
      flipped = true
      step1(e.deltaY > 0 ? 1 : -1, 'wheel')
    }
    let startY: number | null = null
    let lastY = 0
    let touchScrub = false
    const onTouchStart = (e: TouchEvent) => {
      startY = e.touches[0]?.clientY ?? null
      lastY = startY ?? 0
      touchScrub = false
    }
    const onTouchMove = (e: TouchEvent) => {
      const y = e.touches[0]?.clientY
      if (y == null) return
      e.preventDefault()
      if (feed((lastY - y) * 3)) touchScrub = true
      lastY = y
    }
    const onTouchEnd = (e: TouchEvent) => {
      const endY = e.changedTouches[0]?.clientY
      if (startY == null || endY == null || touchScrub) return
      const dy = startY - endY
      if (Math.abs(dy) > 40) step1(dy > 0 ? 1 : -1, 'wheel')
      startY = null
    }
    const onKey = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement | null)?.tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA') return
      const k = e.key
      if (k === 'ArrowDown' || k === 'PageDown' || (k === ' ' && !e.shiftKey)) step1(1, 'play')
      else if (k === 'ArrowUp' || k === 'PageUp' || (k === ' ' && e.shiftKey)) step1(-1, 'play')
      else if (k === 'Home') go(0, 'play')
      else if (k === 'End') go(LAST, 'play')
      else return
      e.preventDefault()
    }
    el.addEventListener('wheel', onWheel, { passive: false })
    el.addEventListener('touchstart', onTouchStart, { passive: true })
    el.addEventListener('touchmove', onTouchMove, { passive: false })
    el.addEventListener('touchend', onTouchEnd, { passive: true })
    window.addEventListener('keydown', onKey)
    return () => {
      el.removeEventListener('wheel', onWheel)
      el.removeEventListener('touchstart', onTouchStart)
      el.removeEventListener('touchmove', onTouchMove)
      el.removeEventListener('touchend', onTouchEnd)
      window.removeEventListener('keydown', onKey)
    }
  }, [go, step1, scrubTo])

  useEffect(() => () => playing.current?.stop(), [])

  const screen = screenOf(step)
  const t = reduce ? { duration: 0 } : { duration: 0.45, ease: [0.22, 1, 0.36, 1] as const }

  return (
    <div ref={root} className={styles.root}>
      <PublicHeader onHome={() => go(0, 'play')} />
      <div className={styles.stage}>
        <AnimatePresence initial={false} custom={dir}>
          <motion.div
            key={screen}
            className={styles.screen}
            custom={dir}
            variants={{
              enter: (d: number) => ({ y: d > 0 ? 80 : -80, opacity: 0 }),
              center: { y: 0, opacity: 1 },
              exit: (d: number) => ({ y: d > 0 ? -80 : 80, opacity: 0 }),
            }}
            initial="enter"
            animate="center"
            exit="exit"
            transition={t}
          >
            {screen === 'hero' && <Headline scrub={scrub} onNext={() => go(1, 'play')} />}
            {screen === 'run' && (
              <RunStage
                entry={entry}
                phase={phaseOf(step)}
                scrub={scrub}
                onPhase={(p) => go(p + 1, 'play')}
              />
            )}
            {screen === 'end' && <EndPage />}
          </motion.div>
        </AnimatePresence>
        <DeckNav keys={STEP_KEYS} current={step} onGo={(to) => go(to, 'play')} />
      </div>
    </div>
  )
}
