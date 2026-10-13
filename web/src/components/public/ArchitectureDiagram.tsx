import type { IconName } from '@grafana/data'
import { useTranslation } from 'react-i18next'
import { Diagram, type DiagramFocus, type DiagramGroup } from './Diagram'

// The system as it is: the console; Stroppy Cloud (the product server, its
// identity and store, the pipelines it ships); Graphene (the durable core
// that runs pipelines and owns resources); the user's cloud (the stand).
// `step` lights one path of a run's life; 0 shows everything.

type Spec = { id: string; icon: IconName; area: string }
const GROUPS: {
  id: string
  icon: IconName
  dashed?: boolean
  columns: number
  areas: string
  nodes: Spec[]
}[] = [
  {
    id: 'browser',
    icon: 'monitor',
    columns: 1,
    areas: `"console" "."`,
    nodes: [{ id: 'console', icon: 'monitor', area: 'console' }],
  },
  {
    id: 'cloud',
    icon: 'cog',
    columns: 1,
    areas: `"api" "identity" "state" "pipes"`,
    nodes: [
      { id: 'api', icon: 'brackets-curly', area: 'api' },
      { id: 'identity', icon: 'shield', area: 'identity' },
      { id: 'state', icon: 'database', area: 'state' },
      { id: 'pipes', icon: 'code-branch', area: 'pipes' },
    ],
  },
  {
    id: 'graphene',
    icon: 'sitemap',
    columns: 2,
    areas: `"door door" "telemetry temporal" "registry worker" ". crossplane"`,
    nodes: [
      { id: 'door', icon: 'exchange-alt', area: 'door' },
      { id: 'temporal', icon: 'history', area: 'temporal' },
      { id: 'registry', icon: 'cube', area: 'registry' },
      { id: 'worker', icon: 'play', area: 'worker' },
      { id: 'crossplane', icon: 'cloud', area: 'crossplane' },
      { id: 'telemetry', icon: 'chart-line', area: 'telemetry' },
    ],
  },
  {
    id: 'yours',
    icon: 'cloud',
    dashed: true,
    columns: 1,
    areas: `"agent" "db" "runner"`,
    nodes: [
      { id: 'agent', icon: 'cube', area: 'agent' },
      { id: 'db', icon: 'database', area: 'db' },
      { id: 'runner', icon: 'bolt', area: 'runner' },
    ],
  },
]

const EDGES: { from: string; to: string; label?: string }[] = [
  { from: 'console', to: 'api', label: 'consoleApi' },
  { from: 'console', to: 'identity', label: 'signIn' },
  { from: 'api', to: 'state' },
  { from: 'api', to: 'door', label: 'startRun' },
  { from: 'pipes', to: 'registry', label: 'push' },
  { from: 'door', to: 'temporal' },
  { from: 'temporal', to: 'worker' },
  { from: 'registry', to: 'worker', label: 'image' },
  { from: 'worker', to: 'crossplane', label: 'machines' },
  { from: 'crossplane', to: 'agent', label: 'yourKey' },
  { from: 'agent', to: 'door', label: 'session' },
  { from: 'agent', to: 'db' },
  { from: 'agent', to: 'runner' },
  { from: 'runner', to: 'db', label: 'load' },
  { from: 'door', to: 'telemetry', label: 'otlp' },
]

/** The steps of a run's life the architecture screen walks through (0 = all). */
export const ARCH_STEPS = ['all', 'launch', 'pipeline', 'stand', 'run', 'observe'] as const

const FOCUS: Record<(typeof ARCH_STEPS)[number], DiagramFocus | null> = {
  all: null,
  launch: {
    nodes: ['console', 'api', 'state', 'door', 'temporal'],
    edges: ['console-api', 'api-state', 'api-door', 'door-temporal'],
  },
  pipeline: {
    nodes: ['pipes', 'registry', 'temporal', 'worker'],
    edges: ['pipes-registry', 'registry-worker', 'temporal-worker'],
  },
  stand: {
    nodes: ['worker', 'crossplane', 'agent', 'door'],
    edges: ['worker-crossplane', 'crossplane-agent', 'agent-door'],
  },
  run: { nodes: ['agent', 'db', 'runner'], edges: ['agent-db', 'agent-runner', 'runner-db'] },
  observe: {
    nodes: ['agent', 'door', 'telemetry', 'api', 'console'],
    edges: ['agent-door', 'door-telemetry', 'api-door', 'console-api'],
  },
}

export function ArchitectureDiagram({ step = 0 }: { step?: number }) {
  const { t } = useTranslation()
  const k = (id: string) => `how.arch.${id}`
  const groups: DiagramGroup[] = GROUPS.map((g) => ({
    ...g,
    label: t(`${k(g.id)}.label`),
    nodes: g.nodes.map((n) => ({
      ...n,
      title: t(`${k(n.id)}.title`),
      body: t(`${k(n.id)}.body`),
      more: t(`${k(n.id)}.more`),
      foot: t(`${k(n.id)}.tech`),
    })),
  }))
  const edges = EDGES.map((e) => ({
    ...e,
    label: e.label ? t(`how.arch.edges.${e.label}`) : undefined,
  }))
  return (
    <Diagram
      columns="minmax(0, 0.75fr) minmax(0, 1fr) minmax(0, 2fr) minmax(0, 1fr)"
      groups={groups}
      edges={edges}
      focus={FOCUS[ARCH_STEPS[step] ?? 'all']}
      gap={10}
    />
  )
}
