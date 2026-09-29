import type { Run, RunOverview } from '@api/types'
import { AppLink } from '@app/AppLink'
import { CopyText } from '@components/CopyText'
import { KeyValueList, type KV } from '@components/KeyValueList'
import { RelativeTime } from '@components/RelativeTime'
import { StatusBadge } from '@components/StatusBadge'
import { Badge, Drawer, Icon, Stack, Text } from '@grafana/ui'
import { useTranslation } from 'react-i18next'
import { snapshotMachine } from './build-graph'

export type TopologySelection =
  | { kind: 'machine'; name: string }
  | { kind: 'component'; id: string }

function MachineDetails({
  machine,
  run,
  slug,
}: {
  machine: RunOverview['machines'][number]
  run: Run
  slug: string
}) {
  const { t } = useTranslation()
  const plan = snapshotMachine(run.snapshot, machine.name)
  const f = (k: string) => t(`runs.topology.fields.${k}`)
  const items: KV[] = [
    { label: f('name'), value: <CopyText value={machine.name} /> },
    { label: f('role'), value: machine.role },
    { label: f('status'), value: <StatusBadge status={machine.status} /> },
    { label: f('presence'), value: <StatusBadge status={machine.presence} /> },
    {
      label: f('heartbeat'),
      value: machine.last_heartbeat_at ? (
        <RelativeTime value={machine.last_heartbeat_at} refresh={5000} />
      ) : (
        '—'
      ),
    },
    {
      label: f('size'),
      value:
        (machine.size ?? plan?.size) ? (
          <Badge text={machine.size ?? plan?.size ?? ''} color="darkgrey" />
        ) : (
          '—'
        ),
    },
    { label: f('cpu'), value: plan?.cpu !== undefined ? String(plan.cpu) : '—' },
    { label: f('memory'), value: plan?.memory_gb !== undefined ? `${plan.memory_gb} GB` : '—' },
    {
      label: f('disk'),
      value:
        plan?.disk_gb !== undefined
          ? `${plan.disk_gb} GB${plan.disk_type ? ` · ${plan.disk_type}` : ''}`
          : '—',
    },
    { label: f('instance'), value: plan?.instance_type ?? '—' },
    { label: f('location'), value: plan?.location ?? '—' },
    { label: f('address'), value: machine.address ? <CopyText value={machine.address} /> : '—' },
    {
      label: f('publicIp'),
      value: machine.public_ip ? <CopyText value={machine.public_ip} /> : '—',
    },
    {
      label: f('resource'),
      value: machine.provider_resource_id ? <CopyText value={machine.provider_resource_id} /> : '—',
    },
  ]
  return (
    <Stack direction="column" gap={3}>
      <KeyValueList items={items} />
      <Stack direction="column" gap={1}>
        <AppLink
          to="/t/$slug/runs/$id/logs"
          params={{ slug, id: run.id }}
          search={{ machine: [machine.name] }}
        >
          <Icon name="file-alt" /> {t('runs.topology.actions.logsMachine')}
        </AppLink>
        <AppLink to="/t/$slug/runs/$id/metrics" params={{ slug, id: run.id }}>
          <Icon name="chart-line" /> {t('runs.topology.actions.metrics')}
        </AppLink>
      </Stack>
    </Stack>
  )
}

function ComponentDetails({
  component,
  run,
  slug,
}: {
  component: RunOverview['components'][number]
  run: Run
  slug: string
}) {
  const { t } = useTranslation()
  const f = (k: string) => t(`runs.topology.fields.${k}`)
  const eps = component.endpoints ?? []
  const items: KV[] = [
    { label: f('id'), value: <CopyText value={component.id} /> },
    { label: f('role'), value: component.role },
    { label: f('status'), value: <StatusBadge status={component.status} /> },
    { label: f('machine'), value: component.machine },
    { label: f('engine'), value: component.engine ?? '—' },
    { label: f('image'), value: component.image ? <CopyText value={component.image} /> : '—' },
    {
      label: f('endpoints'),
      value: eps.length ? (
        <Stack direction="column" gap={0.5}>
          {eps.map((e, i) => (
            <CopyText
              key={`${e.name}-${i}`}
              value={`${e.address ?? ''}${e.port ? `:${e.port}` : ''}`}
              display={`${e.name ? `${e.name} ` : ''}${e.address ?? ''}${e.port ? `:${e.port}` : ''}`}
            />
          ))}
        </Stack>
      ) : (
        '—'
      ),
    },
    { label: f('scrape'), value: component.scrape ? <CopyText value={component.scrape} /> : '—' },
  ]
  return (
    <Stack direction="column" gap={3}>
      <KeyValueList items={items} />
      <Stack direction="column" gap={1}>
        <AppLink
          to="/t/$slug/runs/$id/logs"
          params={{ slug, id: run.id }}
          search={{ container: [component.id] }}
        >
          <Icon name="file-alt" /> {t('runs.topology.actions.logsContainer')}
        </AppLink>
        <AppLink
          to="/t/$slug/runs/$id/logs"
          params={{ slug, id: run.id }}
          search={{ machine: [component.machine] }}
        >
          <Icon name="file-alt" /> {t('runs.topology.actions.logsMachine')}
        </AppLink>
      </Stack>
    </Stack>
  )
}

// Node details: a machine (plan + live state) or a container, with jumps to its logs/metrics.
export function TopologyDrawer({
  selection,
  overview,
  run,
  slug,
  onClose,
}: {
  selection: TopologySelection
  overview: RunOverview
  run: Run
  slug: string
  onClose: () => void
}) {
  const { t } = useTranslation()
  if (selection.kind === 'machine') {
    const m = overview.machines.find((x) => x.name === selection.name)
    return (
      <Drawer
        title={selection.name}
        subtitle={t('runs.topology.drawer.machine')}
        size="sm"
        onClose={onClose}
      >
        {m ? (
          <MachineDetails machine={m} run={run} slug={slug} />
        ) : (
          <Text color="secondary">{t('runs.topology.drawer.gone')}</Text>
        )}
      </Drawer>
    )
  }
  const c = overview.components.find((x) => x.id === selection.id)
  return (
    <Drawer
      title={c?.role ?? selection.id}
      subtitle={t('runs.topology.drawer.component')}
      size="sm"
      onClose={onClose}
    >
      {c ? (
        <ComponentDetails component={c} run={run} slug={slug} />
      ) : (
        <Text color="secondary">{t('runs.topology.drawer.gone')}</Text>
      )}
    </Drawer>
  )
}
