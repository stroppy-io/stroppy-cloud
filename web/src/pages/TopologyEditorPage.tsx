import {
  useState, useEffect, useCallback, useRef, useMemo, createContext, useContext, memo,
} from "react"
import { useNavigate, useParams } from "react-router-dom"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Save, Loader2, ChevronLeft, Trash2, Server, Cpu, HardDrive,
  MemoryStick, X, Settings2, Crown, Copy, KeyRound, GripVertical,
  AlertCircle, CheckCircle2, Database, Hexagon, Minus, Plus,
} from "lucide-react"
import {
  ReactFlow, ReactFlowProvider, Background, Controls, MiniMap,
  Handle, Position, useReactFlow,
  type Node, type Edge,
  useNodesState, useEdgesState,
  type NodeProps,
} from "@xyflow/react"
import { AnimatePresence, motion } from "framer-motion"
import { create } from "@bufbuild/protobuf"
import {
  Postgres_ClusterSchema, Postgres_NodeSchema, Postgres_SettingsSchema,
  Postgres_PostgresServiceSchema,
  Postgres_Settings_Version, Postgres_Settings_StorageEngine,
  Postgres_PostgresService_Role,
  type Postgres_Cluster, type Postgres_Node,
} from "@/proto/database/postgres_pb.ts"
import {
  Picodata_ClusterSchema,
  Picodata_Cluster_TemplateSchema,
  Picodata_Cluster_Template_TopologySchema,
  Picodata_SettingsSchema,
  Picodata_Settings_StorageEngine,
  type Picodata_Cluster,
} from "@/proto/database/picodata_pb.ts"
import { Database_TemplateSchema, type Database_Template } from "@/proto/database/database_pb.ts"
import { HardwareSchema, type Hardware } from "@/proto/deployment/deployment_pb.ts"
import { Target } from "@/proto/deployment/deployment_pb.ts"
import { TestSchema } from "@/proto/stroppy/test_pb.ts"

// ---------------------------------------------------------------------------
// Shared types
// ---------------------------------------------------------------------------

type DbType = "postgres" | "picodata"

type EditorCallbacksType = { selectNode: (idx: number) => void }

const EditorCallbacks = createContext<EditorCallbacksType>({ selectNode: () => {} })

// ---------------------------------------------------------------------------
// Postgres node types for ReactFlow
// ---------------------------------------------------------------------------

type PgNodeData = {
  pgNode: Postgres_Node; nodeIdx: number; role: "master" | "replica" | "etcd"; hasErrors: boolean
}

const PG_ROLE_STYLES = {
  master: { accent: "border-l-amber-400", dot: "bg-amber-400", badge: "bg-amber-500/15 text-amber-300", label: "Master" },
  replica: { accent: "border-l-sky-400", dot: "bg-sky-400", badge: "bg-sky-500/15 text-sky-300", label: "Replica" },
  etcd: { accent: "border-l-emerald-400", dot: "bg-emerald-400", badge: "bg-emerald-500/15 text-emerald-300", label: "Etcd" },
} as const

