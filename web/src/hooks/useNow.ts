import { useEffect, useState } from 'react'

// useNow is the current time, re-read every `ms` while enabled: progress and countdowns computed
// from server anchors move on their own between requests.
export function useNow(ms: number, enabled: boolean): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!enabled) return
    setNow(Date.now())
    const id = window.setInterval(() => setNow(Date.now()), ms)
    return () => window.clearInterval(id)
  }, [ms, enabled])
  return now
}
