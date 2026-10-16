import { i18n } from '@lib/i18n'

// Catalog metrics (the contract's MetricKey) in the UI language
// (`catalog.metrics.<key>`); lib/vocab.test.ts keeps every key covered.
export function metricTitle(key: string, fallback?: string): string {
  const k = `catalog.metrics.${key}.title`
  return i18n.exists(k) ? i18n.t(k) : (fallback ?? key)
}

export function metricDescription(key: string, fallback?: string): string | undefined {
  const k = `catalog.metrics.${key}.description`
  return i18n.exists(k) ? i18n.t(k) : fallback
}
