import { describe, expect, it } from 'vitest'
import { defaultHardware, hardwareCounts } from './rating'

const row = (key: string) => ({ hardware: { key, machines: [] } })

describe('defaultHardware', () => {
  it('picks the class with the most rows', () => {
    const rows = ['1x16c64g', '1x32c128g', '1x32c128g', '2x32c128g', '1x16c64g', '1x32c128g']
    expect(defaultHardware(rows.map(row))).toBe('1x32c128g')
  })

  it('on a tie picks the larger stand', () => {
    expect(defaultHardware(['1x16c64g', '1x32c128g'].map(row))).toBe('1x32c128g')
    expect(defaultHardware(['1x32c128g', '1x16c64g'].map(row))).toBe('1x32c128g')
    expect(defaultHardware(['2x16c64g+1x4c8g', '1x32c128g'].map(row))).toBe('2x16c64g+1x4c8g')
    // Same vCPU: more memory wins.
    expect(defaultHardware(['1x32c64g', '1x32c128g'].map(row))).toBe('1x32c128g')
  })

  it('is undefined without rows', () => {
    expect(defaultHardware([])).toBeUndefined()
  })
})

describe('hardwareCounts', () => {
  it('counts rows per class', () => {
    expect([...hardwareCounts(['a', 'b', 'a'].map(row))]).toEqual([
      ['a', 2],
      ['b', 1],
    ])
  })
})
