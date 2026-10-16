import { describe, expect, it } from 'vitest'
import {
  decodePick,
  encodePick,
  facetOf,
  filterNames,
  seriesLabel,
  sortNames,
  sortSeries,
  unitOf,
} from './metric-explorer'

const names = [
  {
    name: 'node_cpu_seconds_total',
    origin: 'exporter',
    kind: 'counter',
    series: 32,
    machines: ['db-1', 'runner-1'],
    roles: ['db', 'runner'],
    containers: ['db-1-node-exporter', 'runner-1-node-exporter'],
  },
  {
    name: 'pg_up',
    origin: 'exporter',
    kind: 'gauge',
    series: 1,
    machines: ['db-1'],
    roles: ['db'],
    containers: ['db-1-postgres-exporter'],
  },
  {
    name: 'stroppy_iterations_total',
    origin: 'workload',
    kind: 'counter',
    series: 2,
    machines: ['runner-1'],
    roles: ['runner'],
    containers: [],
  },
] as const

describe('metric explorer', () => {
  it('round-trips a pick through the URL form', () => {
    const p = {
      name: 'docker.container.cpu.percent',
      fn: 'value' as const,
      by: ['machine', 'mode'],
      agg: 'max' as const,
    }
    expect(decodePick(encodePick(p))).toEqual(p)
    expect(decodePick('pg_up~~~')).toEqual({ name: 'pg_up', fn: undefined, by: [], agg: undefined })
    expect(decodePick('')).toBeUndefined()
  })

  it('reads the unit from the name', () => {
    expect(unitOf('node_network_receive_bytes_total', 'rate')).toBe('Bps')
    expect(unitOf('node_memory_MemAvailable_bytes', 'value')).toBe('bytes')
    expect(unitOf('node_cpu_seconds_total', 'rate')).toBe('percentunit')
    expect(unitOf('pg_stat_database_xact_commit', 'rate')).toBe('ops')
    expect(unitOf('pg_up', 'value')).toBeUndefined()
  })

  it('names a series by its labels and origin', () => {
    expect(
      seriesLabel(
        {
          labels: { mode: 'user', cpu: '0' },
          machine: 'db-1',
          container: 'db-1-node-exporter',
          points: [],
        },
        'm'
      )
    ).toBe('cpu=0, mode=user, db-1 · db-1-node-exporter')
    expect(seriesLabel({ labels: {}, points: [] }, 'm')).toBe('m')
  })

  it('filters by search, origin and where the series are', () => {
    const list = [...names] as never[]
    expect(filterNames(list, { q: 'postgres' }).map((m: { name: string }) => m.name)).toEqual([
      'pg_up',
    ])
    expect(filterNames(list, { origin: ['workload'] })).toHaveLength(1)
    expect(filterNames(list, { role: ['db'] })).toHaveLength(2)
    expect(filterNames(list, { kind: ['counter'], machine: ['runner-1'] })).toHaveLength(2)
  })

  it('sorts names and counts facets', () => {
    const list = [...names] as never[]
    expect(sortNames(list, 'series').map((m: { name: string }) => m.name)[0]).toBe(
      'node_cpu_seconds_total'
    )
    expect(sortNames(list, '-name').map((m: { name: string }) => m.name)[0]).toBe(
      'stroppy_iterations_total'
    )
    expect(facetOf(list, (m: { roles: string[] }) => m.roles)).toEqual([
      { value: 'db', count: 2 },
      { value: 'runner', count: 2 },
    ])
  })

  it('sorts series by any column', () => {
    const s = [
      { label: 'b', labels: {}, points: [], aggregates: { avg: 1, max: 9 } },
      { label: 'a', labels: {}, points: [], aggregates: { avg: 5, max: 2 } },
    ]
    expect(sortSeries(s, 'avg', true)[0].label).toBe('a')
    expect(sortSeries(s, 'max', true)[0].label).toBe('b')
    expect(sortSeries(s, 'label', false)[0].label).toBe('a')
  })
})