function PgNodeComponent({ data, selected }: NodeProps<Node<PgNodeData>>) {
  const { pgNode, nodeIdx, role, hasErrors } = data
  const { selectNode } = useContext(EditorCallbacks)
  const s = PG_ROLE_STYLES[role]
  const services: string[] = []
  if (pgNode.etcd && role !== "etcd") services.push("etcd")
  if (pgNode.pgbouncer) services.push("pgb")
  if (pgNode.monitoring) services.push("mon")

  return (
    <>
      <Handle type="target" position={Position.Top} className="!w-2.5 !h-1 !rounded-sm !bg-muted-foreground/30 !border-none !min-h-0" />
      <div onClick={() => selectNode(nodeIdx)} className={`cursor-pointer select-none border-l-[3px] ${s.accent} px-3 py-2.5 min-w-[170px] rounded-r-lg rounded-l-sm bg-card border border-border transition-all ${selected ? "border-primary/60 shadow-[0_0_16px_var(--color-primary)/0.12]" : "hover:border-muted-foreground/40"} ${hasErrors ? "ring-1 ring-red-500/50" : ""}`}>
        <div className="flex items-center gap-2 mb-1.5">
          <div className={`w-2 h-2 rounded-full ${s.dot}`} />
          <span className="font-mono text-xs font-semibold text-foreground tracking-tight">{pgNode.name}</span>
        </div>
        <span className={`text-[10px] font-mono px-1.5 py-0.5 rounded ${s.badge}`}>{s.label}</span>
        <div className="flex gap-2.5 mt-1.5 text-[10px] text-muted-foreground font-mono tabular-nums">
          <span>{pgNode.hardware?.cores ?? 0}C</span>
          <span>{pgNode.hardware?.memory ?? 0}G</span>
          <span>{pgNode.hardware?.disk ?? 0}GB</span>
        </div>
        {services.length > 0 && (
          <div className="flex gap-1 mt-1.5">
            {services.map(sv => <span key={sv} className="text-[9px] px-1.5 py-0.5 rounded bg-muted/80 text-muted-foreground font-mono">{sv}</span>)}
          </div>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} className="!w-2.5 !h-1 !rounded-sm !bg-muted-foreground/30 !border-none !min-h-0" />
    </>
  )
}

// ---------------------------------------------------------------------------
// Picodata node type for ReactFlow
// ---------------------------------------------------------------------------

type PicoNodeData = {
  index: number; nodesCount: number; hw: Hardware; monitor: boolean
}

function PicoNodeComponent({ data, selected }: NodeProps<Node<PicoNodeData>>) {
  const { index, hw, monitor } = data
  return (
    <>
      <Handle type="target" position={Position.Top} className="!w-2.5 !h-1 !rounded-sm !bg-muted-foreground/30 !border-none !min-h-0" />
      <div className={`select-none border-l-[3px] border-l-violet-400 px-3 py-2.5 min-w-[170px] rounded-r-lg rounded-l-sm bg-card border border-border transition-all ${selected ? "border-primary/60" : "hover:border-muted-foreground/40"}`}>
        <div className="flex items-center gap-2 mb-1.5">
          <div className="w-2 h-2 rounded-full bg-violet-400" />
          <span className="font-mono text-xs font-semibold text-foreground tracking-tight">picodata-{index + 1}</span>
        </div>
        <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-violet-500/15 text-violet-300">Node</span>
        <div className="flex gap-2.5 mt-1.5 text-[10px] text-muted-foreground font-mono tabular-nums">
          <span>{hw?.cores ?? 0}C</span>
          <span>{hw?.memory ?? 0}G</span>
          <span>{hw?.disk ?? 0}GB</span>
        </div>
        {monitor && <span className="text-[9px] mt-1.5 inline-block px-1.5 py-0.5 rounded bg-muted/80 text-muted-foreground font-mono">mon</span>}
      </div>
      <Handle type="source" position={Position.Bottom} className="!w-2.5 !h-1 !rounded-sm !bg-muted-foreground/30 !border-none !min-h-0" />
    </>
  )
}

const nodeTypes = {
  pgNode: memo(PgNodeComponent),
  picoNode: memo(PicoNodeComponent),
}

// ---------------------------------------------------------------------------
// Postgres helpers
// ---------------------------------------------------------------------------

function getRole(n: Postgres_Node): "master" | "replica" | "etcd" {
  if (!n.postgres) return "etcd"
  if (n.postgres.role === Postgres_PostgresService_Role.MASTER) return "master"
  return "replica"
}

function generateNodeName(role: "master" | "replica" | "etcd", existing: Postgres_Node[]): string {
  const count = existing.filter(n => n.name.startsWith(role)).length
  if (role === "master" && count === 0) return "master"
  return `${role}-${count + 1}`
}

function createPgNode(role: "master" | "replica" | "etcd"): Postgres_Node {
  if (role === "etcd") {
    return create(Postgres_NodeSchema, {
      name: "",
      hardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 20 }),
      etcd: {},
    })
  }
  return create(Postgres_NodeSchema, {
    name: "",
    hardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 50 }),
    postgres: create(Postgres_PostgresServiceSchema, {
      role: role === "master" ? Postgres_PostgresService_Role.MASTER : Postgres_PostgresService_Role.REPLICA,
    }),
  })
}

const FATAL_PATTERNS = [/must have exactly 1 master/i, /duplicate node name/i]
function isFatalError(msg: string): boolean { return FATAL_PATTERNS.some(p => p.test(msg)) }

function buildPgTemplate(pgNodes: Postgres_Node[], pgVersion: number, storageEngine: number): Database_Template {
  const defaults = create(Postgres_SettingsSchema, { version: pgVersion, storageEngine })
  const cluster = create(Postgres_ClusterSchema, { defaults, nodes: pgNodes })
  return create(Database_TemplateSchema, { template: { case: "postgresCluster", value: cluster } })
}

function computePgEdges(pgNodes: Postgres_Node[]): Edge[] {
  const master = pgNodes.find(n => n.postgres?.role === Postgres_PostgresService_Role.MASTER)
  if (!master) return []
  return pgNodes
    .filter(n => n.postgres?.role === Postgres_PostgresService_Role.REPLICA)
    .map(rep => ({
      id: `${master.name}->${rep.name}`, source: master.name, target: rep.name,
      animated: true,
      style: { stroke: "oklch(0.7 0.15 220)", strokeWidth: 1.5, opacity: 0.6 },
      label: "replication",
      labelStyle: { fontSize: 9, fill: "oklch(0.55 0 0)", fontFamily: "var(--font-mono, monospace)" },
    }))
}

function pgDefaultPosition(n: Postgres_Node, idx: number): { x: number; y: number } {
  const role = getRole(n)
  return { x: idx * 240, y: role === "master" ? 50 : role === "replica" ? 250 : 450 }
}

// ---------------------------------------------------------------------------
// Picodata helpers
// ---------------------------------------------------------------------------

function buildPicoTemplate(nodesCount: number, hw: Hardware, version: string, storageEngine: number, monitor: boolean): Database_Template {
  const settings = create(Picodata_SettingsSchema, { version, storageEngine })
  const topology = create(Picodata_Cluster_Template_TopologySchema, {
    settings, nodeHardware: hw, nodesCount, monitor,
  })
  const tmpl = create(Picodata_Cluster_TemplateSchema, { topology })
  const cluster = create(Picodata_ClusterSchema, { template: tmpl })
  return create(Database_TemplateSchema, { template: { case: "picodataCluster", value: cluster } })
}

