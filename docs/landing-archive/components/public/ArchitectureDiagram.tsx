import type { IconName } from '@grafana/data'
import { useSyncExternalStore } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Diagram,
  type DiagramEdge,
  type DiagramFact,
  type DiagramFocus,
  type DiagramGroup,
} from './Diagram'

// The system as it is, zone by zone from the product down to your cloud
// (the console, running in the browser, is marked inside the product). In
// each zone the run's own parts flow left to right; under them, a row of
// what they stand on (Temporal, Kratos, S3…), linked on hover and in steps.
// `step` lights one path of a run's life; 0 shows everything.

type Spec = { id: string; icon: IconName; area: string; client?: boolean }
type Zone = {
  id: string
  icon: IconName
  dashed?: boolean
  columns: number
  areas: string
  narrow?: { columns: number; areas: string }
  nodes: Spec[]
  deps: Spec[]
}

const WIDE = '(min-width: 1800px)'
const subscribe = (cb: () => void) => {
  const m = window.matchMedia(WIDE)
  m.addEventListener('change', cb)
  return () => m.removeEventListener('change', cb)
}
const isWide = () => window.matchMedia(WIDE).matches

// One row of parts per zone: links inside a zone join neighbours only, links
// between zones cross the gap between rows — no line runs through a block.
const GROUPS: Zone[] = [
  {
    id: 'cloud',
    icon: 'cog',
    columns: 4,
    areas: `"identity console api pipes"`,
    deps: [
      { id: 'state', icon: 'database', area: '' },
      { id: 'kratos', icon: 'key-skeleton-alt', area: '' },
    ],
    nodes: [
      { id: 'console', icon: 'monitor', area: 'console', client: true },
      { id: 'api', icon: 'brackets-curly', area: 'api' },
      { id: 'identity', icon: 'shield', area: 'identity' },
      { id: 'pipes', icon: 'code-branch', area: 'pipes' },
    ],
  },
  {
    id: 'graphene',
    icon: 'sitemap',
    columns: 4,
    // The entry point sits under the server and the pipelines that call it,
    // provisioning over the network it creates.
    areas: `"crossplane worker door door"`,
    deps: [
      { id: 'temporal', icon: 'history', area: '' },
      { id: 'k8s', icon: 'apps', area: '' },
      { id: 'xp', icon: 'cloud-upload', area: '' },
      { id: 'oci', icon: 'cloud-download', area: '' },
      { id: 'telemetry', icon: 'chart-line', area: '' },
      { id: 'blobs', icon: 'folder', area: '' },
    ],
    nodes: [
      { id: 'door', icon: 'exchange-alt', area: 'door' },
      { id: 'worker', icon: 'play', area: 'worker' },
      { id: 'crossplane', icon: 'cloud', area: 'crossplane' },
    ],
  },
  {
    id: 'yours',
    icon: 'cloud',
    dashed: true,
    columns: 6,
    areas: `"network agent exporters runner proxy db"`,
    // Six blocks in a row get too narrow below full HD: two rows of three.
    narrow: { columns: 3, areas: `"network agent exporters" "runner proxy db"` },
    deps: [
      { id: 'vm', icon: 'cloud-database-tree', area: '' },
      { id: 'docker', icon: 'layer-group', area: '' },
    ],
    nodes: [
      { id: 'agent', icon: 'cube', area: 'agent' },
      { id: 'network', icon: 'sitemap', area: 'network' },
      { id: 'exporters', icon: 'tachometer-fast', area: 'exporters' },
      { id: 'runner', icon: 'bolt', area: 'runner' },
      { id: 'proxy', icon: 'arrow-random', area: 'proxy' },
      { id: 'db', icon: 'database', area: 'db' },
    ],
  },
]

