import type { RunEvent } from '@api/types'
import { i18n } from '@lib/i18n'
import type { TFunction } from 'i18next'

// An event kind as the locale names it: the pipeline's milestones are
// `phase.started`, Graphene's run lifecycle is `run-started` /
// `run-canceled`; both become `phase_started` / `run_cancelled`.
// internal/domain/run/uitext_test.go keeps every kind the server sends
// covered in `runs.events.titles`.
export function eventKey(kind: string): string {
  return kind.replace(/[.-]/g, '_').replace('canceled', 'cancelled')
}

export function eventSubject(ev: Pick<RunEvent, 'kind' | 'subject'>, t: TFunction): string {
  if (!ev.subject) return ''
  if (ev.kind.startsWith('phase.'))
    return t(`common.phase.${ev.subject}`, { defaultValue: ev.subject })
  return ev.subject
}

/** The event's title in the UI language; an unknown kind keeps the server's title. */
export function eventTitle(ev: Pick<RunEvent, 'kind' | 'subject' | 'title'>, t: TFunction): string {
  const key = `runs.events.titles.${eventKey(ev.kind)}`
  if (!i18n.exists(key)) return ev.title
  return t(key, { subject: eventSubject(ev, t) })
}

// `run.failed`, `run-failed` → `runs.events.kinds.run` + `runs.events.kinds.failed` for the chip.
export function eventKindLabel(kind: string, t: TFunction): string {
  const [group, ...rest] = eventKey(kind).split('_')
  const action = rest.join('_')
  if (!group || !action) return kind
  const g = t(`runs.events.kinds.${group}`, { defaultValue: group })
  const a = t(`runs.events.kinds.${action}`, { defaultValue: action })
  return `${g} · ${a}`
}
