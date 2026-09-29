import type { Run } from '@api/types'
import { Badge, Tooltip } from '@grafana/ui'
import { formatDuration } from '@helpers/format'
import { formatDateTime } from '@helpers/time'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

// "Stand kept · 1h 12m left" with a self-refreshing countdown.
export function KeepBadge({ run }: { run: Run }) {
  const { t } = useTranslation()
  const [, tick] = useState(0)
  useEffect(() => {
    if (!run.keep_until) return
    const id = window.setInterval(() => tick((n) => n + 1), 30_000)
    return () => window.clearInterval(id)
  }, [run.keep_until])
  if (!run.stand_kept) return null
  const left = run.keep_until ? (new Date(run.keep_until).getTime() - Date.now()) / 1000 : undefined
  const text =
    left === undefined
      ? t('runs.keep.kept')
      : left <= 0
        ? t('runs.keep.expiring')
        : t('runs.keep.left', { time: formatDuration(left) })
  return (
    <Tooltip
      content={
        run.keep_until
          ? t('runs.keptUntil', { time: formatDateTime(run.keep_until) })
          : t('runs.keep.kept')
      }
    >
      <span>
        <Badge
          text={text}
          color={left !== undefined && left < 900 ? 'orange' : 'purple'}
          icon="lock"
        />
      </span>
    </Tooltip>
  )
}
