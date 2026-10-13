import { useCallback, useEffect, useState } from 'react'

/** A countdown in seconds: `start()` arms it, `left` is 0 when ready again. */
export function useCooldown(seconds: number): { left: number; start: () => void } {
  const [until, setUntil] = useState(0)
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (until <= now) return
    const id = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(id)
  }, [until, now])
  const start = useCallback(() => {
    const t = Date.now()
    setNow(t)
    setUntil(t + seconds * 1000)
  }, [seconds])
  return { left: Math.max(0, Math.ceil((until - now) / 1000)), start }
}