function picoFlowNodes(count: number, hw: Hardware, monitor: boolean): Node[] {
  return Array.from({ length: count }, (_, i) => ({
    id: `pico-${i}`,
    type: "picoNode" as const,
    position: { x: i * 220, y: 100 },
    data: { index: i, nodesCount: count, hw, monitor } satisfies PicoNodeData,
  }))
}

function picoFlowEdges(count: number): Edge[] {
  if (count < 2) return []
  const edges: Edge[] = []
  for (let i = 0; i < count; i++) {
    const next = (i + 1) % count
    edges.push({
      id: `pico-${i}->pico-${next}`,
      source: `pico-${i}`, target: `pico-${next}`,
      animated: true,
      style: { stroke: "oklch(0.7 0.13 290)", strokeWidth: 1.5, opacity: 0.5 },
    })
  }
  return edges
}

// ---------------------------------------------------------------------------
// Palette (postgres only)
// ---------------------------------------------------------------------------

const PG_PALETTE_ITEMS = [
  { role: "master" as const, icon: Crown, label: "Master", desc: "Primary write node" },
  { role: "replica" as const, icon: Copy, label: "Replica", desc: "Read-only standby" },
  { role: "etcd" as const, icon: KeyRound, label: "Etcd", desc: "Consensus service" },
]

