import { isApiError } from '@api/errors'
import { runMutations } from '@api/queries/runs'
import type { LogPage } from '@api/types'
import { CodeEditorLazy } from '@components/code/CodeEditorLazy'
import { Alert, Button, Drawer, Field, Input, Stack, Text } from '@grafana/ui'
import { downloadText } from '@lib/download'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { LogView } from './LogView'

// Advanced: LogsQL inside the run's enforced scope (server adds the run/tenant labels).
export function LogsRawDrawer({
  slug,
  runId,
  start,
  end,
  onClose,
}: {
  slug: string
  runId: string
  start?: string
  end?: string
  onClose: () => void
}) {
  const { t } = useTranslation()
  const [query, setQuery] = useState('_msg:ERROR')
  const [limit, setLimit] = useState(500)
  const [wrap, setWrap] = useState(true)
  const m = useMutation({
    mutationFn: () => runMutations.logsRaw(slug, runId, { query, start, end, limit }),
  })
  const page = m.data as LogPage | undefined
  const err = m.error
  return (
    <Drawer
      title={t('runs.logs.raw.title')}
      subtitle={t('runs.logs.raw.subtitle')}
      size="lg"
      onClose={onClose}
    >
      <Stack direction="column" gap={1}>
        <Field
          label={t('runs.logs.raw.query')}
          description={t('runs.logs.raw.queryHint')}
          invalid={!!err && isApiError(err) && !!err.validation}
          error={
            err && isApiError(err) && err.validation
              ? err.validation.errors?.[0]?.message
              : undefined
          }
        >
          <CodeEditorLazy
            value={query}
            language="logsql"
            height={100}
            readOnly={false}
            onChange={setQuery}
          />
        </Field>
        <Stack gap={1} alignItems="flex-end" wrap="wrap">
          <Field label={t('runs.logs.raw.limit')}>
            <Input
              type="number"
              width={10}
              value={limit}
              onChange={(e) => setLimit(Number(e.currentTarget.value) || 200)}
            />
          </Field>
          <Field label={t('runs.logs.raw.window')}>
            <Text color="secondary" variant="bodySmall">
              {start || end ? `${start ?? '…'} → ${end ?? '…'}` : t('runs.logs.raw.wholeRun')}
            </Text>
          </Field>
          <div style={{ marginLeft: 'auto' }} />
          <Button
            variant="secondary"
            icon="wrap-text"
            fill={wrap ? 'solid' : 'outline'}
            onClick={() => setWrap((w) => !w)}
          >
            {t('runs.logs.wrap')}
          </Button>
          <Button
            variant="secondary"
            icon="download-alt"
            disabled={!page?.data.length}
            onClick={() =>
              page &&
              downloadText(
                `logs-${runId}.txt`,
                page.data
                  .map((l) => `${l.time} ${l.level ?? ''} ${l.machine ?? ''} ${l.message}`)
                  .join('\n')
              )
            }
          >
            {t('common.actions.download')}
          </Button>
          <Button icon="play" onClick={() => m.mutate()} disabled={m.isPending}>
            {t('runs.logs.raw.run')}
          </Button>
        </Stack>
        {err && !(isApiError(err) && err.validation) && (
          <Alert severity="error" title={isApiError(err) ? err.title : t('common.errors.generic')}>
            {isApiError(err) ? err.detail : String(err)}
          </Alert>
        )}
        <LogView
          lines={page?.data ?? []}
          wrap={wrap}
          follow={false}
          onFollowChange={() => undefined}
          height="calc(100vh - 360px)"
          emptyText={m.isPending ? t('common.misc.loading') : t('runs.logs.raw.empty')}
        />
        {page?.truncated && (
          <Text color="secondary" variant="bodySmall">
            {t('runs.logs.raw.truncated', { count: page.data.length })}
          </Text>
        )}
      </Stack>
    </Drawer>
  )
}
