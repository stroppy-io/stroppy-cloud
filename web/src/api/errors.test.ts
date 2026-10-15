import { i18n } from '@lib/i18n'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { issueText } from './errors'

describe('issueText', () => {
  const initial = i18n.language
  beforeAll(() => i18n.changeLanguage('en'))
  afterAll(() => i18n.changeLanguage(initial))

  it('takes the number of a list bound, not the last word', () => {
    expect(
      issueText({
        path: 'zones',
        code: 'ERROR_CODE_MIN_ITEMS_VIOLATED',
        constraint: 'min_items',
        message: 'must have at least 3 items',
        severity: 'ERROR',
      })
    ).toBe('At least 3 items')
  })

  it('keeps a string bound whole', () => {
    expect(
      issueText({
        path: 'image',
        code: 'ERROR_CODE_PREFIX_MISMATCH',
        constraint: 'prefix',
        message: 'must start with "v2"',
        severity: 'ERROR',
      })
    ).toBe('Must start with v2')
  })
})