function PgPalette() {
  return (
    <div className="absolute top-16 left-4 z-10 pointer-events-auto">
      <div className="bg-card/90 backdrop-blur-md border rounded-xl shadow-xl p-2 w-[156px] space-y-1">
        <div className="px-2 py-1">
          <span className="text-[9px] font-mono uppercase tracking-widest text-muted-foreground">Components</span>
        </div>
        {PG_PALETTE_ITEMS.map(item => {
          const st = PG_ROLE_STYLES[item.role]
          return (
            <div key={item.role} draggable onDragStart={e => { e.dataTransfer.setData("application/topology-node-role", item.role); e.dataTransfer.effectAllowed = "move" }}
              className="flex items-center gap-2.5 px-2.5 py-2 rounded-lg border border-transparent cursor-grab active:cursor-grabbing select-none transition-all hover:bg-muted/60 hover:border-border active:scale-95">
              <div className={`w-5 h-5 rounded flex items-center justify-center ${st.badge}`}><item.icon className="w-3 h-3" /></div>
              <div className="min-w-0">
                <div className="text-xs font-medium text-foreground">{item.label}</div>
                <div className="text-[9px] text-muted-foreground truncate">{item.desc}</div>
              </div>
              <GripVertical className="w-3 h-3 text-muted-foreground/40 ml-auto shrink-0" />
            </div>
          )
        })}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Picodata controls (left panel)
// ---------------------------------------------------------------------------

function PicoPalette({
  nodesCount, setNodesCount, hw, setHw, monitor, setMonitor,
}: {
  nodesCount: number; setNodesCount: (n: number) => void
  hw: Hardware; setHw: (h: Hardware) => void
  monitor: boolean; setMonitor: (v: boolean) => void
}) {
  return (
    <div className="absolute top-16 left-4 z-10 pointer-events-auto">
      <div className="bg-card/90 backdrop-blur-md border rounded-xl shadow-xl p-3 w-[200px] space-y-3">
        <span className="text-[9px] font-mono uppercase tracking-widest text-muted-foreground">Cluster Topology</span>

        <div className="space-y-1.5">
          <Label className="text-[10px] font-mono text-muted-foreground">Nodes</Label>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="icon" className="h-7 w-7" onClick={() => nodesCount > 1 && setNodesCount(nodesCount - 1)}><Minus className="w-3 h-3" /></Button>
            <span className="font-mono text-sm font-semibold w-6 text-center">{nodesCount}</span>
            <Button variant="outline" size="icon" className="h-7 w-7" onClick={() => setNodesCount(nodesCount + 1)}><Plus className="w-3 h-3" /></Button>
          </div>
        </div>

        <div className="space-y-1.5">
          <Label className="text-[10px] font-mono text-muted-foreground">Hardware per node</Label>
          <div className="grid grid-cols-3 gap-1.5">
            <div>
              <span className="text-[9px] font-mono text-muted-foreground/60">CPU</span>
              <Input type="number" min={1} className="h-7 text-xs font-mono" value={hw.cores} onChange={e => setHw({ ...hw, cores: parseInt(e.target.value) || 1 })} />
            </div>
            <div>
              <span className="text-[9px] font-mono text-muted-foreground/60">RAM</span>
              <Input type="number" min={1} className="h-7 text-xs font-mono" value={hw.memory} onChange={e => setHw({ ...hw, memory: parseInt(e.target.value) || 1 })} />
            </div>
            <div>
              <span className="text-[9px] font-mono text-muted-foreground/60">Disk</span>
              <Input type="number" min={1} className="h-7 text-xs font-mono" value={hw.disk} onChange={e => setHw({ ...hw, disk: parseInt(e.target.value) || 10 })} />
            </div>
          </div>
        </div>

        <ServiceToggle label="Monitoring" description="HTTP metrics" checked={monitor} onChange={setMonitor} />
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Validation bar & notification
// ---------------------------------------------------------------------------

type ValidationState = "idle" | "validating" | "valid" | "invalid"

function ValidationBar({ state, errors, nodeCount }: { state: ValidationState; errors: string[]; nodeCount: number }) {
  return (
    <div className="absolute bottom-6 left-1/2 -translate-x-1/2 z-10 pointer-events-auto">
      <div className="bg-card/90 backdrop-blur-md rounded-xl border shadow-xl px-4 py-2 flex items-center gap-3 min-w-[280px]">
        {state === "validating" && <><Loader2 className="w-3.5 h-3.5 animate-spin text-amber-400" /><span className="text-xs text-muted-foreground font-mono">Validating...</span></>}
        {state === "valid" && <><CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" /><span className="text-xs text-emerald-400 font-mono">Valid &middot; {nodeCount} node{nodeCount !== 1 ? "s" : ""}</span></>}
        {state === "invalid" && <><AlertCircle className="w-3.5 h-3.5 text-red-400" /><span className="text-xs text-red-400 font-mono truncate max-w-[360px]">{errors[0] ?? "Invalid topology"}</span></>}
        {state === "idle" && <><div className="w-2 h-2 rounded-full bg-muted-foreground/30" /><span className="text-xs text-muted-foreground font-mono">{nodeCount} node{nodeCount !== 1 ? "s" : ""}</span></>}
      </div>
    </div>
  )
}

function Notification({ message, onDone }: { message: string; onDone: () => void }) {
  useEffect(() => { const t = setTimeout(onDone, 3500); return () => clearTimeout(t) }, [onDone])
  return (
    <motion.div initial={{ y: -40, opacity: 0 }} animate={{ y: 0, opacity: 1 }} exit={{ y: -40, opacity: 0 }}
      className="absolute top-16 left-1/2 -translate-x-1/2 z-50 pointer-events-auto">
      <div className="bg-red-950/90 backdrop-blur-md border border-red-500/30 rounded-lg shadow-xl px-4 py-2.5 flex items-center gap-2 max-w-[480px]">
        <AlertCircle className="w-4 h-4 text-red-400 shrink-0" />
        <span className="text-xs text-red-200 font-mono">{message}</span>
      </div>
    </motion.div>
  )
}

// ---------------------------------------------------------------------------
// Main export
// ---------------------------------------------------------------------------

export function TopologyEditorPage() {
  return <ReactFlowProvider><TopologyEditorInner /></ReactFlowProvider>
}

// ---------------------------------------------------------------------------
// Main editor
// ---------------------------------------------------------------------------

function TopologyEditorInner() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { screenToFlowPosition } = useReactFlow()
  const isEdit = Boolean(id)

  // --- common state ---
  const [dbType, setDbType] = useState<DbType>("postgres")
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [saving, setSaving] = useState(false)
  const [loading, setLoading] = useState(isEdit)
  const [selectedNodeIdx, setSelectedNodeIdx] = useState<number | null>(null)
  const [showSettings, setShowSettings] = useState(false)
  const [validationState, setValidationState] = useState<ValidationState>("idle")
  const [validationErrors, setValidationErrors] = useState<string[]>([])
  const [notification, setNotification] = useState<string | null>(null)
  const skipValidation = useRef(false)

  // --- postgres state ---
  const [pgVersion, setPgVersion] = useState(Postgres_Settings_Version.VERSION_17)
  const [pgStorageEngine, setPgStorageEngine] = useState(Postgres_Settings_StorageEngine.HEAP)
  const [pgNodes, setPgNodes] = useState<Postgres_Node[]>(() => [
    create(Postgres_NodeSchema, { name: "master", hardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 50 }), postgres: create(Postgres_PostgresServiceSchema, { role: Postgres_PostgresService_Role.MASTER }) }),
    create(Postgres_NodeSchema, { name: "replica-1", hardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 50 }), postgres: create(Postgres_PostgresServiceSchema, { role: Postgres_PostgresService_Role.REPLICA }) }),
  ])
  const pgValidStateRef = useRef<Postgres_Node[] | null>(null)
  const pendingDrop = useRef<{ name: string; x: number; y: number } | null>(null)

  // --- picodata state ---
  const [picoNodesCount, setPicoNodesCount] = useState(3)
  const [picoHw, setPicoHw] = useState<Hardware>(() => create(HardwareSchema, { cores: 4, memory: 8, disk: 100 }))
  const [picoVersion, setPicoVersion] = useState("latest")
  const [picoStorageEngine, setPicoStorageEngine] = useState(Picodata_Settings_StorageEngine.MEMTX)
  const [picoMonitor, setPicoMonitor] = useState(true)

  // --- ReactFlow ---
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])

  const callbacks = useMemo<EditorCallbacksType>(() => ({
    selectNode: (idx: number) => { setSelectedNodeIdx(idx); setShowSettings(false) },
  }), [])

  // --- derived ---
  const nodeCount = dbType === "postgres" ? pgNodes.length : picoNodesCount

  // ======================= LOAD EXISTING TEMPLATE ==========================

  useEffect(() => {
    if (!id) return
    api.getTopologyTemplate({ templateId: id }).then(res => {
      const t = res.topologyTemplate
      if (!t) return
      setName(t.name)
      setDescription(t.description ?? "")

      if (t.template?.template.case === "postgresCluster") {
        setDbType("postgres")
        const cluster = t.template.template.value as Postgres_Cluster
        if (cluster.defaults) { setPgVersion(cluster.defaults.version); setPgStorageEngine(cluster.defaults.storageEngine) }
        if (cluster.nodes.length > 0) { setPgNodes(cluster.nodes); pgValidStateRef.current = cluster.nodes }
      } else if (t.template?.template.case === "picodataCluster") {
        setDbType("picodata")
        const cluster = t.template.template.value as Picodata_Cluster
        const topo = cluster.template?.topology
        if (topo) {
          setPicoNodesCount(topo.nodesCount || 1)
          if (topo.nodeHardware) setPicoHw(topo.nodeHardware)
          if (topo.settings) {
            setPicoVersion(topo.settings.version || "latest")
            setPicoStorageEngine(topo.settings.storageEngine)
          }
          setPicoMonitor(topo.monitor)
        }
      }
      setLoading(false)
    }).catch(() => { setLoading(false); navigate("/topologies") })
  }, [id, navigate])

  // ======================= BUILD GRAPH =====================================

  // Postgres graph
  useEffect(() => {
    if (dbType !== "postgres") return
    setNodes(prev => {
      const posMap = new Map(prev.map(n => [n.id, n.position]))
      return pgNodes.map((pgNode, idx) => {
        const nodeId = pgNode.name
        const drop = pendingDrop.current
        const existingPos = posMap.get(nodeId)
        const dropPos = drop?.name === nodeId ? { x: drop.x, y: drop.y } : null
        if (dropPos) pendingDrop.current = null
        return {
          id: nodeId, type: "pgNode",
          position: existingPos ?? dropPos ?? pgDefaultPosition(pgNode, idx),
          data: { pgNode, nodeIdx: idx, role: getRole(pgNode), hasErrors: false } satisfies PgNodeData,
        }
      })
    })
    setEdges(computePgEdges(pgNodes))
  }, [dbType, pgNodes, setNodes, setEdges])

  // Picodata graph
  useEffect(() => {
    if (dbType !== "picodata") return
    setNodes(picoFlowNodes(picoNodesCount, picoHw, picoMonitor))
    setEdges(picoFlowEdges(picoNodesCount))
  }, [dbType, picoNodesCount, picoHw, picoMonitor, setNodes, setEdges])

  // ======================= BUILD TEMPLATE ==================================

  const currentTemplate = useCallback((): Database_Template => {
    if (dbType === "picodata") {
      return buildPicoTemplate(picoNodesCount, picoHw, picoVersion, picoStorageEngine, picoMonitor)
    }
    return buildPgTemplate(pgNodes, pgVersion, pgStorageEngine)
  }, [dbType, pgNodes, pgVersion, pgStorageEngine, picoNodesCount, picoHw, picoVersion, picoStorageEngine, picoMonitor])

  // ======================= VALIDATION ======================================

  const validationDeps = dbType === "postgres"
    ? [pgNodes, pgVersion, pgStorageEngine]
    : [picoNodesCount, picoHw, picoVersion, picoStorageEngine, picoMonitor]

  useEffect(() => {
    if (skipValidation.current) { skipValidation.current = false; return }
    setValidationState("validating")

    const timer = setTimeout(async () => {
      try {
        const template = currentTemplate()
        const test = create(TestSchema, {
          name: "__validate__",
          databaseRef: { case: "databaseTemplate", value: template },
        })
        const res = await api.validateTopology({ test, target: Target.DOCKER })

        if (res.valid) {
          if (dbType === "postgres") pgValidStateRef.current = [...pgNodes]
          setValidationState("valid")
          setValidationErrors([])
        } else {
          const messages = res.issues.map(i => i.message)
          const shouldRollback = dbType === "postgres" && pgValidStateRef.current && messages.some(isFatalError)
          if (shouldRollback) {
            skipValidation.current = true
            setPgNodes(pgValidStateRef.current!)
            setNotification(messages[0] ?? "Invalid topology — rolled back")
            setValidationState("valid")
          } else {
            setValidationErrors(messages)
            setValidationState("invalid")
          }
        }
      } catch {
        setValidationState("idle")
      }
    }, 400)

    return () => clearTimeout(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dbType, ...validationDeps])

  // ======================= DND (postgres) ==================================

  const onDragOver = useCallback((e: React.DragEvent) => { e.preventDefault(); e.dataTransfer.dropEffect = "move" }, [])

  const onDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    if (dbType !== "postgres") return
    const role = e.dataTransfer.getData("application/topology-node-role") as "master" | "replica" | "etcd"
    if (!role) return
    const position = screenToFlowPosition({ x: e.clientX, y: e.clientY })
    const node = createPgNode(role)
    const nodeName = generateNodeName(role, pgNodes)
    node.name = nodeName
    pendingDrop.current = { name: nodeName, x: position.x, y: position.y }
    setPgNodes(prev => [...prev, node])
  }, [dbType, screenToFlowPosition, pgNodes])

  // ======================= NODE OPS (postgres) =============================

  const removeNode = useCallback((idx: number) => {
    if (pgNodes.length <= 1) return
    if (selectedNodeIdx === idx) setSelectedNodeIdx(null)
    else if (selectedNodeIdx !== null && selectedNodeIdx > idx) setSelectedNodeIdx(prev => prev! - 1)
    setPgNodes(prev => prev.filter((_, i) => i !== idx))
  }, [pgNodes.length, selectedNodeIdx])

  const updateNode = useCallback((idx: number, updates: Partial<Postgres_Node>) => {
    setPgNodes(prev => prev.map((n, i) => i === idx ? { ...n, ...updates } as Postgres_Node : n))
  }, [])

  // ======================= SAVE ============================================

  const handleSave = async () => {
    if (!name.trim()) { setNotification("Name is required"); return }
    setSaving(true)
    try {
      const template = currentTemplate()
      if (isEdit && id) {
        await api.updateTopologyTemplate({ templateId: id, name, description: description || undefined, template })
      } else {
        await api.createTopologyTemplate({ name, description: description || undefined, template })
      }
      navigate("/topologies")
    } catch (err) {
      setNotification(err instanceof Error ? err.message : "Save failed")
    }
    setSaving(false)
  }

  // ======================= RENDER ==========================================

  const selectedNode = dbType === "postgres" && selectedNodeIdx !== null ? pgNodes[selectedNodeIdx] : null
  const panelOpen = selectedNodeIdx !== null || showSettings

  if (loading) return <div className="flex items-center justify-center h-64"><Loader2 className="w-5 h-5 animate-spin text-muted-foreground" /></div>

  return (
    <EditorCallbacks.Provider value={callbacks}>
      <div className="-m-6 h-[calc(100vh-3.5rem)] relative overflow-hidden">
        {/* Canvas */}
        <div className="absolute inset-0">
          <ReactFlow
            nodes={nodes} edges={edges}
            onNodesChange={onNodesChange} onEdgesChange={onEdgesChange}
            onPaneClick={() => { setSelectedNodeIdx(null); setShowSettings(false) }}
            onDragOver={onDragOver} onDrop={onDrop}
            nodeTypes={nodeTypes} fitView fitViewOptions={{ padding: 0.4 }}
            proOptions={{ hideAttribution: true }}
            minZoom={0.2} maxZoom={2.5} deleteKeyCode={null}
            nodesDraggable nodesConnectable={false}
          >
            <Background gap={24} size={1} className="opacity-20" />
            <Controls showInteractive={false} className="!bg-card/90 !backdrop-blur-md !border-border !rounded-xl !shadow-xl" />
            <MiniMap className="!bg-card/90 !backdrop-blur-md !border-border !rounded-xl" maskColor="oklch(0.14 0 0 / 0.8)" nodeColor={dbType === "postgres" ? "oklch(0.7 0.15 220)" : "oklch(0.7 0.13 290)"} />
          </ReactFlow>
        </div>

        {/* Palette / Controls (left) */}
        {dbType === "postgres" ? <PgPalette /> : (
          <PicoPalette
            nodesCount={picoNodesCount} setNodesCount={setPicoNodesCount}
            hw={picoHw} setHw={setPicoHw}
            monitor={picoMonitor} setMonitor={setPicoMonitor}
          />
        )}

        {/* Top toolbar */}
        <div className="absolute top-4 left-4 right-4 z-10 pointer-events-none">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2 pointer-events-auto">
              <Button variant="outline" size="icon" className="h-8 w-8 bg-card/90 backdrop-blur-md shadow-xl" onClick={() => navigate("/topologies")}>
                <ChevronLeft className="w-4 h-4" />
              </Button>

              {/* DB type selector */}
              {!isEdit && (
                <div className="bg-card/90 backdrop-blur-md rounded-xl border shadow-xl flex overflow-hidden">
                  <button onClick={() => setDbType("postgres")} className={`px-3 py-1.5 text-xs font-mono flex items-center gap-1.5 transition-colors ${dbType === "postgres" ? "bg-primary/15 text-primary" : "text-muted-foreground hover:text-foreground"}`}>
                    <Database className="w-3 h-3" /> PostgreSQL
                  </button>
                  <button onClick={() => setDbType("picodata")} className={`px-3 py-1.5 text-xs font-mono flex items-center gap-1.5 transition-colors ${dbType === "picodata" ? "bg-violet-500/15 text-violet-300" : "text-muted-foreground hover:text-foreground"}`}>
                    <Hexagon className="w-3 h-3" /> Picodata
                  </button>
                </div>
              )}

              <div className="bg-card/90 backdrop-blur-md rounded-xl border shadow-xl px-3 py-1.5 flex items-center gap-2">
                <Input className="h-7 w-44 bg-transparent border-none text-sm font-semibold font-mono placeholder:text-muted-foreground/40 focus-visible:ring-0 px-0" value={name} onChange={e => setName(e.target.value)} placeholder="template-name" />
                <span className="text-muted-foreground/20">|</span>
                <Input className="h-7 w-40 bg-transparent border-none text-xs text-muted-foreground placeholder:text-muted-foreground/25 focus-visible:ring-0 px-0" value={description} onChange={e => setDescription(e.target.value)} placeholder="Description..." />
              </div>
            </div>

            <div className="flex items-center gap-2 pointer-events-auto">
              <Button variant="outline" size="sm" className="h-8 bg-card/90 backdrop-blur-md shadow-xl text-xs font-mono"
                onClick={() => { setShowSettings(!showSettings); setSelectedNodeIdx(null) }}>
                <Settings2 className="w-3.5 h-3.5 mr-1.5" /> Settings
              </Button>
              <Button onClick={handleSave} disabled={saving} size="sm" className="h-8 shadow-xl font-mono">
                {saving ? <Loader2 className="w-3.5 h-3.5 mr-1.5 animate-spin" /> : <Save className="w-3.5 h-3.5 mr-1.5" />}
                {isEdit ? "Update" : "Save"}
              </Button>
            </div>
          </div>
        </div>

        {/* Validation bar */}
        <ValidationBar state={validationState} errors={validationErrors} nodeCount={nodeCount} />

        {/* Notification */}
        <AnimatePresence>
          {notification && <Notification message={notification} onDone={() => setNotification(null)} />}
        </AnimatePresence>

        {/* Right panel */}
        <AnimatePresence>
          {panelOpen && (
            <motion.div initial={{ x: 340, opacity: 0 }} animate={{ x: 0, opacity: 1 }} exit={{ x: 340, opacity: 0 }}
              transition={{ type: "spring", damping: 30, stiffness: 350 }}
              className="absolute top-4 right-4 bottom-4 w-80 z-10 pointer-events-auto">
              <div className="h-full bg-card/95 backdrop-blur-md border rounded-xl shadow-2xl flex flex-col overflow-hidden">
                {showSettings ? (
                  dbType === "postgres" ? (
                    <PgSettingsPanel pgVersion={pgVersion} setPgVersion={setPgVersion} storageEngine={pgStorageEngine} setStorageEngine={setPgStorageEngine} onClose={() => setShowSettings(false)} />
                  ) : (
                    <PicoSettingsPanel version={picoVersion} setVersion={setPicoVersion} storageEngine={picoStorageEngine} setStorageEngine={setPicoStorageEngine} onClose={() => setShowSettings(false)} />
                  )
                ) : selectedNode ? (
                  <PgNodePanel node={selectedNode} idx={selectedNodeIdx!} canDelete={pgNodes.length > 1} onUpdate={updateNode} onDelete={removeNode} onClose={() => setSelectedNodeIdx(null)} />
                ) : null}
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </EditorCallbacks.Provider>
  )
}

// ---------------------------------------------------------------------------
// Postgres settings panel
// ---------------------------------------------------------------------------

function PgSettingsPanel({ pgVersion, setPgVersion, storageEngine, setStorageEngine, onClose }: {
  pgVersion: number; setPgVersion: (v: number) => void; storageEngine: number; setStorageEngine: (v: number) => void; onClose: () => void
}) {
  return (
    <>
      <div className="flex items-center justify-between px-4 py-3 border-b">
        <div className="flex items-center gap-2"><Settings2 className="w-4 h-4 text-primary" /><span className="font-medium text-sm">PostgreSQL Settings</span></div>
        <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClose}><X className="w-3.5 h-3.5" /></Button>
      </div>
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        <div className="space-y-1.5">
          <Label className="text-[10px] uppercase tracking-wider text-muted-foreground font-mono">Version</Label>
          <select className="w-full h-8 rounded-lg border border-input bg-background text-foreground px-3 text-sm font-mono" value={pgVersion} onChange={e => setPgVersion(Number(e.target.value))}>
            <option value={Postgres_Settings_Version.VERSION_17}>PostgreSQL 17</option>
            <option value={Postgres_Settings_Version.VERSION_16}>PostgreSQL 16</option>
            <option value={Postgres_Settings_Version.VERSION_18}>PostgreSQL 18</option>
          </select>
        </div>
        <div className="space-y-1.5">
          <Label className="text-[10px] uppercase tracking-wider text-muted-foreground font-mono">Storage Engine</Label>
          <select className="w-full h-8 rounded-lg border border-input bg-background text-foreground px-3 text-sm font-mono" value={storageEngine} onChange={e => setStorageEngine(Number(e.target.value))}>
            <option value={Postgres_Settings_StorageEngine.HEAP}>Heap (default)</option>
            <option value={Postgres_Settings_StorageEngine.ORIOLEDB}>OrioleDB</option>
          </select>
        </div>
      </div>
    </>
  )
}

