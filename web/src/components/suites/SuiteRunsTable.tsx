import { suiteRunMutations } from '@api/queries/suites'
import type { SuiteRun } from '@api/types'
import { toast } from '@app/Toaster'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { RunTimeCell, TriggerMark } from '@components/runs/list/RunCells'
import type { IconName } from '@grafana/data'
import { ConfirmModal, Stack, Text } from '@grafana/ui'
import { isTerminal } from '@helpers/run-status'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { suiteRunAsRun } from './run-like'
import { SuiteRunProgressBar } from './SuiteRunProgressBar'
import { SuiteRunShareDialog } from './SuiteRunShareDialog'

const TRIGGER_ICON: Record<SuiteRun['trigger'], IconName> = {
  manual: 'user',
  suite: 'layer-group',
  api: 'brackets-curly',
  schedule: 'clock-nine',
}

type Dialog = { kind: 'none' } | { kind: 'share' | 'delete'; sr: SuiteRun }

// History table of suite runs (a suite's «Runs» tab). The endpoint has no sort params, so the
// columns carry no sort keys; the server returns the newest first.
export function SuiteRunsTable({
  rows,
  loading,
  error,
  onRetry,
  showSuite,
}: {
  rows: SuiteRun[]
  loading?: boolean
  error?: unknown
  onRetry?: () => void
  showSuite?: boolean
}) {
  const { t } = useTranslation()
  const { slug, can } = useTenant()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [dialog, setDialog] = useState<Dialog>({ kind: 'none' })
  const invalidate = () => {
    void qc.invalidateQueries({ queryKey: ['t', slug, 'suites'] })
    void qc.invalidateQueries({ queryKey: ['t', slug, 'suite-runs'] })
    void qc.invalidateQueries({ queryKey: ['t', slug, 'runs'] })
  }
  const onError = (e: unknown) => toast.error(e)
  const cancel = useMutation({
    mutationFn: (sr: SuiteRun) => suiteRunMutations.cancel(slug, sr.id),
    onSuccess: () => {
      toast.success(t('suites.suiteRuns.toasts.cancelled'))
      invalidate()
    },
    onError,
  })
  const retry = useMutation({
    mutationFn: (sr: SuiteRun) => suiteRunMutations.retryFailed(slug, sr.id),
    onSuccess: (next) => {
      toast.success(t('suites.suiteRuns.toasts.retried'), { description: next.name })
      invalidate()
      void navigate({ to: '/t/$slug/suite-runs/$id', params: { slug, id: next.id } })
    },
    onError,
  })
  const remove = useMutation({
    mutationFn: (sr: SuiteRun) => suiteRunMutations.remove(slug, sr.id),
    onSuccess: () => {
      toast.success(t('suites.suiteRuns.toasts.deleted'))
      invalidate()
      setDialog({ kind: 'none' })
    },
    onError: (e) => {
      onError(e)
      setDialog({ kind: 'none' })
    },
  })

  const copy = (text: string) =>
    void navigator.clipboard
      .writeText(text)
      .then(() => toast.success(t('common.actions.copied')))
      .catch((e) => toast.error(e))

  const rowActions = (sr: SuiteRun): RowAction[] => {
    const terminal = isTerminal(sr.status)
    const canRun = can('run')
    const canCancel = can('cancel-any-run', sr.author.id)
    const noPerm = t('runs.actions.noPermission')
    return [
      {
        key: 'open',
        label: t('common.actions.open'),
        icon: 'eye',
        onClick: () =>
          void navigate({ to: '/t/$slug/suite-runs/$id', params: { slug, id: sr.id } }),
      },
      {
        key: 'suite',
        label: t('suites.suiteRuns.actions.openSuite'),
        description: sr.suite.name,
        icon: 'layer-group',
        onClick: () =>
          void navigate({ to: '/t/$slug/suites/$id', params: { slug, id: sr.suite.id } }),
      },
      {
        key: 'runs',
        label: t('suites.suiteRuns.actions.allRuns'),
        icon: 'play',
        onClick: () =>
          void navigate({
            to: '/t/$slug/runs',
            params: { slug },
            search: { suiteRun: sr.id } as never,
          }),
      },
      {
        key: 'share',
        group: true,
        label: t('suites.suiteRuns.actions.share'),
        icon: 'share-alt',
        disabled: !can('share'),
        disabledReason: noPerm,
        onClick: () => setDialog({ kind: 'share', sr }),
      },
      {
        key: 'copyLink',
        label: t('runs.actions.copyLink'),
        icon: 'link',
        onClick: () => copy(`${window.location.origin}/t/${slug}/suite-runs/${sr.id}`),
      },
      {
        key: 'copyId',
        label: t('runs.actions.copyId'),
        icon: 'copy',
        onClick: () => copy(sr.id),
      },
      {
        key: 'retry',
        group: true,
        label: t('suites.suiteRuns.actions.retryFailed'),
        icon: 'repeat',
        disabled: !canRun || !terminal || sr.progress.failed === 0 || retry.isPending,
        disabledReason: !canRun
          ? noPerm
          : !terminal
            ? t('runs.actions.rerunNotTerminal')
            : t('suites.suiteRuns.actions.noFailed'),
        onClick: () => retry.mutate(sr),
      },
      {
        key: 'cancel',
        label: t('suites.suiteRuns.actions.cancel'),
        icon: 'times',
        disabled: !canCancel || terminal || sr.status === 'cancelling' || cancel.isPending,
        disabledReason: !canCancel ? noPerm : t('runs.actions.cancelNotActive'),
        onClick: () => cancel.mutate(sr),
      },
      {
        key: 'delete',
        group: true,
        label: t('suites.suiteRuns.actions.delete'),
        icon: 'trash-alt',
        destructive: true,
        disabled: !canCancel || !terminal,
        disabledReason: !canCancel ? noPerm : t('runs.actions.deleteNotTerminal'),
        onClick: () => setDialog({ kind: 'delete', sr }),
      },
    ]
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure
  const columns = useMemo<DataTableColumn<SuiteRun>[]>(
    () => [
      col.status<SuiteRun>({
        id: 'status',
        title: t('runs.columns.status'),
        status: (sr) => sr.status,
      }),
      col.identity<SuiteRun>({
        id: 'name',
        header: t('suites.suiteRuns.columns.name'),
        render: (sr) => {
          const running = sr.status === 'running' || sr.status === 'cancelling'
          const p = sr.progress
          return {
            title: sr.name,
            link: { to: '/t/$slug/suite-runs/$id', params: { slug, id: sr.id } },
            lead:
              sr.trigger !== 'manual' ? (
                <TriggerMark trigger={sr.trigger} icon={TRIGGER_ICON[sr.trigger]} />
              ) : undefined,
            subtitle: [
              showSuite ? (sr.suite.name ?? sr.suite.id) : undefined,
              running
                ? t('suites.suiteRuns.list.live', { running: p.running, pending: p.pending })
                : sr.author.display_name,
            ]
              .filter(Boolean)
              .join(' · '),
            progress: running ? (p.pct ?? (p.done / Math.max(1, p.total)) * 100) : undefined,
          }
        },
      }),
      col.custom<SuiteRun>({
        id: 'cells',
        header: t('suites.suiteRuns.columns.progress'),
        width: 220,
        cell: (sr) => (
          <Stack direction="column" gap={0.5}>
            <SuiteRunProgressBar progress={sr.progress} />
            <Text color="secondary" variant="bodySmall" truncate>
              {t('suites.suiteRuns.list.cells', {
                done: sr.progress.done,
                count: sr.progress.total,
              })}
              {sr.progress.failed
                ? ` · ${sr.progress.failed} ${t('suites.suiteRuns.progress.failed')}`
                : ''}
            </Text>
          </Stack>
        ),
      }),
      col.custom<SuiteRun>({
        id: 'time',
        header: t('runs.columns.time'),
        width: 180,
        cell: (sr) => <RunTimeCell run={suiteRunAsRun(sr)} />,
      }),
      col.actions<SuiteRun>({ title: (sr) => sr.name, actions: rowActions }),
    ],
    [t, slug, showSuite]
  )
  return (
    <>
      <DataTable<SuiteRun>
        columns={columns}
        data={rows}
        getRowId={(r) => r.id}
        loading={loading}
        error={error}
        onRetry={onRetry}
        rowHref={(r) => `/t/${slug}/suite-runs/${r.id}`}
        empty={{ message: t('suites.suiteRuns.empty') }}
      />
      {dialog.kind === 'share' && (
        <SuiteRunShareDialog suiteRun={dialog.sr} onClose={() => setDialog({ kind: 'none' })} />
      )}
      <ConfirmModal
        isOpen={dialog.kind === 'delete'}
        title={t('common.confirm.deleteTitle', {
          name: dialog.kind === 'delete' ? dialog.sr.name : '',
        })}
        body={t('suites.suiteRuns.confirm.deleteBody')}
        confirmText={t('common.confirm.yesDelete')}
        confirmButtonVariant="destructive"
        dismissText={t('common.actions.cancel')}
        disabled={remove.isPending}
        onConfirm={() => {
          if (dialog.kind === 'delete') remove.mutate(dialog.sr)
        }}
        onDismiss={() => setDialog({ kind: 'none' })}
      />
    </>
  )
}
