import type { Run, SuiteRun } from '@api/types'
import { toast } from '@app/Toaster'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import type { TFunction } from 'i18next'

// The run cells (`components/runs/list/RunCells`) read a handful of `Run` fields: status, the
// start / finish / duration and the denormalized `summary`. Suite runs and suite-run cells carry
// exactly those fields under the same names, so they are viewed through a `Run`-shaped object
// instead of duplicating the cells. Only the fields listed here are populated.

type SuiteRunCell = SuiteRun['cells'][number]

export function suiteRunCellAsRun(c: SuiteRunCell): Run {
  return {
    id: c.run?.id ?? c.cell_id,
    name: c.run?.name ?? c.name ?? c.cell_id,
    status: c.status,
    started_at: c.run?.started_at ?? null,
    summary: c.summary,
  } as unknown as Run
}

export function suiteRunAsRun(sr: SuiteRun): Run {
  return {
    id: sr.id,
    name: sr.name,
    status: sr.status,
    started_at: sr.started_at ?? null,
    finished_at: sr.finished_at ?? null,
    duration: sr.duration ?? null,
  } as unknown as Run
}

// Kebab of a row that points at a run which may not exist yet (a suite-run cell still waiting
// for a slot): every action is listed and disabled with the reason until the run appears.
export function runRefActions({
  runId,
  slug,
  t,
  navigate,
  notStartedReason,
}: {
  runId: string | undefined
  slug: string
  t: TFunction
  navigate: (to: string) => void
  notStartedReason: string
}): RowAction[] {
  const copy = (text: string) =>
    void navigator.clipboard
      .writeText(text)
      .then(() => toast.success(t('common.actions.copied')))
      .catch((e) => toast.error(e))
  const base = `/t/${slug}/runs/${runId}`
  const off = { disabled: !runId, disabledReason: notStartedReason }
  return [
    {
      key: 'open',
      label: t('suites.suiteRuns.cells.actions.openRun'),
      icon: 'eye',
      ...off,
      onClick: () => navigate(base),
    },
    {
      key: 'logs',
      label: t('runs.tabs.logs'),
      icon: 'document-info',
      ...off,
      onClick: () => navigate(`${base}/logs`),
    },
    {
      key: 'metrics',
      label: t('runs.tabs.metrics'),
      icon: 'chart-line',
      ...off,
      onClick: () => navigate(`${base}/metrics`),
    },
    {
      key: 'copyLink',
      group: true,
      label: t('runs.actions.copyLink'),
      icon: 'link',
      ...off,
      onClick: () => copy(`${window.location.origin}${base}`),
    },
    {
      key: 'copyId',
      label: t('runs.actions.copyId'),
      icon: 'copy',
      ...off,
      onClick: () => runId && copy(runId),
    },
  ]
}
