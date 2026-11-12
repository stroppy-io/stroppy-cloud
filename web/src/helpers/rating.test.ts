import { i18n } from '@lib/i18n'
import { afterAll, describe, expect, it } from 'vitest'
import {
  defaultHardware,
  defaultHardwareByScenario,
  hardwareCounts,
  hardwareLabel,
  scenarioLabel,
  sharedHardware,
} from './rating'

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

  it('weighs a class by its runs, not its rows', () => {
    // Four single-node configurations of 15 runs against four clusters with
    // fewer runs each: the single node has more numbers behind it.
    const single = (runs: number) => ({ ...big, runs })
    const cluster = row([3, 32, 128], [1, 4, 8])
    const clusters = (runs: number) => ({ ...cluster, runs })
    const rows = [single(15), single(15), single(16), single(14)]
    const others = [clusters(15), clusters(15), clusters(2), clusters(10)]
    expect(defaultHardware([...others, ...rows])).toBe(big.hardware.key)
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
      '1 × 32 vCPU · 128 GB · Intel Ice Lake, compute-optimized · non-replicated SSD + 1 × 4 vCPU · 8 GB · Intel Ice Lake'
    )
    await i18n.changeLanguage('ru')
    expect(hardwareLabel(i18n.t, hw)).toBe(
      '1 × 32 vCPU · 128 ГБ · Intel Ice Lake, Compute Optimized · нереплицируемый SSD + 1 × 4 vCPU · 8 ГБ · Intel Ice Lake'
    )
  })

  it('shows an id it has no name for as it is', async () => {
    await i18n.changeLanguage('en')
    const hw = {
      key: '',
      machines: [{ count: 1, cpu: 8, memory_gb: 32, platform: 'm7i.2xlarge', disk_type: 'gp3' }],
    }
    expect(hardwareLabel(i18n.t, hw)).toBe('1 × 8 vCPU · 32 GB · m7i.2xlarge · gp3')
  })
})

describe('defaultHardwareByScenario', () => {
  const at = (scenario: string, r: ReturnType<typeof row>) => ({ ...r, scenario })
  it('picks each scenario its own class with the most rows', () => {
    const standard = row([1, 32, 128])
    const highfreq = {
      hardware: {
        key: '1x32c128g:highfreq-v3:network-ssd-nonreplicated',
        machines: [
          {
            count: 1,
            cpu: 32,
            memory_gb: 128,
            platform: 'highfreq-v3',
            disk_type: 'network-ssd-nonreplicated',
          },
        ],
      },
    }
    const rows = [
      at('tpcc/tx', standard),
      at('tpcc/tx', standard),
      at('tpcc/tx', row([3, 32, 128])),
      at('tpch/tx', highfreq),
      at('tpch/tx', highfreq),
      at('tpch/tx', standard),
    ]
    expect(defaultHardwareByScenario(rows)).toEqual(
      new Map([
        ['tpcc/tx', standard.hardware.key],
        ['tpch/tx', highfreq.hardware.key],
      ])
    )
  })

  it('is empty without rows', () => {
    expect(defaultHardwareByScenario([]).size).toBe(0)
  })
})

describe('sharedHardware', () => {
  it('is the class every row has, or undefined when they differ', () => {
    const a = row([1, 32, 128])
    const b = row([3, 32, 128])
    expect(sharedHardware([a, a])).toEqual(a.hardware)
    expect(sharedHardware([a, b])).toBeUndefined()
    expect(sharedHardware([])).toBeUndefined()
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
