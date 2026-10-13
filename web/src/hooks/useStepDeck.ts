import { type RefObject, useCallback, useEffect, useRef, useState } from 'react'

// A page that moves in steps rather than scrolling: one wheel gesture, swipe
// or key = one step. The landing's deck adds scrubbed phases on top of the
// same rules; pages with plain steps (/how) use this.

// A wheel gesture ends after this much quiet (trackpad inertia included).
const GESTURE_GAP = 220
// Minimum time between two steps, so one fling never skips a step.
const STEP_LOCK = 600

export function useStepDeck(
  root: RefObject<HTMLElement | null>,
  count: number,
  opts: { hashes?: readonly string[] } = {}
) {
  const { hashes } = opts
  const [step, setStep] = useState(() => fromHash(hashes))
  const [dir, setDir] = useState(1)
  const stepRef = useRef(step)
  const lockedUntil = useRef(0)

  const go = useCallback(
    (next: number) => {
      const to = Math.max(0, Math.min(count - 1, next))
      if (to === stepRef.current) return
      setDir(to > stepRef.current ? 1 : -1)
      stepRef.current = to
      setStep(to)
    },
    [count]
  )
  const move = useCallback(
    (delta: 1 | -1) => {
      const now = performance.now()
      if (now < lockedUntil.current) return
      lockedUntil.current = now + STEP_LOCK
      go(stepRef.current + delta)
    },
    [go]
  )

  useEffect(() => {
    const el = root.current
    if (!el) return
    let last = 0
    let flipped = false
    const onWheel = (e: WheelEvent) => {
      if (e.ctrlKey) return
      e.preventDefault()
      const now = performance.now()
      const fresh = now - last > GESTURE_GAP
      last = now
      if (fresh) flipped = false
      if (flipped || Math.abs(e.deltaY) < 4) return
      flipped = true
      move(e.deltaY > 0 ? 1 : -1)
    }
    let startY: number | null = null
    const onTouchStart = (e: TouchEvent) => {
      startY = e.touches[0]?.clientY ?? null
    }
    const onTouchEnd = (e: TouchEvent) => {
      const endY = e.changedTouches[0]?.clientY
      if (startY == null || endY == null) return
      const dy = startY - endY
      if (Math.abs(dy) > 40) move(dy > 0 ? 1 : -1)
      startY = null
    }
    const onKey = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement | null)?.tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA') return
      const k = e.key
      if (k === 'ArrowDown' || k === 'PageDown' || (k === ' ' && !e.shiftKey)) move(1)
      else if (k === 'ArrowUp' || k === 'PageUp' || (k === ' ' && e.shiftKey)) move(-1)
      else if (k === 'Home') go(0)
      else if (k === 'End') go(count - 1)
      else return
      e.preventDefault()
    }
    el.addEventListener('wheel', onWheel, { passive: false })
    el.addEventListener('touchstart', onTouchStart, { passive: true })
    el.addEventListener('touchend', onTouchEnd, { passive: true })
    window.addEventListener('keydown', onKey)
    return () => {
      el.removeEventListener('wheel', onWheel)
      el.removeEventListener('touchstart', onTouchStart)
      el.removeEventListener('touchend', onTouchEnd)
      window.removeEventListener('keydown', onKey)
    }
  }, [root, move, go, count])

  // The address follows the deck, replaced in place; a typed anchor moves it.
  useEffect(() => {
    if (!hashes) return
    const hash = hashes[step] ?? ''
    if (window.location.hash.replace(/^#/, '') !== hash) {
      const url = hash ? `#${hash}` : window.location.pathname + window.location.search
      window.history.replaceState(window.history.state, '', url)
    }
  }, [step, hashes])
  useEffect(() => {
    if (!hashes) return
    const onHash = () => go(fromHash(hashes))
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [go, hashes])

  return { step, dir, go }
}

function fromHash(hashes?: readonly string[]) {
  const i = hashes?.indexOf(window.location.hash.replace(/^#/, '')) ?? -1
  return i < 0 ? 0 : i
}
