import { i18n } from '@lib/i18n'
import { afterAll, describe, expect, it } from 'vitest'
import { defaultHardware, hardwareCounts, hardwareLabel, scenarioLabel } from './rating'

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

type Group = [count: number, cpu: number, memory: number]

/** A rated row of a stand on standard-v3 network SSD, keyed like the server does. */
const row = (...groups: Group[]) => {
  const machines = groups.map(([count, cpu, memory_gb]) => ({
    count,
    cpu,
    memory_gb,
    platform: 'standard-v3',
    disk_type: 'network-ssd',
  }))
  const key = machines
    .map((m) => `${m.count}x${m.cpu}c${m.memory_gb}g:${m.platform}:${m.disk_type}`)
    .join('+')
  return { hardware: { key, machines } }
}

describe('defaultHardware', () => {
  const small = row([1, 16, 64])
  const big = row([1, 32, 128])

  it('picks the class with the most rows', () => {
    const rows = [small, big, big, row([2, 32, 128]), small, big]
    expect(defaultHardware(rows)).toBe(big.hardware.key)
  })

  it('on a tie picks the larger stand', () => {
    expect(defaultHardware([small, big])).toBe(big.hardware.key)
    expect(defaultHardware([big, small])).toBe(big.hardware.key)
    const cluster = row([2, 16, 64], [1, 4, 8])
    expect(defaultHardware([cluster, big])).toBe(cluster.hardware.key)
    // Same vCPU: more memory wins.
    expect(defaultHardware([row([1, 32, 64]), big])).toBe(big.hardware.key)
  })

  it('is undefined without rows', () => {
    expect(defaultHardware([])).toBeUndefined()
  })
})

describe('hardwareLabel', () => {
  it('names the platform and the disk of every group', async () => {
    await i18n.changeLanguage('en')
    const hw = {
      key: '',
      machines: [
        {
          count: 1,
          cpu: 32,
          memory_gb: 128,
          platform: 'highfreq-v3',
          disk_type: 'network-ssd-nonreplicated',
        },
        { count: 1, cpu: 4, memory_gb: 8, platform: 'standard-v3', disk_type: '' },
      ],
    }
    expect(hardwareLabel(i18n.t, hw)).toBe(
      '1 × 32 vCPU · 128 GB · highfreq-v3 · network-ssd-nonreplicated + 1 × 4 vCPU · 8 GB · standard-v3'
    )
  })
})

describe('hardwareCounts', () => {
  it('counts rows per class', () => {
    const key = (k: string) => ({ hardware: { key: k, machines: [] } })
    expect([...hardwareCounts(['a', 'b', 'a'].map(key))]).toEqual([
      ['a', 2],
      ['b', 1],
    ])
  })
})
