import type { TFunction } from 'i18next'
import type { components } from '@/api/schema'

// What makes a rating number comparable, said in words: the scenario a run
// measured and the stand it measured it on.

type HardwareClass = components['schemas']['HardwareClass']

const SCENARIOS: Record<string, string> = {
  tpcc: 'TPC-C',
  tpcb: 'TPC-B',
  tpch: 'TPC-H',
  tpcds: 'TPC-DS',
  ycsb: 'YCSB',
}

/** A family named in words rather than by a benchmark's own name. */
function familyName(t: TFunction, family: string): string {
  if (family === 'banking') return t('common.scenario.banking')
  return SCENARIOS[family] ?? family
}

function variantName(t: TFunction, variant: string): string {
  if (variant === 'procs') return t('common.scenario.procs')
  if (variant === 'inject') return t('common.scenario.inject')
  return variant
}

/**
 * A scenario is the workload's scripts: "tpcc/tx" → "TPC-C", "tpcc/procs" →
 * "TPC-C · stored procedures"; several scripts → "TPC-C + YCSB".
 */
export function scenarioLabel(t: TFunction, scenario: string): string {
  if (!scenario) return '—'
  return scenario
    .split('+')
    .map((script) => {
      const [family = '', variant] = script.split('/')
      const name = familyName(t, family)
      if (!variant || variant === 'tx') return name
      return `${name} · ${variantName(t, variant)}`
    })
    .join(' + ')
}

/**
 * A provider's platform or disk type in words ("highfreq-v3" → "Intel Ice
 * Lake, compute-optimized"); an id without a name (an AWS instance type, a
 * new platform) shows as it is.
 */
function providerName(t: TFunction, kind: 'platform' | 'disk', id: string): string {
  return id ? t(`common.hardware.${kind}.${id}`, { defaultValue: id }) : ''
}

/**
 * "2 × 4 vCPU · 16 GB · Intel Ice Lake · SSD + 1 × 2 vCPU · 4 GB · Intel Ice Lake":
 * the stand without its runner, each machine group with its platform and data disk.
 */
export function hardwareLabel(t: TFunction, hw: HardwareClass | undefined): string {
  if (!hw || hw.machines.length === 0) return '—'
  return hw.machines
    .map((m) =>
      [
        t('common.hardware.group', { count: m.count, cpu: m.cpu, memory: m.memory_gb }),
        providerName(t, 'platform', m.platform),
        providerName(t, 'disk', m.disk_type),
      ]
        .filter(Boolean)
        .join(' · ')
    )
    .join(' + ')
}

/**
 * The class each scenario shows when the viewer picked none: its own class
 * with the most rated rows. Scenarios run on different stands (TPC-C on
 * standard machines, the analytics on compute-optimized ones), so one class
 * for the whole board would hide all but one of them.
 */
export function defaultHardwareByScenario(
  rows: readonly { scenario: string; hardware: HardwareClass; runs?: number }[]
): Map<string, string> {
  const byScenario = new Map<string, { hardware: HardwareClass; runs?: number }[]>()
  for (const r of rows) byScenario.set(r.scenario, [...(byScenario.get(r.scenario) ?? []), r])
  const out = new Map<string, string>()
  for (const [scenario, scenarioRows] of byScenario) {
    const key = defaultHardware(scenarioRows)
    if (key !== undefined) out.set(scenario, key)
  }
  return out
}

/** The class all rows share, so it can be said once above them; undefined when they differ. */
export function sharedHardware(
  rows: readonly { hardware: HardwareClass }[]
): HardwareClass | undefined {
  const first = rows[0]?.hardware
  return first && rows.every((r) => r.hardware.key === first.key) ? first : undefined
}

/** Machines, vCPU and memory of a whole stand. */
function hardwareSize(hw: HardwareClass): [machines: number, cpu: number, memory: number] {
  let machines = 0
  let cpu = 0
  let memory = 0
  for (const m of hw.machines) {
    machines += m.count
    cpu += m.count * m.cpu
    memory += m.count * m.memory_gb
  }
  return [machines, cpu, memory]
}

/** The total of a stand, for a compact chip: "3 machines · 10 vCPU · 36 GB". */
export function hardwareTotal(t: TFunction, hw: HardwareClass | undefined): string {
  if (!hw || hw.machines.length === 0) return '—'
  const [machines, cpu, memory] = hardwareSize(hw)
  return t('common.hardware.total', { count: machines, cpu, memory })
}

/** Rated rows per hardware class key. */
export function hardwareCounts(rows: readonly { hardware: HardwareClass }[]): Map<string, number> {
  const counts = new Map<string, number>()
  for (const r of rows) counts.set(r.hardware.key, (counts.get(r.hardware.key) ?? 0) + 1)
  return counts
}

/**
 * Rows grouped by stand, the stand with the most rated runs behind its rows
 * first (the most measurements to compare; a row without a count weighs one);
 * on a tie the larger stand, then the key. Rows keep their order inside a
 * group.
 */
export function hardwareGroups<T extends { hardware: HardwareClass; runs?: number }>(
  rows: readonly T[]
): { hardware: HardwareClass; rows: T[]; weight: number }[] {
  const groups = new Map<string, { hardware: HardwareClass; rows: T[]; weight: number }>()
  for (const r of rows) {
    const g = groups.get(r.hardware.key) ?? { hardware: r.hardware, rows: [], weight: 0 }
    g.rows.push(r)
    g.weight += r.runs ?? 1
    groups.set(r.hardware.key, g)
  }
  return [...groups.values()].sort((a, b) => {
    if (a.weight !== b.weight) return b.weight - a.weight
    const [, aCpu, aMemory] = hardwareSize(a.hardware)
    const [, bCpu, bMemory] = hardwareSize(b.hardware)
    if (aCpu !== bCpu) return bCpu - aCpu
    if (aMemory !== bMemory) return bMemory - aMemory
    return a.hardware.key.localeCompare(b.hardware.key)
  })
}

/**
 * The class a rating shows when it shows one and the viewer picked none: the
 * first of `hardwareGroups`. Undefined when there are no rows.
 */
export function defaultHardware(
  rows: readonly { hardware: HardwareClass; runs?: number }[]
): string | undefined {
  return hardwareGroups(rows)[0]?.hardware.key
}
