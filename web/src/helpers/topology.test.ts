import { describe, expect, it } from 'vitest'
import { topologyText } from './topology'

const node = (role: string, count = 1, engine?: string) => ({ role, count, engine }) as never

describe('topologyText', () => {
  it('says the shape, not the role names', () => {
    expect(topologyText({ nodes: [node('db'), node('runner')] })).toBe('Single server')
    expect(topologyText({ nodes: [node('db'), node('db-replica', 2)] })).toBe(
      'Primary · Replica ×2'
    )
    expect(topologyText({ nodes: [node('db', 3), node('proxy', 1, 'haproxy')] })).toBe(
      'Cluster ×3 · haproxy'
    )
    expect(
      topologyText({
        mode: 'galera',
        nodes: [node('db', 3), node('proxy', 1, 'proxysql')],
      } as never)
    ).toBe('Galera ×3 · proxysql')
    expect(topologyText({ nodes: [node('db'), node('db-compute')] })).toBe('Storage · Query')
    expect(
      topologyText({ mode: 'mirror-3-dc', nodes: [node('db', 9), node('db-compute', 3)] } as never)
    ).toBe('mirror-3-dc · Storage ×9 · Query ×3')
    expect(
      topologyText({
        mode: 'patroni',
        nodes: [node('db'), node('db-replica', 2), node('etcd', 3), node('proxy', 1, 'haproxy')],
      } as never)
    ).toBe('Patroni · Primary · Replica ×2 · etcd ×3 · haproxy')
  })
})
