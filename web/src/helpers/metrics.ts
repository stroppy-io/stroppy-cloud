import { i18n } from '@lib/i18n'

// Catalog metrics in the UI language (`catalog.metrics.<key>`); the server's
// English title stays the fallback for a metric the UI does not know yet.
// internal/domain/catalog/uitext_test.go keeps the catalog covered.
export function metricTitle(key: string, fallback?: string): string {
  const k = `catalog.metrics.${key}.title`
  return i18n.exists(k) ? i18n.t(k) : (fallback ?? key)
}

export function metricDescription(key: string, fallback?: string): string | undefined {
  const k = `catalog.metrics.${key}.description`
  return i18n.exists(k) ? i18n.t(k) : fallback
}
