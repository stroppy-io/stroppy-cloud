import { REFRESH_INTERVALS } from '@helpers/time-range'
import { z } from 'zod'

// URL keys shared by the Logs / Metrics / Events tabs: the time window (raw Grafana form) and the
// auto-refresh interval. Tab links in RunDetailLayout carry them across tabs.
export const TELEMETRY_SEARCH_KEYS = ['from', 'to', 'refresh'] as const

export const telemetrySearchSchema = z.object({
  from: z.string().optional().catch(undefined),
  to: z.string().optional().catch(undefined),
  refresh: z.enum(REFRESH_INTERVALS).optional().catch(undefined),
})
export type TelemetrySearch = z.infer<typeof telemetrySearchSchema>

export function pickTelemetrySearch(search: Record<string, unknown>): TelemetrySearch {
  const parsed = telemetrySearchSchema.safeParse(search)
  return parsed.success ? parsed.data : {}
}
