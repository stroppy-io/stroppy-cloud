import {
  auditActionValues,
  auditTargetKindValues,
  commitDurabilityValues,
  errorReasonValues,
  metricGroupValues,
  metricKeyValues,
  metricKindValues,
  metricOriginValues,
  quotaNameValues,
  quotaUnitValues,
  requirementReasonValues,
  runEventKindValues,
  runStageKindValues,
  runWaitReasonValues,
  topologyModeValues,
  topologyRoleValues,
  tuningModeValues,
} from '@api/enums'
import { eventKey } from '@components/runs/events/eventTitle'
import { describe, expect, it } from 'vitest'
import { i18n } from './i18n'

// Every value of every vocabulary the server sends (the contract's enums) has its text in both
// languages. Log levels and database kinds are shown as they are: they are names, not words.
const key = (s: string) => s.replace(/[.-]/g, '_')

// Database groups read as the product's name in any language.
const productGroups = new Set(['PostgreSQL', 'MySQL', 'CockroachDB', 'Picodata'])

const vocabularies: Record<string, string[]> = {
  TopologyRole: topologyRoleValues.map((r) => `common.topologyRole.${r}`),
  TopologyMode: topologyModeValues.map((m) => `common.topologyMode.${m}`),
  RequirementReason: requirementReasonValues.map((r) => `library.requirements.reasons.${r}`),
  RunEventKind: runEventKindValues.map((k) => `runs.events.titles.${eventKey(k)}`),
  RunWaitReason: runWaitReasonValues.map((r) => `runs.queue.${r}`),
  RunStageKind: runStageKindValues.map((k) => `runs.stage.${k}`),
  AuditAction: auditActionValues.map((a) => `settings.audit.actions.${a.replaceAll('.', '_')}`),
  AuditTargetKind: auditTargetKindValues.map((k) => `settings.audit.targetKinds.${k}`),
  MetricKey: metricKeyValues.map((m) => `catalog.metrics.${m}.title`),
  MetricGroup: metricGroupValues
    .filter((g) => !productGroups.has(g))
    .map((g) => `runs.metrics.groups.${g.toLowerCase()}`),
  QuotaName: quotaNameValues.map((q) => `common.quota.${key(q)}`),
  QuotaUnit: quotaUnitValues.map((u) => `common.quotaUnit.${key(u)}`),
  ErrorReason: errorReasonValues.map((r) => `common.errors.reasons.${r}`),
  MetricOrigin: metricOriginValues.map((o) => `runs.metrics.origin.${o}`),
  MetricKind: metricKindValues.map((k) => `runs.metrics.kind.${k}`),
  TuningMode: tuningModeValues.map((m) => `runs.overview.tuning.modes.${m}`),
  CommitDurability: commitDurabilityValues.map((d) => `runs.overview.tuning.durabilities.${d}`),
}

describe('vocabularies', () => {
  for (const lng of ['ru', 'en'])
    for (const [name, keys] of Object.entries(vocabularies))
      it(`${name} reads in ${lng}`, () => {
        const missing = keys.filter((k) => !i18n.exists(k, { lng }))
        expect(missing).toEqual([])
      })
})
