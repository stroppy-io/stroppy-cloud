import type { RunEvent } from '@api/types'
import type { TFunction } from 'i18next'

// Known event kinds get a localized title (`runs.events.titles.<kind>`); anything else falls
// back to the server-rendered title. Phase subjects are phase ids and are localized too.
const KNOWN = new Set([
  'run.started',
  'run.completed',
  'run.failed',
  'run.cancelled',
  'run.cancelling',
  'phase.started',
  'phase.finished',
  'phase.completed',
  'phase.failed',
  'phase.skipped',
  'phase.cancelled',
  'machine.ready',
  'container.ready',
  'segment.started',
  'segment.finished',
  'segment.failed',
])

export function eventSubject(ev: Pick<RunEvent, 'kind' | 'subject'>, t: TFunction): string {
  if (!ev.subject) return ''
  if (ev.kind.startsWith('phase.'))
    return t(`common.phase.${ev.subject}`, { defaultValue: ev.subject })
  return ev.subject
}

export function eventTitle(ev: Pick<RunEvent, 'kind' | 'subject' | 'title'>, t: TFunction): string {
  if (!KNOWN.has(ev.kind)) return ev.title
  return t(`runs.events.titles.${ev.kind.replace(/\./g, '_')}`, {
    subject: eventSubject(ev, t),
    defaultValue: ev.title,
  })
}

// `run.failed` → `runs.events.kinds.run` + `runs.events.kinds.failed` for the kind chip.
export function eventKindLabel(kind: string, t: TFunction): string {
  const [group, action] = kind.split('.')
  if (!group || !action) return kind
  const g = t(`runs.events.kinds.${group}`, { defaultValue: group })
  const a = t(`runs.events.kinds.${action}`, { defaultValue: action })
  return `${g} · ${a}`
}
