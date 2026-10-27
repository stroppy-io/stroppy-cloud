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

/** "tpcc" → "TPC-C"; several scripts in one workload → "TPC-C + YCSB". */
export function scenarioLabel(scenario: string): string {
  if (!scenario) return '—'
  return scenario
    .split('+')
    .map((s) => SCENARIOS[s] ?? s)
    .join(' + ')
}

/** "2 × 4 vCPU · 16 GB + 1 × 2 vCPU · 4 GB": the stand without its runner. */
export function hardwareLabel(t: TFunction, hw: HardwareClass | undefined): string {
  if (!hw || hw.machines.length === 0) return '—'
  return hw.machines
    .map((m) => t('common.hardware.group', { count: m.count, cpu: m.cpu, memory: m.memory_gb }))
    .join(' + ')
}

/** The total of a stand, for a compact chip: "3 machines · 10 vCPU · 36 GB". */
export function hardwareTotal(t: TFunction, hw: HardwareClass | undefined): string {
  if (!hw || hw.machines.length === 0) return '—'
  let machines = 0
  let cpu = 0
  let memory = 0
  for (const m of hw.machines) {
    machines += m.count
    cpu += m.count * m.cpu
    memory += m.count * m.memory_gb
  }
  return t('common.hardware.total', { count: machines, cpu, memory })
}

/** Total vCPU and memory of a class key ("2x32c128g+1x4c8g"), to say which stand is larger. */
function hardwareSize(key: string): [cpu: number, memory: number] {
  let cpu = 0
  let memory = 0
  for (const group of key.split('+')) {
    const m = /^(\d+)x(\d+)c(\d+)g$/.exec(group)
    if (!m) continue
    cpu += Number(m[1]) * Number(m[2])
    memory += Number(m[1]) * Number(m[3])
  }
  return [cpu, memory]
}

/** Rated rows per hardware class key. */
export function hardwareCounts(rows: readonly { hardware: HardwareClass }[]): Map<string, number> {
  const counts = new Map<string, number>()
  for (const r of rows) counts.set(r.hardware.key, (counts.get(r.hardware.key) ?? 0) + 1)
  return counts
}

/**
 * The class a rating shows when the viewer picked none: the one with the most
 * rated rows (the largest set of comparable numbers); on a tie the larger
 * stand, then the key. Undefined when there are no rows.
 */
export function defaultHardware(rows: readonly { hardware: HardwareClass }[]): string | undefined {
  let best: string | undefined
  let bestCount = 0
  for (const [key, count] of hardwareCounts(rows)) {
    if (best === undefined || count > bestCount) {
      best = key
      bestCount = count
      continue
    }
    if (count < bestCount) continue
    const [cpu, memory] = hardwareSize(key)
    const [bestCpu, bestMemory] = hardwareSize(best)
    if (
      cpu > bestCpu ||
      (cpu === bestCpu && (memory > bestMemory || (memory === bestMemory && key < best)))
    )
      best = key
  }
  return best
}
