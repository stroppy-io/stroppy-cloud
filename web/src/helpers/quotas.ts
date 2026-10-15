import { formatBytes, formatNumber } from '@helpers/format'
import { i18n } from '@lib/i18n'

// Provider quotas (pipelines/internal/cloud: Quota Manager ids for Yandex,
// Service Quotas codes for AWS) in the UI language; pipelines/internal/cloud
// quota_uitext_test.go keeps every id and unit covered.
interface QuotaLike {
  name: string
  title?: string | null
  unit?: string | null
}

const key = (name: string) => name.replace(/[.-]/g, '_')

export function quotaTitle(q: QuotaLike): string {
  const k = `common.quota.${key(q.name)}`
  return i18n.exists(k) ? i18n.t(k) : (q.title ?? q.name)
}

/** An amount of the quota: bytes as sizes, everything else as a count with its unit. */
export function quotaAmount(q: QuotaLike, value: number): string {
  if (q.unit === 'bytes') return formatBytes(value)
  const unit = q.unit ? i18n.t(`common.quotaUnit.${key(q.unit)}`, { defaultValue: q.unit }) : ''
  return `${formatNumber(value, 1)}${unit ? ` ${unit}` : ''}`
}

/** The number alone (bytes still as a size), for "used of limit". */
export function quotaValue(q: QuotaLike, value: number): string {
  return q.unit === 'bytes' ? formatBytes(value) : formatNumber(value, 1)
}
