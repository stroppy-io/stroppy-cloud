import type { Run, RunOverview, Schemas } from '@api/types'
import type { Edge, Node } from '@xyflow/react'
import { MarkerType } from '@xyflow/react'

// Deterministic topology layout (no force layout): machines are group boxes with their
// containers as cards inside; columns run left → right in traffic order
// runner → proxy → database → coordinator → other → external; the control plane (this
// server), when the resource tree shows agents, sits centered underneath.

export type RoleClass = 'runner' | 'proxy' | 'db' | 'replica' | 'coordinator' | 'monitor' | 'other'
export type EdgeKind = 'flow' | 'replication' | 'proxy' | 'coordination' | 'control'

type Machine = RunOverview['machines'][number]
type Component = RunOverview['components'][number]
type Flow = NonNullable<RunOverview['flows']>[number]
type SnapshotMachine = NonNullable<Run['snapshot']['machines']>[number]
type Tree = Schemas['ResourceTree']

export type MachineNodeData = {
  kind: 'machine'
  name: string
  role: string
  roleClass: RoleClass
  status: string
  presence: string
  size?: string
  address?: string
  publicIp?: string
  accent: string
  statusColor: string
  presenceColor: string
  componentCount: number
}
export type CardNodeData = {
  kind: 'component'
  id: string
  role: string
  roleClass: RoleClass
  engine?: string
  image?: string
  machine: string
  status: string
  accent: string
  statusColor: string
}
export type PlainNodeData = {
  kind: 'server' | 'external'
  title: string
  subtitle?: string
  accent: string
}
export type TopoNodeData = MachineNodeData | CardNodeData | PlainNodeData
export type TopoNode = Node<TopoNodeData, 'machine' | 'card' | 'plain'>

export interface TopoColors {
  role: (cls: RoleClass) => string
  status: (status: string) => string
  presence: (presence: string) => string
  edge: (kind: EdgeKind) => string
  labelText: string
  labelBg: string
  fontFamily: string
  fontSize: number
}

export interface TopoInput {
  overview: RunOverview
  snapshot?: Run['snapshot']
  tree?: Tree
  running: boolean
}

export interface TopoGraph {
  nodes: TopoNode[]
  edges: Edge[]
  roleClasses: RoleClass[]
  edgeKinds: EdgeKind[]
  statuses: string[]
}

// Geometry (canvas units).
export const CARD_W = 240
export const CARD_H = 46
const CARD_GAP = 8
const GROUP_PAD_X = 12
const GROUP_PAD_TOP = 48
const GROUP_PAD_BOTTOM = 12
export const GROUP_W = CARD_W + GROUP_PAD_X * 2
const GROUP_MIN_BODY = CARD_H
const MACHINE_GAP = 32
const COL_GAP = 170
export const PLAIN_W = 220
export const PLAIN_H = 52
const SERVER_GAP = 90

const COLUMN_ORDER: RoleClass[][] = [
  ['runner'],
  ['proxy'],
  ['db', 'replica'],
  ['coordinator'],
  ['monitor', 'other'],
]
const CLASS_RANK: Record<RoleClass, number> = {
  runner: 0,
  proxy: 1,
  db: 2,
  replica: 3,
  coordinator: 4,
  monitor: 5,
  other: 6,
}

export function roleClassOf(role: string | undefined, engine?: string): RoleClass {
  const r = `${role ?? ''} ${engine ?? ''}`.toLowerCase()
  if (/runner|stroppy|workload|client|bench/.test(r)) return 'runner'
  if (/proxy|haproxy|maxscale|pgbouncer|router|balancer/.test(r)) return 'proxy'
  if (/etcd|zookeeper|\bzk\b|consul|coordinator/.test(r)) return 'coordinator'
  if (/replica|standby|secondary|follower/.test(r)) return 'replica'
  if (/monitor|exporter|vector|grafana|victoria|prometheus|vmagent|agent/.test(r)) return 'monitor'
  if (
    /db|master|primary|leader|node|storage|database|source|instance|shard|postgres|mysql|ydb|cockroach|tidb|yugabyte|mongo|clickhouse|oracle/.test(
      r
    )
  )
    return 'db'
  return 'other'
}

const machineId = (name: string) => `m:${name}`
const cardId = (id: string) => `c:${id}`
const externalId = (key: string) => `x:${key}`
const SERVER_ID = 'server'

function groupHeight(n: number): number {
  const body = n ? n * CARD_H + (n - 1) * CARD_GAP : GROUP_MIN_BODY
  return GROUP_PAD_TOP + body + GROUP_PAD_BOTTOM
}

