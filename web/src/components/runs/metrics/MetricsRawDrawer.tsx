import { isApiError } from '@api/errors'
import { runMutations } from '@api/queries/runs'
import type { RunMetrics } from '@api/types'
import { CodeEditorLazy } from '@components/code/CodeEditorLazy'
import { Alert, Button, Drawer, Field, Input, Stack, Text } from '@grafana/ui'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { MetricPanel } from './MetricPanel'

// Advanced: PromQL within the run scope (server injects the run/tenant selectors).
export function MetricsRawDrawer({
  slug,
  runId,
  start,
  end,
  onClose,
}: {
  slug: string
  runId: string
  start: string
  end: string
  onClose: () => void
}) {
  const { t } = useTranslation()
  const [query, setQuery] = useState('rate(stroppy_transactions_total[1m])')
  const [step, setStep] = useState('15s')
  const m = useMutation({
    mutationFn: () => runMutations.metricsRaw(slug, runId, { query, start, end, step }),
  })
  const data = m.data as RunMetrics | undefined
  const err = m.error
  return (
    <Drawer
      title={t('runs.metrics.raw.title')}
      subtitle={t('runs.metrics.raw.subtitle')}
      size="lg"
      onClose={onClose}
    >
      <Stack direction="column" gap={1}>
        <Field
          label={t('runs.metrics.raw.query')}
          description={t('runs.metrics.raw.queryHint')}
          invalid={!!err && isApiError(err) && !!err.validation}
          error={
            err && isApiError(err) && err.validation
              ? err.validation.errors?.[0]?.message
              : undefined
          }
        >
          <CodeEditorLazy
            value={query}
            language="promql"
            height={100}
            readOnly={false}
            onChange={setQuery}
          />
        </Field>
        <Stack gap={1} alignItems="flex-end" wrap="wrap">
          <Field label={t('runs.metrics.raw.step')}>
            <Input width={10} value={step} onChange={(e) => setStep(e.currentTarget.value)} />
          </Field>
          <Field label={t('runs.metrics.window')}>
            <Text color="secondary" variant="bodySmall">
              {start} → {end}
            </Text>
          </Field>
          <div style={{ marginLeft: 'auto' }} />
          <Button icon="play" onClick={() => m.mutate()} disabled={m.isPending}>
            {t('runs.metrics.raw.run')}
          </Button>
        </Stack>
        {err && !(isApiError(err) && err.validation) && (
          <Alert severity="error" title={isApiError(err) ? err.title : t('common.errors.generic')}>
            {isApiError(err) ? err.detail : String(err)}
          </Alert>
        )}
        <MetricPanel
          title={query}
          metrics={data}
          keys={data?.series.map((s) => s.key) ?? []}
          height={320}
          loading={m.isPending}
        />
        {data && (
          <Text color="secondary" variant="bodySmall">
            {t('runs.metrics.raw.series', {
              count: data.series.length,
              points: data.series.reduce((a, s) => a + s.points.length, 0),
            })}
          </Text>
        )}
      </Stack>
    </Drawer>
  )
}
