import type { RunOverview } from '@api/types'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { useCopy } from '@hooks/useCopy'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

// Stand tables of the run overview: deployed components, machines, workload segments. Bounded
// lists inside the overview tab — they grow with their rows and sort on the client.

type Component = RunOverview['components'][number]
type Machine = RunOverview['machines'][number]
type Segment = NonNullable<RunOverview['workload_segments']>[number]
type Endpoint = NonNullable<Component['endpoints']>[number]

function endpointAddr(e: Endpoint): string {
  return `${e.address ?? ''}${e.port ? `:${e.port}` : ''}`
}

function endpointLabel(e: Endpoint): string {
  return `${e.name ? `${e.name} ` : ''}${endpointAddr(e)}`
}

export function ComponentsTable({ components }: { components: Component[] }) {
  const { t } = useTranslation()
  const copy = useCopy()
  const columns = useMemo<DataTableColumn<Component>[]>(() => {
    const actions = (c: Component): RowAction[] => {
      const eps = (c.endpoints ?? []).filter((e) => e.address)
      return [
        {
          key: 'copyImage',
          label: t('runs.overview.stand.copyImage'),
          icon: 'copy',
          disabled: !c.image,
          disabledReason: t('runs.overview.stand.noImage'),
          onClick: () => c.image && copy(c.image),
        },
        ...(eps.length
          ? eps.map(
              (e, i): RowAction => ({
                key: `ep:${i}`,
                group: i === 0,
                label: t('runs.overview.stand.copyEndpoint', { endpoint: endpointLabel(e) }),
                icon: 'link',
                onClick: () => copy(endpointAddr(e)),
              })
            )
          : [
              {
                key: 'ep',
                group: true,
                label: t('runs.overview.stand.copyEndpoints'),
                icon: 'link',
                disabled: true,
                disabledReason: t('runs.overview.stand.noEndpoints'),
                onClick: () => undefined,
              } satisfies RowAction,
            ]),
        {
          key: 'copyId',
          group: true,
          label: t('runs.overview.stand.copyComponentId'),
          icon: 'copy',
          onClick: () => copy(c.id),
        },
      ]
    }
    return [
      col.status<Component>({
        id: 'status',
        title: t('common.fields.status'),
        status: (c) => c.status,
      }),
      col.identity<Component>({
        id: 'id',
        header: t('runs.overview.components.id'),
        minWidth: 220,
        render: (c) => ({
          title: c.id,
          subtitle: [c.role, c.engine].filter(Boolean).join(' · '),
        }),
      }),
      col.code<Component>({
        id: 'image',
        header: t('runs.overview.components.image'),
        max: 36,
        value: (c) => c.image,
      }),
      col.text<Component>({
        id: 'machine',
        header: t('runs.overview.components.machine'),
        width: 160,
        value: (c) => c.machine,
      }),
      col.text<Component>({
        id: 'endpoints',
        header: t('runs.overview.components.endpoints'),
        minWidth: 200,
        value: (c) => (c.endpoints ?? []).map(endpointLabel).join(', ') || undefined,
      }),
      col.actions<Component>({ title: (c) => c.id, actions }),
    ]
  }, [t, copy])
  return (
    <DataTable<Component>
      columns={columns}
      data={components}
      getRowId={(c) => c.id}
      clientSort
      empty={{ message: t('runs.overview.components.empty') }}
    />
  )
}

export function MachinesTable({ machines }: { machines: Machine[] }) {
  const { t } = useTranslation()
  const copy = useCopy()
  const columns = useMemo<DataTableColumn<Machine>[]>(() => {
    const actions = (m: Machine): RowAction[] => [
      {
        key: 'copyAddress',
        label: t('runs.overview.stand.copyAddress'),
        icon: 'copy',
        disabled: !m.address,
        disabledReason: t('runs.overview.stand.noAddress'),
        onClick: () => m.address && copy(m.address),
      },
      {
        key: 'copyPublicIp',
        label: t('runs.overview.stand.copyPublicIp'),
        icon: 'copy',
        disabled: !m.public_ip,
        disabledReason: t('runs.overview.stand.noPublicIp'),
        onClick: () => m.public_ip && copy(m.public_ip),
      },
      {
        key: 'copyResource',
        label: t('runs.overview.stand.copyResource'),
        icon: 'copy',
        disabled: !m.provider_resource_id,
        disabledReason: t('runs.overview.stand.noResource'),
        onClick: () => m.provider_resource_id && copy(m.provider_resource_id),
      },
      {
        key: 'copyName',
        group: true,
        label: t('runs.overview.stand.copyMachineName'),
        icon: 'copy',
        onClick: () => copy(m.name),
      },
    ]
    return [
      col.status<Machine>({
        id: 'status',
        title: t('common.fields.status'),
        status: (m) => m.status,
      }),
      col.identity<Machine>({
        id: 'name',
        header: t('common.fields.name'),
        minWidth: 200,
        render: (m) => ({
          title: m.name,
          subtitle: [m.role, m.size].filter(Boolean).join(' · '),
        }),
      }),
      col.statusText<Machine>({
        id: 'presence',
        header: t('runs.overview.machines.presence'),
        status: (m) => m.presence,
      }),
      col.stack<Machine>({
        id: 'address',
        header: t('runs.overview.machines.address'),
        minWidth: 180,
        value: (m) => m.address,
        render: (m) => {
          const pub = m.public_ip
            ? `${t('runs.overview.machines.public')}: ${m.public_ip}`
            : undefined
          return {
            primary: m.address ?? m.public_ip,
            secondary: m.address ? pub : undefined,
            title: [m.address, pub].filter(Boolean).join('\n') || undefined,
          }
        },
      }),
      col.time<Machine>({
        id: 'heartbeat',
        header: t('runs.overview.machines.heartbeat'),
        width: 150,
        value: (m) => m.last_heartbeat_at,
      }),
      col.text<Machine>({
        id: 'resource',
        header: t('runs.overview.machines.resource'),
        width: 200,
        value: (m) => m.provider_resource_id,
      }),
      col.actions<Machine>({ title: (m) => m.name, actions }),
    ]
  }, [t, copy])
  return (
    <DataTable<Machine>
      columns={columns}
      data={machines}
      getRowId={(m) => m.name}
      clientSort
      empty={{ message: t('runs.overview.machines.empty') }}
    />
  )
}

export function SegmentsTable({ segments }: { segments: Segment[] }) {
  const { t } = useTranslation()
  const columns = useMemo<DataTableColumn<Segment>[]>(
    () => [
      col.status<Segment>({
        id: 'status',
        title: t('common.fields.status'),
        status: (s) => s.status,
      }),
      col.identity<Segment>({
        id: 'name',
        header: t('runs.overview.segments.name'),
        render: (s) => ({ title: s.name }),
      }),
      col.time<Segment>({
        id: 'started',
        header: t('common.fields.started'),
        width: 150,
        value: (s) => s.started_at,
      }),
      col.duration<Segment>({
        id: 'duration',
        header: t('common.fields.duration'),
        value: (s) => {
          if (!s.started_at) return undefined
          const end = s.finished_at ? new Date(s.finished_at).getTime() : Date.now()
          return (end - new Date(s.started_at).getTime()) / 1000
        },
      }),
    ],
    [t]
  )
  return (
    <DataTable<Segment>
      columns={columns}
      data={segments}
      getRowId={(s) => s.name}
      clientSort
      empty={{ message: t('runs.overview.segments.empty') }}
    />
  )
}
