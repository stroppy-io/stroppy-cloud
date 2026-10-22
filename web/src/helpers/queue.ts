import type { components } from '@api/schema'
import { quotaAmount, quotaTitle } from '@helpers/quotas'
import { i18n } from '@lib/i18n'

type RunQueue = components['schemas']['RunQueue']

/** What a queued run waits for, in the UI language (RunWaitReason). */
export function queueText(q: RunQueue): string {
  switch (q.waiting_for) {
    case 'suite_slot':
      return i18n.t(`runs.queue.${q.waiting_for}`, { live: q.live ?? 0, limit: q.limit ?? 0 })
    case 'quota': {
      const quota = { name: q.quota ?? '', unit: q.unit }
      return i18n.t('runs.queue.quota', {
        quota: quotaTitle(quota),
        zone: q.zone ? i18n.t('runs.queue.quota_zone', { zone: q.zone }) : '',
        need: quotaAmount(quota, q.need ?? 0),
        free: quotaAmount(quota, q.free ?? 0),
      })
    }
    case 'provider':
    case 'subnets':
      return i18n.t(`runs.queue.${q.waiting_for}`, { detail: q.detail ?? '' })
    default:
      return i18n.t(`runs.queue.${q.waiting_for}`)
  }
}
