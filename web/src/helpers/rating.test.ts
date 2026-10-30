import { i18n } from '@lib/i18n'
import { afterAll, describe, expect, it } from 'vitest'
import { defaultHardware, hardwareCounts, scenarioLabel } from './rating'

describe('scenarioLabel', () => {
  const initial = i18n.language
  afterAll(() => i18n.changeLanguage(initial))

  it.each([
    ['en', 'tpcc/tx', 'TPC-C'],
    ['en', 'tpcc/procs', 'TPC-C · stored procedures'],
    ['ru', 'tpcc/procs', 'TPC-C · хранимые процедуры'],
    ['en', 'tpcds', 'TPC-DS'],
    ['en', 'banking', 'Bank transfers'],
    ['ru', 'banking/inject', 'Банковские переводы · внедрение сбоев'],
    ['en', 'tpcb/tx+ycsb/run', 'TPC-B + YCSB · run'],
    ['en', '', '—'],
  ])('%s: %s → %s', async (lng, scenario, want) => {
    await i18n.changeLanguage(lng)
    expect(scenarioLabel(i18n.t, scenario)).toBe(want)
  })
})

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
