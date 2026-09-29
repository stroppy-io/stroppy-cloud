import type { RunPhase, RunStatus } from '@api/types'
import type { IconName } from '@grafana/data'

export type StatusTone = 'success' | 'error' | 'warning' | 'info' | 'secondary'

export interface StatusMeta {
  tone: StatusTone
  icon: IconName
  spin?: boolean
  // i18n key under `common.status.*`
  labelKey: string
  terminal: boolean
}

export function runStatusMeta(status: RunStatus | string | undefined): StatusMeta {
  switch (status) {
    case 'completed':
      return { tone: 'success', icon: 'check-circle', labelKey: 'completed', terminal: true }
    case 'failed':
      return { tone: 'error', icon: 'exclamation-circle', labelKey: 'failed', terminal: true }
    case 'cancelled':
      return { tone: 'secondary', icon: 'minus-circle', labelKey: 'cancelled', terminal: true }
    case 'cancelling':
      return { tone: 'warning', icon: 'sync', spin: true, labelKey: 'cancelling', terminal: false }
    case 'running':
      return { tone: 'info', icon: 'sync', spin: true, labelKey: 'running', terminal: false }
    case 'pending':
      return { tone: 'secondary', icon: 'clock-nine', labelKey: 'pending', terminal: false }
    case 'ready':
    case 'ok':
    case 'synced':
    case 'active':
    case 'online':
    case 'success':
      return { tone: 'success', icon: 'check', labelKey: status, terminal: true }
    case 'verifying':
    case 'creating':
    case 'deleting':
    case 'behind':
      return { tone: 'info', icon: 'sync', spin: true, labelKey: status, terminal: false }
    case 'skipped':
      return { tone: 'secondary', icon: 'forward', labelKey: 'skipped', terminal: true }
    case 'draft':
      return { tone: 'secondary', icon: 'edit', labelKey: 'draft', terminal: true }
    case 'needs_attention':
    case 'stale':
    case 'degraded':
    case 'suspended':
    case 'expired':
      return { tone: 'warning', icon: 'exclamation-triangle', labelKey: status, terminal: true }
    case 'down':
    case 'delete_failed':
    case 'offline':
    case 'orphaned':
    case 'revoked':
    case 'terminated':
      return { tone: 'error', icon: 'times-circle', labelKey: status, terminal: true }
    default:
      return { tone: 'secondary', icon: 'circle', labelKey: status ?? 'unknown', terminal: false }
  }
}

export function isTerminal(status: RunStatus | undefined): boolean {
  return status === 'completed' || status === 'failed' || status === 'cancelled'
}

export const PHASE_ORDER: RunPhase[] = [
  'queued',
  'provisioning',
  'deploying',
  'workload',
  'collecting',
  'teardown',
  'done',
]

export function phaseIcon(phase: RunPhase | string): IconName {
  switch (phase) {
    case 'queued':
      return 'clock-nine'
    case 'provisioning':
      return 'cloud'
    case 'deploying':
      return 'cube'
    case 'workload':
      return 'rocket'
    case 'collecting':
      return 'download-alt'
    case 'teardown':
      return 'trash-alt'
    case 'done':
      return 'check'
    default:
      return 'circle'
  }
}
