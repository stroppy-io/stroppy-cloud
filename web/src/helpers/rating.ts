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