// ---------------------------------------------------------------------------
// Picodata settings panel
// ---------------------------------------------------------------------------

function PicoSettingsPanel({ version, setVersion, storageEngine, setStorageEngine, onClose }: {
  version: string; setVersion: (v: string) => void; storageEngine: number; setStorageEngine: (v: number) => void; onClose: () => void
}) {
  return (
    <>
      <div className="flex items-center justify-between px-4 py-3 border-b">
        <div className="flex items-center gap-2"><Settings2 className="w-4 h-4 text-violet-400" /><span className="font-medium text-sm">Picodata Settings</span></div>
        <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClose}><X className="w-3.5 h-3.5" /></Button>
      </div>
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        <div className="space-y-1.5">
          <Label className="text-[10px] uppercase tracking-wider text-muted-foreground font-mono">Version</Label>
          <Input className="h-8 text-sm font-mono" value={version} onChange={e => setVersion(e.target.value)} placeholder="latest" />
        </div>
        <div className="space-y-1.5">
          <Label className="text-[10px] uppercase tracking-wider text-muted-foreground font-mono">Storage Engine</Label>
          <select className="w-full h-8 rounded-lg border border-input bg-background text-foreground px-3 text-sm font-mono" value={storageEngine} onChange={e => setStorageEngine(Number(e.target.value))}>
            <option value={Picodata_Settings_StorageEngine.MEMTX}>Memtx (in-memory)</option>
            <option value={Picodata_Settings_StorageEngine.VINYL}>Vinyl (disk-based)</option>
          </select>
        </div>
      </div>
    </>
  )
}

