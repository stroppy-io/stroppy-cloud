// Small 5-field cron parser: `minute hour day-of-month month day-of-week`.
// Supports `*`, `*/n`, `a-b`, `a-b/n`, lists, month/weekday names and the usual `@daily` macros.
// Fire times are computed in an IANA timezone via Intl (DST approximated by re-resolving offsets).

export interface CronSpec {
  minute: number[]
  hour: number[]
  dom: number[]
  month: number[]
  dow: number[]
  domAny: boolean
  dowAny: boolean
}

export type CronParse = { ok: true; spec: CronSpec } | { ok: false; error: CronError }
export type CronError =
  | 'fields'
  | 'field:minute'
  | 'field:hour'
  | 'field:dom'
  | 'field:month'
  | 'field:dow'

const MACROS: Record<string, string> = {
  '@yearly': '0 0 1 1 *',
  '@annually': '0 0 1 1 *',
  '@monthly': '0 0 1 * *',
  '@weekly': '0 0 * * 0',
  '@daily': '0 0 * * *',
  '@midnight': '0 0 * * *',
  '@hourly': '0 * * * *',
}
const MONTHS = ['jan', 'feb', 'mar', 'apr', 'may', 'jun', 'jul', 'aug', 'sep', 'oct', 'nov', 'dec']
const DAYS = ['sun', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat']

function parseField(
  raw: string,
  min: number,
  max: number,
  names?: string[]
): { values: number[]; any: boolean } | undefined {
  const out = new Set<number>()
  let any = false
  const atom = (s: string): number | undefined => {
    const lower = s.toLowerCase()
    if (names) {
      const i = names.indexOf(lower.slice(0, 3))
      if (i >= 0 && lower.length <= 3) return i + (names === MONTHS ? 1 : 0)
    }
    if (!/^\d+$/.test(s)) return undefined
    const n = Number(s)
    // weekday 7 == sunday
    if (names === DAYS && n === 7) return 0
    return n >= min && n <= max ? n : undefined
  }
  for (const part of raw.split(',')) {
    if (!part) return undefined
    const [rangeRaw, stepRaw] = part.split('/')
    if (stepRaw !== undefined && (!/^\d+$/.test(stepRaw) || Number(stepRaw) < 1)) return undefined
    const step = stepRaw ? Number(stepRaw) : 1
    let lo: number
    let hi: number
    if (rangeRaw === '*') {
      lo = min
      hi = max
      if (!stepRaw) any = true
    } else if (rangeRaw.includes('-')) {
      const [a, b] = rangeRaw.split('-')
      const la = atom(a)
      const lb = atom(b)
      if (la === undefined || lb === undefined || la > lb) return undefined
      lo = la
      hi = lb
    } else {
      const v = atom(rangeRaw)
      if (v === undefined) return undefined
      lo = v
      hi = stepRaw ? max : v
    }
    for (let v = lo; v <= hi; v += step) out.add(v)
  }
  return { values: [...out].sort((a, b) => a - b), any }
}

export function parseCron(input: string): CronParse {
  const expr = (MACROS[input.trim().toLowerCase()] ?? input).trim().replace(/\s+/g, ' ')
  const fields = expr.split(' ')
  if (fields.length !== 5) return { ok: false, error: 'fields' }
  const minute = parseField(fields[0], 0, 59)
  if (!minute) return { ok: false, error: 'field:minute' }
  const hour = parseField(fields[1], 0, 23)
  if (!hour) return { ok: false, error: 'field:hour' }
  const dom = parseField(fields[2], 1, 31)
  if (!dom) return { ok: false, error: 'field:dom' }
  const month = parseField(fields[3], 1, 12, MONTHS)
  if (!month) return { ok: false, error: 'field:month' }
  const dow = parseField(fields[4], 0, 7, DAYS)
  if (!dow) return { ok: false, error: 'field:dow' }
  return {
    ok: true,
    spec: {
      minute: minute.values,
      hour: hour.values,
      dom: dom.values,
      month: month.values,
      dow: dow.values,
      domAny: dom.any,
      dowAny: dow.any,
    },
  }
}

export function isValidCron(expr: string): boolean {
  return parseCron(expr).ok
}

export function isValidTimeZone(tz: string): boolean {
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: tz })
    return true
  } catch {
    return false
  }
}

