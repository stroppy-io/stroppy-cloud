import { runMutations, runQueries } from '@api/queries/runs'
import type { Artifact } from '@api/types'
import { toast } from '@app/Toaster'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { Stack, Text } from '@grafana/ui'
import { formatBytes } from '@helpers/format'
import { isTerminal } from '@helpers/run-status'
import { useCopy } from '@hooks/useCopy'
import { useRun } from '@hooks/useRun'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

// Artifacts of a run (a tab: the table grows, the tab body scrolls). Bounded list → client sort.
export function RunArtifactsTab({ id }: { id: string }) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const copy = useCopy()
  const run = useRun(slug, id).data
  const terminal = isTerminal(run?.status)
  const q = useQuery({
    ...runQueries.artifacts(slug, id),
    refetchInterval: terminal ? false : 30_000,
  })
  const download = useMutation({
    mutationFn: (a: Artifact) => runMutations.artifactUrl(slug, id, a.id),
    onSuccess: (res) => {
      const url = (res as { url?: string }).url
      if (url) window.open(url, '_blank', 'noopener')
    },
    onError: (e) => toast.error(e),
  })
  const kindLabel = (a: Artifact) => t(`runs.artifacts.kinds.${a.kind}`, { defaultValue: a.kind })

  const rowActions = (a: Artifact): RowAction[] => [
    {
      key: 'download',
      label: t('common.actions.download'),
      icon: 'download-alt',
      disabled: download.isPending && download.variables?.id === a.id,
      disabledReason: t('runs.artifacts.downloading'),
      onClick: () => download.mutate(a),
    },
    {
      key: 'copyName',
      group: true,
      label: t('runs.artifacts.copyName'),
      icon: 'copy',
      onClick: () => copy(a.name),
    },
    {
      key: 'copyDigest',
      label: t('runs.artifacts.copyDigest'),
      icon: 'copy',
      disabled: !a.digest,
      disabledReason: t('runs.artifacts.noDigest'),
      onClick: () => a.digest && copy(a.digest),
    },
    {
      key: 'copyId',
      label: t('runs.artifacts.copyId'),
      icon: 'copy',
      onClick: () => copy(a.id),
    },
  ]

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure over the download state
  const columns = useMemo<DataTableColumn<Artifact>[]>(
    () => [
      col.identity<Artifact>({
        id: 'name',
        header: t('common.fields.name'),
        render: (a) => ({ title: a.name, icon: 'file-alt', subtitle: a.content_type }),
      }),
      col.text<Artifact>({
        id: 'kind',
        header: t('common.fields.kind'),
        width: 150,
        value: kindLabel,
      }),
      col.number<Artifact>({
        id: 'size',
        header: t('common.fields.size'),
        width: 120,
        value: (a) => a.size_bytes,
        format: formatBytes,
        bar: true,
      }),
      col.text<Artifact>({
        id: 'digest',
        header: t('runs.artifacts.digest'),
        width: 180,
        value: (a) => a.digest,
      }),
      col.time<Artifact>({
        id: 'created',
        header: t('common.fields.created'),
        value: (a) => a.created_at,
      }),
      col.actions<Artifact>({ title: (a) => a.name, actions: rowActions }),
    ],
    [t, download.isPending, download.variables]
  )
  const items = q.data?.data ?? []
  const total = items.reduce((a, x) => a + (x.size_bytes ?? 0), 0)
  return (
    <Stack direction="column" gap={1}>
      {items.length > 0 && (
        <Text color="secondary" variant="bodySmall">
          {t('runs.artifacts.summary', { count: items.length, size: formatBytes(total) })}
        </Text>
      )}
      <DataTable<Artifact>
        columns={columns}
        data={items}
        getRowId={(a) => a.id}
        clientSort
        loading={q.isPending}
        error={q.isError ? q.error : undefined}
        onRetry={() => void q.refetch()}
        empty={{
          message: terminal ? t('runs.artifacts.emptyTerminal') : t('runs.artifacts.emptyRunning'),
        }}
      />
    </Stack>
  )
}
