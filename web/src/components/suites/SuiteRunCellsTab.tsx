import type { SuiteRun } from '@api/types'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import {
  databaseView,
  RunMetricsCell,
  RunTimeCell,
  workloadView,
} from '@components/runs/list/RunCells'
import { Box } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useNavigate } from '@tanstack/react-router'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { runRefActions, suiteRunCellAsRun } from './run-like'

type Cell = SuiteRun['cells'][number]

// Cells of a suite run. A row is a run (or a slot waiting for one), so it reads like the runs
// table: status · cell/run · database · workload · metrics · time.
export function SuiteRunCellsTab({ suiteRun }: { suiteRun: SuiteRun }) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const navigate = useNavigate()

  const rowActions = (c: Cell): RowAction[] =>
    runRefActions({
      runId: c.run?.id,
      slug,
      t,
      navigate: (to) => void navigate({ to: to as never }),
      notStartedReason: t('suites.suiteRuns.cells.notStarted'),
    })

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure
  const columns = useMemo<DataTableColumn<Cell>[]>(
    () => [
      col.status<Cell>({
        id: 'status',
        title: t('runs.columns.status'),
        status: (c) => c.status,
      }),
      col.identity<Cell>({
        id: 'cell',
        header: t('suites.suiteRuns.cells.columns.cell'),
        render: (c) => {
          const running = c.status === 'running' || c.status === 'cancelling'
          const pct = c.summary?.progress_pct ?? 0
          return {
            title: c.name ?? c.cell_id,
            link: c.run ? { to: '/t/$slug/runs/$id', params: { slug, id: c.run.id } } : undefined,
            subtitle: !c.run
              ? t('suites.suiteRuns.cells.notStarted')
              : running
                ? [
                    c.summary?.segment ? t('runs.segment', { name: c.summary.segment }) : '',
                    t('runs.progress', { pct }),
                  ]
                    .filter(Boolean)
                    .join(' · ')
                : (c.run.name ?? c.run.id),
            progress: running ? pct : undefined,
          }
        },
      }),
      col.stack<Cell>({
        id: 'database',
        header: t('runs.columns.database'),
        minWidth: 200,
        value: (c) => c.summary?.db_kind,
        render: (c) => databaseView(suiteRunCellAsRun(c), t),
      }),
      col.stack<Cell>({
        id: 'workload',
        header: t('runs.columns.workload'),
        minWidth: 200,
        render: (c) => workloadView(suiteRunCellAsRun(c), t),
      }),
      col.custom<Cell>({
        id: 'metrics',
        header: t('runs.columns.metrics'),
        width: 290,
        cell: (c) => <RunMetricsCell run={suiteRunCellAsRun(c)} />,
      }),
      col.custom<Cell>({
        id: 'time',
        header: t('runs.columns.time'),
        width: 180,
        cell: (c) => <RunTimeCell run={suiteRunCellAsRun(c)} />,
      }),
      col.actions<Cell>({ title: (c) => c.name ?? c.cell_id, actions: rowActions }),
    ],
    [t, slug]
  )
  return (
    <Box marginTop={2}>
      <DataTable<Cell>
        columns={columns}
        data={suiteRun.cells}
        getRowId={(c) => c.cell_id}
        rowHref={(c) => (c.run ? `/t/${slug}/runs/${c.run.id}` : undefined)}
        empty={{ message: t('suites.suiteRuns.cells.empty') }}
      />
    </Box>
  )
}
