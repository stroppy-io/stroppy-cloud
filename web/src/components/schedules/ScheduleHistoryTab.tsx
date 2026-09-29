import { scheduleQueries } from '@api/queries/suites'
import type { Schemas } from '@api/types'
import { toast } from '@app/Toaster'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { Box } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

type Ref = Schemas['ScheduleRunRef']

// Firings of a schedule: status · what it started (run or suite run; the firing error as the
// second line) · when. The history endpoint has no sort params; newest first from the server.
export function ScheduleHistoryTab({ scheduleId }: { scheduleId: string }) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const navigate = useNavigate()
  const history = useQuery({
    ...scheduleQueries.history(slug, scheduleId),
    refetchInterval: (q) =>
      q.state.data?.data.some((r) => r.status === 'running' || r.status === 'pending')
        ? 5000
        : false,
  })

  const href = (r: Ref) =>
    r.kind === 'run' ? `/t/${slug}/runs/${r.id}` : `/t/${slug}/suite-runs/${r.id}`

  const copy = (text: string) =>
    void navigator.clipboard
      .writeText(text)
      .then(() => toast.success(t('common.actions.copied')))
      .catch((e) => toast.error(e))

  const rowActions = (r: Ref): RowAction[] => [
    {
      key: 'open',
      label: t('common.actions.open'),
      icon: 'eye',
      onClick: () => void navigate({ to: href(r) as never }),
    },
    {
      key: 'copyLink',
      group: true,
      label: t('runs.actions.copyLink'),
      icon: 'link',
      onClick: () => copy(`${window.location.origin}${href(r)}`),
    },
    {
      key: 'copyId',
      label: t('runs.actions.copyId'),
      icon: 'copy',
      onClick: () => copy(r.id),
    },
  ]

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure
  const columns = useMemo<DataTableColumn<Ref>[]>(
    () => [
      col.status<Ref>({
        id: 'status',
        title: t('schedules.history.columns.status'),
        status: (r) => r.status,
      }),
      col.identity<Ref>({
        id: 'name',
        header: t('schedules.history.columns.name'),
        render: (r) => ({
          title: r.name ?? r.id,
          icon: r.kind === 'run' ? 'play' : 'layer-group',
          link:
            r.kind === 'run'
              ? { to: '/t/$slug/runs/$id', params: { slug, id: r.id } }
              : { to: '/t/$slug/suite-runs/$id', params: { slug, id: r.id } },
          subtitle: r.error ?? t(`schedules.history.kind.${r.kind}`),
        }),
      }),
      col.time<Ref>({
        id: 'at',
        header: t('schedules.history.columns.at'),
        width: 150,
        value: (r) => r.at,
      }),
      col.actions<Ref>({ title: (r) => r.name ?? r.id, actions: rowActions }),
    ],
    [t, slug]
  )
  return (
    <Box marginTop={2}>
      <DataTable<Ref>
        columns={columns}
        data={history.data?.data ?? []}
        getRowId={(r) => `${r.kind}:${r.id}`}
        loading={history.isPending}
        error={history.isError ? history.error : undefined}
        onRetry={() => void history.refetch()}
        rowHref={href}
        empty={{ message: t('schedules.history.empty') }}
      />
    </Box>
  )
}