function pickHandles(srcX: number, tgtX: number): { sourceHandle: string; targetHandle: string } {
  if (tgtX > srcX + 1) return { sourceHandle: 'sr', targetHandle: 'tl' }
  if (tgtX < srcX - 1) return { sourceHandle: 'sl', targetHandle: 'tr' }
  return { sourceHandle: 'sr', targetHandle: 'tr' }
}

function readParams(snapshot?: Run['snapshot']) {
  const p = (snapshot?.database.params ?? {}) as Record<string, unknown>
  const num = (k: string) => (typeof p[k] === 'number' ? (p[k] as number) : undefined)
  return {
    replicas: num('replicas') ?? 0,
    ha: typeof p.ha === 'string' ? (p.ha as string) : undefined,
    galera: p.galera === true,
    kind: snapshot?.database.kind as string | undefined,
  }
}

// Agents in the resource tree = machines that talk to this server (control-plane edges).
function agentMachines(tree: Tree | undefined): Set<string> {
  const out = new Set<string>()
  const walk = (n: Tree) => {
    if (/agent/i.test(n.kind) && n.labels?.machine) out.add(n.labels.machine)
    for (const c of n.children ?? []) walk(c)
  }
  if (tree) walk(tree)
  return out
}

export function buildTopologyGraph(input: TopoInput, colors: TopoColors): TopoGraph {
  const { overview, snapshot, tree, running } = input
  const machines = overview.machines
  const components = overview.components
  const flows = overview.flows ?? []
  const nodes: TopoNode[] = []
  const edges: Edge[] = []
  if (!machines.length) return { nodes, edges, roleClasses: [], edgeKinds: [], statuses: [] }

  // ── classify ──
  const compsOf = new Map<string, Component[]>()
  for (const m of machines) compsOf.set(m.name, [])
  const orphanComps: Component[] = []
  for (const c of components) {
    const list = compsOf.get(c.machine)
    if (list) list.push(c)
    else orphanComps.push(c)
  }
  const classOfMachine = (m: Machine): RoleClass => {
    const own = compsOf.get(m.name) ?? []
    const classes = own.map((c) => roleClassOf(c.role, c.engine))
    const primary = classes.find((k) => k !== 'monitor' && k !== 'other')
    return primary ?? roleClassOf(m.role)
  }
  const machineClass = new Map(machines.map((m) => [m.name, classOfMachine(m)] as const))
  for (const list of compsOf.values())
    list.sort(
      (a, b) =>
        CLASS_RANK[roleClassOf(a.role, a.engine)] - CLASS_RANK[roleClassOf(b.role, b.engine)] ||
        a.id.localeCompare(b.id)
    )
  const sortedMachines = [...machines].sort(
    (a, b) =>
      CLASS_RANK[machineClass.get(a.name) ?? 'other'] -
        CLASS_RANK[machineClass.get(b.name) ?? 'other'] || a.name.localeCompare(b.name)
  )

  // ── columns ──
  const columns = COLUMN_ORDER.map((classes) =>
    sortedMachines.filter((m) => classes.includes(machineClass.get(m.name) ?? 'other'))
  ).filter((col) => col.length)
  const colHeight = (col: Machine[]) =>
    col.reduce((h, m) => h + groupHeight((compsOf.get(m.name) ?? []).length), 0) +
    Math.max(col.length - 1, 0) * MACHINE_GAP

  // External endpoints (flows to nothing we know) form the last column.
  const known = new Set<string>()
  for (const m of machines) known.add(m.name)
  for (const c of components) known.add(c.role)
  for (const m of machines) known.add(m.role)
  const externals = new Map<string, { title: string; subtitle?: string }>()
  const externalKey = (f: Flow) => {
    const target = f.to && !known.has(f.to) ? f.to : undefined
    const key = target ?? `${f.protocol ?? 'tcp'}:${f.port ?? ''}`
    if (!externals.has(key))
      externals.set(key, {
        title: target ?? snapshot?.database.kind ?? 'external',
        subtitle: [f.protocol, f.port].filter(Boolean).join(':') || undefined,
      })
    return key
  }
  for (const f of flows) if (!f.to || !known.has(f.to)) externalKey(f)

  const maxH = Math.max(
    ...columns.map(colHeight),
    externals.size ? externals.size * (PLAIN_H + CARD_GAP) : 0,
    1
  )

  // ── place machines ──
  const absX = new Map<string, number>() // node id → absolute x (for handle sides)
  let x = 0
  const statuses = new Set<string>()
  const roleClasses = new Set<RoleClass>()
  for (const col of columns) {
    let y = (maxH - colHeight(col)) / 2
    for (const m of col) {
      const own = compsOf.get(m.name) ?? []
      const cls = machineClass.get(m.name) ?? 'other'
      roleClasses.add(cls)
      statuses.add(m.status)
      const h = groupHeight(own.length)
      const gid = machineId(m.name)
      nodes.push({
        id: gid,
        type: 'machine',
        position: { x, y },
        draggable: false,
        style: { width: GROUP_W, height: h },
        data: {
          kind: 'machine',
          name: m.name,
          role: m.role,
          roleClass: cls,
          status: m.status,
          presence: m.presence,
          size: m.size,
          address: m.address,
          publicIp: m.public_ip,
          accent: colors.role(cls),
          statusColor: colors.status(m.status),
          presenceColor: colors.presence(m.presence),
          componentCount: own.length,
        },
      })
      absX.set(gid, x)
      own.forEach((c, i) => {
        const ccls = roleClassOf(c.role, c.engine)
        roleClasses.add(ccls)
        statuses.add(c.status)
        const cid = cardId(c.id)
        nodes.push({
          id: cid,
          type: 'card',
          parentId: gid,
          extent: 'parent',
          position: { x: GROUP_PAD_X, y: GROUP_PAD_TOP + i * (CARD_H + CARD_GAP) },
          draggable: false,
          style: { width: CARD_W, height: CARD_H },
          data: {
            kind: 'component',
            id: c.id,
            role: c.role,
            roleClass: ccls,
            engine: c.engine,
            image: c.image,
            machine: c.machine,
            status: c.status,
            accent: colors.role(ccls),
            statusColor: colors.status(c.status),
          },
        })
        absX.set(cid, x + GROUP_PAD_X)
      })
      y += h + MACHINE_GAP
    }
    x += GROUP_W + COL_GAP
  }
  // Components whose machine is not (yet) listed: a loose column of cards.
  if (orphanComps.length) {
    let y = (maxH - orphanComps.length * (CARD_H + CARD_GAP)) / 2
    for (const c of orphanComps) {
      const ccls = roleClassOf(c.role, c.engine)
      const cid = cardId(c.id)
      nodes.push({
        id: cid,
        type: 'card',
        position: { x, y },
        draggable: false,
        style: { width: CARD_W, height: CARD_H },
        data: {
          kind: 'component',
          id: c.id,
          role: c.role,
          roleClass: ccls,
          engine: c.engine,
          image: c.image,
          machine: c.machine,
          status: c.status,
          accent: colors.role(ccls),
          statusColor: colors.status(c.status),
        },
      })
      absX.set(cid, x)
      y += CARD_H + CARD_GAP
    }
    x += CARD_W + COL_GAP
  }
  if (externals.size) {
    let y = (maxH - externals.size * (PLAIN_H + CARD_GAP)) / 2
    for (const [key, ext] of externals) {
      const id = externalId(key)
      nodes.push({
        id,
        type: 'plain',
        position: { x, y },
        draggable: false,
        style: { width: PLAIN_W, height: PLAIN_H },
        data: {
          kind: 'external',
          title: ext.title,
          subtitle: ext.subtitle,
          accent: colors.role('db'),
        },
      })
      absX.set(id, x)
      y += PLAIN_H + CARD_GAP
    }
    x += PLAIN_W
  }
  const totalW = Math.max(x - COL_GAP, GROUP_W)

  // ── control plane from the resource tree ──
  const agents = agentMachines(tree)
  const agentMachineIds = [...agents].filter((n) => absX.has(machineId(n)))
  if (agentMachineIds.length) {
    nodes.push({
      id: SERVER_ID,
      type: 'plain',
      position: { x: (totalW - PLAIN_W) / 2, y: maxH + SERVER_GAP },
      draggable: false,
      style: { width: PLAIN_W, height: PLAIN_H },
      data: {
        kind: 'server',
        title: 'Stroppy Cloud',
        subtitle: 'agents',
        accent: colors.role('other'),
      },
    })
  }

  // ── edges ──
  const primaryCardOf = (m: string): string | undefined => {
    const own = compsOf.get(m) ?? []
    const main = own.find((c) => {
      const k = roleClassOf(c.role, c.engine)
      return k !== 'monitor' && k !== 'other'
    })
    return main ? cardId(main.id) : undefined
  }
  // A flow endpoint is a machine name or a role; prefer the container that plays the role.
  const resolve = (name: string | undefined): string[] => {
    if (!name) return []
    if (compsOf.has(name)) return [primaryCardOf(name) ?? machineId(name)]
    const byCompRole = components.filter((c) => c.role === name && compsOf.has(c.machine))
    if (byCompRole.length) return byCompRole.map((c) => cardId(c.id))
    const byMachineRole = machines.filter((m) => m.role === name)
    if (byMachineRole.length)
      return byMachineRole.map((m) => primaryCardOf(m.name) ?? machineId(m.name))
    return []
  }
  const seen = new Set<string>()
  const edgeKinds = new Set<EdgeKind>()
  const addEdge = (
    source: string,
    target: string,
    kind: EdgeKind,
    label?: string,
    opts: { quiet?: boolean; animated?: boolean } = {}
  ) => {
    if (source === target || (!absX.has(source) && source !== SERVER_ID)) return
    if (!absX.has(target) && target !== SERVER_ID) return
    const k = `${source}|${target}|${kind}|${label ?? ''}`
    if (seen.has(k)) return
    seen.add(k)
    edgeKinds.add(kind)
    const color = colors.edge(kind)
    const quiet = !!opts.quiet
    const handles =
      target === SERVER_ID
        ? { sourceHandle: 'sb', targetHandle: 'tt' }
        : pickHandles(absX.get(source) ?? 0, absX.get(target) ?? 0)
    edges.push({
      id: `e${edges.length}:${k}`,
      source,
      target,
      ...handles,
      type: 'smoothstep',
      pathOptions: { borderRadius: 8, offset: 24 },
      label: quiet ? undefined : label,
      animated: !quiet && !!opts.animated,
      markerEnd: { type: MarkerType.ArrowClosed, width: 12, height: 12, color },
      style: {
        stroke: color,
        strokeWidth: quiet ? 1 : 1.5,
        opacity: quiet ? 0.45 : 0.9,
        strokeDasharray: quiet ? '4 4' : undefined,
      },
      labelStyle: {
        fill: colors.labelText,
        fontSize: colors.fontSize,
        fontFamily: colors.fontFamily,
      },
      labelBgStyle: { fill: colors.labelBg, fillOpacity: 0.92 },
      labelBgPadding: [4, 2],
      labelBgBorderRadius: 3,
      labelShowBg: true,
      zIndex: quiet ? 0 : 5,
    } as Edge)
  }

  for (const f of flows) {
    const sources = resolve(f.from)
    const targets = f.to && known.has(f.to) ? resolve(f.to) : [externalId(externalKey(f))]
    const label = [f.protocol, f.port].filter(Boolean).join(' · ') || undefined
    for (const s of sources)
      for (const t of targets) addEdge(s, t, 'flow', label, { animated: running })
  }

  // Replication / proxy / coordination edges the database params imply (postgres, mysql).
  const params = readParams(snapshot)
  if (params.kind === 'postgres' || params.kind === 'mysql') {
    const byClass = (cls: RoleClass) =>
      sortedMachines
        .filter((m) => machineClass.get(m.name) === cls)
        .map((m) => primaryCardOf(m.name) ?? machineId(m.name))
    const masters = byClass('db')
    const replicas = byClass('replica')
    const proxies = byClass('proxy')
    const coordinators = byClass('coordinator')
    if (params.galera) {
      const all = [...masters, ...replicas]
      for (let i = 0; i < all.length; i++)
        for (let j = i + 1; j < all.length; j++)
          addEdge(all[i], all[j], 'replication', 'galera', { animated: running })
    } else if (params.replicas > 0 || replicas.length) {
      for (const m of masters)
        for (const r of replicas) addEdge(m, r, 'replication', 'replication', { animated: running })
    }
    for (const p of proxies)
      for (const d of [...masters, ...replicas]) addEdge(p, d, 'proxy', 'proxy')
    if (params.ha === 'patroni' || coordinators.length)
      for (const d of [...masters, ...replicas])
        for (const c of coordinators) addEdge(d, c, 'coordination', 'etcd', { quiet: true })
  }
  for (const m of agentMachineIds)
    addEdge(machineId(m), SERVER_ID, 'control', 'agent', { quiet: true })

  return {
    nodes,
    edges,
    roleClasses: [...roleClasses].sort((a, b) => CLASS_RANK[a] - CLASS_RANK[b]),
    edgeKinds: [...edgeKinds],
    statuses: [...statuses],
  }
}

// Machine plan from the snapshot (cpu / memory / disk / instance / location) by name.
export function snapshotMachine(
  snapshot: Run['snapshot'] | undefined,
  name: string
): SnapshotMachine | undefined {
  return snapshot?.machines?.find((m) => m.name === name)
}