// ---------------------------------------------------------------------------
// Postgres node panel
// ---------------------------------------------------------------------------

function PgNodePanel({ node, idx, canDelete, onUpdate, onDelete, onClose }: {
  node: Postgres_Node; idx: number; canDelete: boolean
  onUpdate: (idx: number, updates: Partial<Postgres_Node>) => void
  onDelete: (idx: number) => void; onClose: () => void
}) {
  const role = getRole(node)
  const s = PG_ROLE_STYLES[role]
  return (
    <>
      <div className="flex items-center justify-between px-4 py-3 border-b">
        <div className="flex items-center gap-2">
          <Server className="w-4 h-4 text-primary" />
          <span className="font-medium text-sm font-mono">{node.name}</span>
          <span className={`text-[10px] font-mono px-1.5 py-0.5 rounded ${s.badge}`}>{s.label}</span>
        </div>
        <div className="flex items-center gap-1">
          {canDelete && <Button variant="ghost" size="icon" className="h-7 w-7 text-destructive hover:text-destructive" onClick={() => onDelete(idx)}><Trash2 className="w-3.5 h-3.5" /></Button>}
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClose}><X className="w-3.5 h-3.5" /></Button>
        </div>
      </div>
      <div className="flex-1 overflow-y-auto p-4 space-y-5">
        <div className="space-y-1.5">
          <Label className="text-[10px] uppercase tracking-wider text-muted-foreground font-mono">Node Name</Label>
          <Input className="h-8 text-sm font-mono" value={node.name} onChange={e => onUpdate(idx, { name: e.target.value })} />
        </div>
        <div className="space-y-3">
          <Label className="text-[10px] uppercase tracking-wider text-muted-foreground font-mono">Hardware</Label>
          <div className="grid grid-cols-3 gap-2">
            <div className="space-y-1">
              <div className="flex items-center gap-1 text-muted-foreground"><Cpu className="w-3 h-3" /><span className="text-[10px] font-mono">CPU</span></div>
              <Input type="number" min={1} className="h-8 text-sm font-mono" value={node.hardware?.cores ?? 2} onChange={e => onUpdate(idx, { hardware: { ...node.hardware!, cores: parseInt(e.target.value) || 1 } })} />
            </div>
            <div className="space-y-1">
              <div className="flex items-center gap-1 text-muted-foreground"><MemoryStick className="w-3 h-3" /><span className="text-[10px] font-mono">RAM</span></div>
              <Input type="number" min={1} className="h-8 text-sm font-mono" value={node.hardware?.memory ?? 4} onChange={e => onUpdate(idx, { hardware: { ...node.hardware!, memory: parseInt(e.target.value) || 1 } })} />
            </div>
            <div className="space-y-1">
              <div className="flex items-center gap-1 text-muted-foreground"><HardDrive className="w-3 h-3" /><span className="text-[10px] font-mono">Disk</span></div>
              <Input type="number" min={1} className="h-8 text-sm font-mono" value={node.hardware?.disk ?? 50} onChange={e => onUpdate(idx, { hardware: { ...node.hardware!, disk: parseInt(e.target.value) || 10 } })} />
            </div>
          </div>
          <div className="text-[10px] text-muted-foreground/40 font-mono">cores &middot; GB &middot; GB</div>
        </div>
        <div className="space-y-3">
          <Label className="text-[10px] uppercase tracking-wider text-muted-foreground font-mono">Services</Label>
          <div className="space-y-2">
            <ServiceToggle label="Etcd" description="Distributed consensus" checked={Boolean(node.etcd)} onChange={v => onUpdate(idx, { etcd: v ? {} as any : undefined })} />
            <ServiceToggle label="PgBouncer" description="Connection pooler" checked={Boolean(node.pgbouncer)} onChange={v => onUpdate(idx, { pgbouncer: v ? {} as any : undefined })} />
            <ServiceToggle label="Monitoring" description="Node & PG exporters" checked={Boolean(node.monitoring)} onChange={v => onUpdate(idx, { monitoring: v ? {} as any : undefined })} />
          </div>
        </div>
      </div>
    </>
  )
}

// ---------------------------------------------------------------------------
// Service toggle
// ---------------------------------------------------------------------------

function ServiceToggle({ label, description, checked, onChange }: {
  label: string; description: string; checked: boolean; onChange: (v: boolean) => void
}) {
  return (
    <button onClick={() => onChange(!checked)}
      className={`w-full flex items-center justify-between px-3 py-2.5 rounded-lg border text-left transition-all duration-150 ${checked ? "border-primary/40 bg-primary/5" : "border-border hover:border-muted-foreground/30"}`}>
      <div>
        <div className="text-sm font-medium text-foreground">{label}</div>
        <div className="text-[10px] text-muted-foreground">{description}</div>
      </div>
      <div className={`w-8 h-5 rounded-full transition-colors flex items-center ${checked ? "bg-primary justify-end" : "bg-muted justify-start"}`}>
        <div className="w-3.5 h-3.5 rounded-full bg-white mx-0.5 shadow-sm" />
      </div>
    </button>
  )
}
