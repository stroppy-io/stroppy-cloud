// IANA timezone list for pickers: a curated head + everything the runtime knows.
const COMMON = [
  'UTC',
  'Europe/Moscow',
  'Europe/London',
  'Europe/Berlin',
  'Europe/Paris',
  'Europe/Amsterdam',
  'Europe/Warsaw',
  'Europe/Kiev',
  'Europe/Istanbul',
  'Asia/Yekaterinburg',
  'Asia/Novosibirsk',
  'Asia/Almaty',
  'Asia/Tbilisi',
  'Asia/Yerevan',
  'Asia/Dubai',
  'Asia/Kolkata',
  'Asia/Singapore',
  'Asia/Shanghai',
  'Asia/Tokyo',
  'Australia/Sydney',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'America/Sao_Paulo',
]

let cached: string[] | undefined

export function listTimeZones(): string[] {
  if (cached) return cached
  let all: string[] = []
  try {
    const intl = Intl as unknown as { supportedValuesOf?: (k: string) => string[] }
    all = intl.supportedValuesOf ? intl.supportedValuesOf('timeZone') : []
  } catch {
    all = []
  }
  const set = new Set<string>([...COMMON, ...all])
  cached = [...set]
  return cached
}

export function localTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
  } catch {
    return 'UTC'
  }
}

// "+03:00" for a zone at a given instant.
export function zoneOffsetLabel(tz: string, at: Date = new Date()): string {
  try {
    const part = new Intl.DateTimeFormat('en-US', { timeZone: tz, timeZoneName: 'longOffset' })
      .formatToParts(at)
      .find((p) => p.type === 'timeZoneName')?.value
    return part === 'GMT' ? '+00:00' : (part?.replace('GMT', '') ?? '')
  } catch {
    return ''
  }
}
