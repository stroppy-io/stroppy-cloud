import { useState } from 'react'
import { REFRESH_MS, type RefreshInterval } from './list-search'

// Interval for react-query `refetchInterval`, paused while a table overlay (filter popover,
// row menu) is open so a refetch never yanks the UI from under the user.
export function useAutoRefresh(interval: RefreshInterval) {
  const [paused, setPaused] = useState(false)
  const ms = REFRESH_MS[interval]
  return {
    refetchInterval: paused || ms === 0 ? (false as const) : ms,
    paused,
    setPaused,
  }
}