const EDGES: DiagramEdge[] = [
  { from: 'console', to: 'api', label: 'consoleApi' },
  { from: 'console', to: 'identity', label: 'signIn' },
  { from: 'api', to: 'door', label: 'startRun' },
  { from: 'pipes', to: 'door', label: 'push' },
  { from: 'door', to: 'worker', label: 'workflow' },
  { from: 'worker', to: 'crossplane', label: 'machines' },
  { from: 'crossplane', to: 'network', label: 'yourKey' },
  { from: 'network', to: 'agent', label: 'cloudInit' },
  { from: 'agent', to: 'door', label: 'session' },
  { from: 'agent', to: 'exporters', label: 'scrape' },
  { from: 'runner', to: 'proxy', label: 'load' },
  { from: 'proxy', to: 'db' },
  // What each part stands on.
  { from: 'api', to: 'state', hidden: true },
  { from: 'identity', to: 'kratos', hidden: true },
  { from: 'door', to: 'temporal', hidden: true },
  { from: 'worker', to: 'temporal', hidden: true },
  { from: 'worker', to: 'k8s', hidden: true },
  { from: 'door', to: 'oci', hidden: true },
  { from: 'worker', to: 'oci', hidden: true },
  { from: 'crossplane', to: 'xp', hidden: true },
  { from: 'door', to: 'telemetry', hidden: true },
  { from: 'worker', to: 'blobs', hidden: true },
  { from: 'network', to: 'vm', hidden: true },
  { from: 'agent', to: 'docker', hidden: true },
  { from: 'db', to: 'docker', hidden: true },
]

/** The steps of a run's life the architecture screen walks through (0 = all). */
export const ARCH_STEPS = ['all', 'launch', 'pipeline', 'stand', 'run', 'observe'] as const

const FOCUS: Record<(typeof ARCH_STEPS)[number], DiagramFocus | null> = {
  all: null,
  launch: {
    nodes: ['console', 'api', 'identity', 'state', 'kratos', 'door', 'temporal'],
    edges: [
      'console-api',
      'console-identity',
      'api-door',
      'api-state',
      'identity-kratos',
      'door-temporal',
    ],
  },
  pipeline: {
    nodes: ['pipes', 'oci', 'door', 'worker', 'temporal', 'k8s'],
    edges: ['pipes-door', 'door-oci', 'worker-oci', 'door-worker', 'worker-temporal', 'worker-k8s'],
  },
  stand: {
    nodes: ['worker', 'crossplane', 'xp', 'network', 'vm', 'agent', 'door'],
    edges: [
      'worker-crossplane',
      'crossplane-xp',
      'crossplane-network',
      'network-vm',
      'network-agent',
      'agent-door',
    ],
  },
  run: {
    nodes: ['agent', 'docker', 'runner', 'proxy', 'db', 'exporters'],
    edges: ['agent-docker', 'db-docker', 'runner-proxy', 'proxy-db', 'agent-exporters'],
  },
  observe: {
    nodes: ['exporters', 'agent', 'door', 'telemetry', 'worker', 'blobs', 'api', 'console'],
    edges: [
      'agent-exporters',
      'agent-door',
      'door-telemetry',
      'worker-blobs',
      'api-door',
      'console-api',
    ],
  },
}

export function ArchitectureDiagram({ step = 0 }: { step?: number }) {
  const { t } = useTranslation()
  const k = (id: string) => `how.arch.${id}`
  const node = (n: Spec) => ({
    ...n,
    title: t(`${k(n.id)}.title`),
    body: t(`${k(n.id)}.body`),
    more: t(`${k(n.id)}.more`),
    sample: t(`${k(n.id)}.tech`),
    facts: t(`${k(n.id)}.facts`, { returnObjects: true, defaultValue: [] }) as DiagramFact[],
  })
  const wide = useSyncExternalStore(subscribe, isWide)
  const groups: DiagramGroup[] = GROUPS.map(({ narrow, ...g }) => ({
    ...g,
    ...(!wide && narrow ? narrow : {}),
    label: t(`${k(g.id)}.label`),
    nodes: g.nodes.map(node),
    deps: g.deps.map(node),
    depsLabel: t('how.arch.depsLabel'),
  }))
  const edges = EDGES.map((e) => ({
    ...e,
    label: e.label ? t(`how.arch.edges.${e.label}`) : undefined,
  }))
  return (
    <Diagram
      columns="minmax(0, 1fr)"
      groups={groups}
      edges={edges}
      focus={FOCUS[ARCH_STEPS[step] ?? 'all']}
      gap={6}
    />
  )
}
