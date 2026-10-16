import type { components } from '@api/schema'
import { i18n } from '@lib/i18n'
import { afterAll, describe, expect, it } from 'vitest'
import { queueText } from './queue'

type Reason = components['schemas']['RunWaitReason']

// Every wait reason of the contract reads as a sentence in both languages.
const reasons: Reason[] = [
  'pipelines',
  'slot',
  'suite_slot',
  'quota',
  'quota_unknown',
  'provider',
  'suspended',
]

describe('queueText', () => {
  const initial = i18n.language
  afterAll(() => i18n.changeLanguage(initial))

  for (const lng of ['ru', 'en']) {
    it(`names every wait reason in ${lng}`, async () => {
      await i18n.changeLanguage(lng)
      for (const r of reasons) {
        const text = queueText({
          waiting_for: r,
          position: 1,
          live: 3,
          limit: 3,
          quota: 'compute.ssdDisks.size',
          unit: 'bytes',
          need: 1e12,
          free: 5e11,
        })
        expect(text).not.toContain('runs.queue')
        expect(text).not.toContain('{{')
      }
    })
  }

  it('says which quota and zone hold the run', async () => {
    await i18n.changeLanguage('ru')
    expect(
      queueText({
        waiting_for: 'quota',
        position: 2,
        quota: 'compute.ssdDisks.size',
        zone: 'ru-central1-d',
        unit: 'bytes',
        need: 1e12,
        free: 5e11,
      })
    ).toContain('ru-central1-d')
  })
})
