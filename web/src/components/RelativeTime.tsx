import { Tooltip } from '@grafana/ui'
import { formatDateTime, relativeTime } from '@helpers/time'
import { useEffect, useState } from 'react'

// "5 minutes ago" that refreshes itself; tooltip shows the absolute timestamp.
export function RelativeTime({
  value,
  refresh = 30_000,
}: {
  value: string | null | undefined
  refresh?: number
}) {
  const [, tick] = useState(0)
  useEffect(() => {
    if (!value) return
    const id = window.setInterval(() => tick((n) => n + 1), refresh)
    return () => window.clearInterval(id)
  }, [value, refresh])
  if (!value) return <span>—</span>
  return (
    <Tooltip content={formatDateTime(value)}>
      <time dateTime={value}>{relativeTime(value)}</time>
    </Tooltip>
  )
}
