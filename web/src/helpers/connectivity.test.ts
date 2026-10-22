import type { RunConnectivity } from '@api/types'
import { describe, expect, it } from 'vitest'
import { connectivityRows, connectivityTone } from './connectivity'

const check: RunConnectivity = {
  total: 3,
  failed: 2,
  critical: 1,
  snapshots: [],
  pairs: [
    { from: 'runner-1', to: 'db-1', port: 9187, critical: false, outcome: 'ok' },
    { from: 'runner-1', to: 'db-1', port: 9100, critical: false, outcome: 'refused' },
    { from: 'db-2', to: 'db-1', port: 5432, critical: true, outcome: 'timeout' },
  ],
}

describe('connectivity', () => {
  it('reads the verdict', () => {
    expect(connectivityTone(check)).toBe('error')
    expect(connectivityTone({ failed: 1, critical: 0 })).toBe('warning')
    expect(connectivityTone({ failed: 0, critical: 0 })).toBe('ok')
  })
  it('lists failed paths first, critical ones before them', () => {
    expect(connectivityRows(check, false).map((p) => p.port)).toEqual([5432, 9100])
    expect(connectivityRows(check, true).map((p) => p.port)).toEqual([5432, 9100, 9187])
  })
})