// ---- timezone math ----
const fmtCache = new Map<string, Intl.DateTimeFormat>()
function formatter(tz: string): Intl.DateTimeFormat {
  let f = fmtCache.get(tz)
  if (!f) {
    f = new Intl.DateTimeFormat('en-US', {
      timeZone: tz,
      hourCycle: 'h23',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
    fmtCache.set(tz, f)
  }
  return f
}

interface Wall {
  y: number
  mo: number // 1..12
  d: number
  h: number
  mi: number
}

function wallOf(date: Date, tz: string): Wall {
  const parts = formatter(tz).formatToParts(date)
  const get = (type: string) => Number(parts.find((p) => p.type === type)?.value ?? 0)
  return { y: get('year'), mo: get('month'), d: get('day'), h: get('hour') % 24, mi: get('minute') }
}

function wallAsUtcMs(w: Wall): number {
  return Date.UTC(w.y, w.mo - 1, w.d, w.h, w.mi)
}

function wallFromUtcMs(ms: number): Wall {
  const d = new Date(ms)
  return {
    y: d.getUTCFullYear(),
    mo: d.getUTCMonth() + 1,
    d: d.getUTCDate(),
    h: d.getUTCHours(),
    mi: d.getUTCMinutes(),
  }
}

function offsetMs(instant: number, tz: string): number {
  return wallAsUtcMs(wallOf(new Date(instant), tz)) - instant
}

// Wall-clock in `tz` → instant. Two passes handle DST transitions well enough.
function zonedToUtc(w: Wall, tz: string): Date {
  const guess = wallAsUtcMs(w)
  const off1 = offsetMs(guess, tz)
  let result = guess - off1
  const off2 = offsetMs(result, tz)
  if (off2 !== off1) result = guess - off2
  return new Date(result)
}

function dayMatches(spec: CronSpec, w: Wall): boolean {
  const dowOfDate = new Date(Date.UTC(w.y, w.mo - 1, w.d)).getUTCDay()
  const domOk = spec.dom.includes(w.d)
  const dowOk = spec.dow.includes(dowOfDate)
  if (spec.domAny && spec.dowAny) return true
  if (spec.domAny) return dowOk
  if (spec.dowAny) return domOk
  return domOk || dowOk // vixie cron: both restricted → OR
}

// Next `count` fire times strictly after `from`, in timezone `tz`.
export function nextFireTimes(
  expr: string,
  tz: string,
  count = 5,
  from: Date = new Date()
): Date[] {
  const parsed = parseCron(expr)
  if (!parsed.ok || !isValidTimeZone(tz) || count <= 0) return []
  const { spec } = parsed
  const out: Date[] = []
  // start at the next whole minute in the target tz
  let w = wallOf(new Date(Math.floor(from.getTime() / 60_000) * 60_000 + 60_000), tz)
  let guard = 0
  const MAX_ITER = 200_000
  while (out.length < count && guard++ < MAX_ITER) {
    if (!spec.month.includes(w.mo)) {
      w = { y: w.mo === 12 ? w.y + 1 : w.y, mo: w.mo === 12 ? 1 : w.mo + 1, d: 1, h: 0, mi: 0 }
      continue
    }
    if (!dayMatches(spec, w)) {
      w = wallFromUtcMs(Date.UTC(w.y, w.mo - 1, w.d + 1))
      continue
    }
    if (!spec.hour.includes(w.h)) {
      w = wallFromUtcMs(Date.UTC(w.y, w.mo - 1, w.d, w.h + 1))
      continue
    }
    if (!spec.minute.includes(w.mi)) {
      w = wallFromUtcMs(Date.UTC(w.y, w.mo - 1, w.d, w.h, w.mi + 1))
      continue
    }
    out.push(zonedToUtc(w, tz))
    w = wallFromUtcMs(Date.UTC(w.y, w.mo - 1, w.d, w.h, w.mi + 1))
  }
  return out
}

export function nextFireTime(expr: string, tz: string, from?: Date): Date | undefined {
  return nextFireTimes(expr, tz, 1, from)[0]
}

// ---- human description ----
export type Translate = (key: string, opts?: Record<string, unknown>) => string

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

function listJoin(items: string[], lang: string): string {
  try {
    return new Intl.ListFormat(lang, { style: 'long', type: 'conjunction' }).format(items)
  } catch {
    return items.join(', ')
  }
}

function weekdayName(i: number, lang: string): string {
  // 2023-01-01 is a Sunday
  return new Intl.DateTimeFormat(lang, { weekday: 'long', timeZone: 'UTC' }).format(
    new Date(Date.UTC(2023, 0, 1 + i))
  )
}

function monthName(m: number, lang: string): string {
  return new Intl.DateTimeFormat(lang, { month: 'long', timeZone: 'UTC' }).format(
    new Date(Date.UTC(2023, m - 1, 1))
  )
}

// Detect `*/n`-style periodic fields (0, n, 2n, …).
function periodOf(values: number[], min: number, max: number): number | undefined {
  if (values.length < 2 || values[0] !== min) return undefined
  const step = values[1] - values[0]
  if (step <= 0) return undefined
  for (let i = 1; i < values.length; i++) if (values[i] - values[i - 1] !== step) return undefined
  return values[values.length - 1] + step > max ? step : undefined
}

// Human text for a cron expression. Keys live under `schedules.cron.*` in the locales.
export function describeCron(expr: string, t: Translate, lang: string): string {
  const parsed = parseCron(expr)
  if (!parsed.ok) return t('schedules.cron.invalid')
  const s = parsed.spec
  const everyMinute = s.minute.length === 60
  const everyHour = s.hour.length === 24
  const everyMonth = s.month.length === 12

  // time part
  let time: string
  const minutePeriod = periodOf(s.minute, 0, 59)
  const hourPeriod = periodOf(s.hour, 0, 23)
  if (everyMinute && everyHour) time = t('schedules.cron.everyMinute')
  else if (everyHour && minutePeriod) time = t('schedules.cron.everyNMinutes', { n: minutePeriod })
  else if (everyHour && s.minute.length === 1)
    time = t('schedules.cron.everyHourAt', { minute: pad(s.minute[0]) })
  else if (hourPeriod && s.minute.length === 1)
    time = t('schedules.cron.everyNHours', { n: hourPeriod, minute: pad(s.minute[0]) })
  else if (s.hour.length * s.minute.length <= 6) {
    const times = s.hour.flatMap((h) => s.minute.map((m) => `${pad(h)}:${pad(m)}`))
    time = t('schedules.cron.at', { times: listJoin(times, lang) })
  } else {
    time = t('schedules.cron.atMinutesHours', {
      minutes: listJoin(s.minute.map(String), lang),
      hours: listJoin(s.hour.map(String), lang),
    })
  }

  // day part
  const dayBits: string[] = []
  if (!s.dowAny) {
    const days = s.dow.map((d) => weekdayName(d, lang))
    const weekdays = [1, 2, 3, 4, 5]
    const isWeekdays = s.dow.length === 5 && weekdays.every((d) => s.dow.includes(d))
    dayBits.push(
      isWeekdays
        ? t('schedules.cron.onWeekdays')
        : t('schedules.cron.onDays', { days: listJoin(days, lang) })
    )
  }
  if (!s.domAny)
    dayBits.push(t('schedules.cron.onDom', { days: listJoin(s.dom.map(String), lang) }))
  if (!everyMonth)
    dayBits.push(
      t('schedules.cron.inMonths', {
        months: listJoin(
          s.month.map((m) => monthName(m, lang)),
          lang
        ),
      })
    )
  const dayPart =
    dayBits.length === 0
      ? everyMinute && everyHour
        ? ''
        : t('schedules.cron.everyDay')
      : dayBits.join(t('schedules.cron.or'))

  return dayPart ? `${time}, ${dayPart}` : time
}

// Formats an instant as wall clock in a zone: "Tue, 29 Sep 2026, 02:00".
export function formatInZone(date: Date, tz: string, lang: string): string {
  try {
    return new Intl.DateTimeFormat(lang, {
      timeZone: tz,
      weekday: 'short',
      day: 'numeric',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      hourCycle: 'h23',
    }).format(date)
  } catch {
    return date.toISOString()
  }
}
