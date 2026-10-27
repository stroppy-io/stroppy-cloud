import { describe, expect, it } from 'vitest'
import { clientWait, latencyRows, percentileOf } from './segment-metrics'

const bounds = [
  0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000,
  10000, 20000, 30000, 60000,
]

describe('segment metrics', () => {
  it('interpolates inside the bucket instead of taking its bound', () => {
    // The PostgreSQL 18 nightly: 99.6% of iterations in 100–250 ms.
    const h = {
      type: 'histogram',
      bounds,
      bucket_counts: [
        0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 14, 40, 91, 3253434, 12715, 1066, 36, 0, 0, 0, 0, 0, 0,
      ],
    }
    const p50 = percentileOf(h, 0.5) ?? 0
    const p99 = percentileOf(h, 0.99) ?? 0
    expect(p50).toBeGreaterThan(170)
    expect(p50).toBeLessThan(180)
    expect(p99).toBeGreaterThan(p50)
    expect(percentileOf({ bounds: [10, 20], bucket_counts: [0, 0, 5] }, 0.5)).toBe(20)
    expect(percentileOf(undefined, 0.5)).toBeUndefined()
  })

  it('orders the client and the database first and reads the report buckets', () => {
    const rows = latencyRows({
      name: 'main',
      metrics: {
        tpcc_payment_duration_avg: { value: 130 },
        tx_total_duration_avg: { value: 8 },
        tx_total_duration_p50: { value: 10 },
        iteration_duration_avg: { value: 141 },
        iteration_duration_p50: { value: 250 },
        insert_duration_avg: { value: 5000 },
      },
      report: {
        metrics: {
          iteration_duration: { type: 'histogram', bounds: [100, 250], bucket_counts: [0, 10, 0] },
        },
      },
    })
    expect(rows.map((r) => r.name)).toEqual(['iteration', 'tx_total', 'tpcc_payment'])
    expect(rows[0].p50).toBe(175)
    expect(rows[1].p50).toBe(10)
  })

  it('names a client wait only when it matters', () => {
    const m = (iteration: number, tx: number) => ({
      iteration_duration_avg: { value: iteration },
      tx_total_duration_avg: { value: tx },
    })
    expect(clientWait(m(141, 8.4))?.ms).toBeCloseTo(132.6)
    expect(clientWait(m(50, 34))?.ms).toBe(16)
    expect(clientWait(m(12, 10))).toBeUndefined()
    expect(clientWait({})).toBeUndefined()
  })
})
