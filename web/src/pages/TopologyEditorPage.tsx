import { useState, useEffect, useCallback } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import {
  Save, Loader2, ChevronLeft, Plus, Trash2, Server, Cpu, HardDrive,
  MemoryStick, X, Settings2, PanelRightOpen, PanelRightClose,
} from "lucide-react"
import {
  ReactFlow, Background, Controls, MiniMap,
  type Node, type Edge, Position,
  useNodesState, useEdgesState,
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
import { Database_TemplateSchema, type Database_Template } from "@/proto/database/database_pb.ts"
import { HardwareSchema } from "@/proto/deployment/deployment_pb.ts"

export function TopologyEditorPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const isEdit = Boolean(id)

  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [saving, setSaving] = useState(false)
  const [loading, setLoading] = useState(isEdit)

  const [selectedNodeIdx, setSelectedNodeIdx] = useState<number | null>(null)
  const [showSettings, setShowSettings] = useState(false)

  const [pgVersion, setPgVersion] = useState(Postgres_Settings_Version.VERSION_17)
  const [storageEngine, setStorageEngine] = useState(Postgres_Settings_StorageEngine.HEAP)
  const [pgNodes, setPgNodes] = useState<Postgres_Node[]>([
    create(Postgres_NodeSchema, {
      name: "master",
      hardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 50 }),
      postgres: create(Postgres_PostgresServiceSchema, { role: Postgres_PostgresService_Role.MASTER }),
    }),
    create(Postgres_NodeSchema, {
      name: "replica-1",
      hardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 50 }),
      postgres: create(Postgres_PostgresServiceSchema, { role: Postgres_PostgresService_Role.REPLICA }),
    }),
  ])

  useEffect(() => {
    if (!id) return
    api.getTopologyTemplate({ templateId: id }).then(res => {
      const t = res.topologyTemplate
      if (!t) return
      setName(t.name)
      setDescription(t.description ?? "")
      if (t.template?.template.case === "postgresCluster") {
        const cluster = t.template.template.value as Postgres_Cluster
        if (cluster.defaults) {
          setPgVersion(cluster.defaults.version)
          setStorageEngine(cluster.defaults.storageEngine)
        }
        if (cluster.nodes.length > 0) setPgNodes(cluster.nodes)
      }
      setLoading(false)
    }).catch(() => { setLoading(false); navigate("/topologies") })
  }, [id, navigate])

  const addNode = (role: Postgres_PostgresService_Role) => {
    const roleName = role === Postgres_PostgresService_Role.MASTER ? "master" : "replica"
    const count = pgNodes.filter(n => n.name.startsWith(roleName)).length
    setPgNodes(prev => [...prev, create(Postgres_NodeSchema, {
      name: `${roleName}-${count + 1}`,
      hardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 50 }),
      postgres: create(Postgres_PostgresServiceSchema, { role }),
    })])
  }

  const removeNode = (idx: number) => {
    if (pgNodes.length <= 2) return
    if (selectedNodeIdx === idx) setSelectedNodeIdx(null)
    else if (selectedNodeIdx !== null && selectedNodeIdx > idx) setSelectedNodeIdx(selectedNodeIdx - 1)
    setPgNodes(prev => prev.filter((_, i) => i !== idx))
  }

  const updateNode = (idx: number, updates: Partial<Postgres_Node>) => {
    setPgNodes(prev => prev.map((n, i) => i === idx ? { ...n, ...updates } as Postgres_Node : n))
  }

  const buildTemplate = (): Database_Template => {
    const defaults = create(Postgres_SettingsSchema, { version: pgVersion, storageEngine })
    const cluster = create(Postgres_ClusterSchema, { defaults, nodes: pgNodes })
    return create(Database_TemplateSchema, { template: { case: "postgresCluster", value: cluster } })
  }

  const handleSave = async () => {
    if (!name.trim()) { alert("Name is required"); return }
    setSaving(true)
    try {
      const template = buildTemplate()
      if (isEdit && id) {
        await api.updateTopologyTemplate({ templateId: id, name, description: description || undefined, template })
      } else {
        await api.createTopologyTemplate({ name, description: description || undefined, template })
      }
      navigate("/topologies")
    } catch (err) { alert(err instanceof Error ? err.message : "Save failed") }
    setSaving(false)
  }

  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])

  const buildGraph = useCallback(() => {
    const masters = pgNodes.filter(n => n.postgres?.role === Postgres_PostgresService_Role.MASTER)
    const replicas = pgNodes.filter(n => n.postgres?.role === Postgres_PostgresService_Role.REPLICA)
    const others = pgNodes.filter(n => !n.postgres || (n.postgres.role !== Postgres_PostgresService_Role.MASTER && n.postgres.role !== Postgres_PostgresService_Role.REPLICA))

    const flowNodes: Node[] = []
    const totalWidth = Math.max(masters.length, replicas.length, others.length) * 260
    const centerX = totalWidth / 2

    masters.forEach((n, i) => {
      const x = centerX - (masters.length * 260) / 2 + i * 260
      const idx = pgNodes.indexOf(n)
      flowNodes.push(makeFlowNode(n, idx, x, 60, idx === selectedNodeIdx))
    })

    replicas.forEach((n, i) => {
      const x = centerX - (replicas.length * 260) / 2 + i * 260
      const idx = pgNodes.indexOf(n)
      flowNodes.push(makeFlowNode(n, idx, x, 260, idx === selectedNodeIdx))
    })

    others.forEach((n, i) => {
      const x = centerX - (others.length * 260) / 2 + i * 260
      const idx = pgNodes.indexOf(n)
      flowNodes.push(makeFlowNode(n, idx, x, 460, idx === selectedNodeIdx))
    })

    const master = pgNodes.find(n => n.postgres?.role === Postgres_PostgresService_Role.MASTER)
    const flowEdges: Edge[] = replicas.map(n => ({
      id: `${master?.name ?? "m"}-${n.name}`,
      source: master?.name ?? "",
      target: n.name,
      label: "replication",
      animated: true,
      style: { stroke: 'var(--color-primary)', strokeWidth: 2 },
      labelStyle: { fontSize: 10, fill: 'var(--color-muted-foreground)', fontFamily: 'var(--font-mono)' },
    }))

    setNodes(flowNodes)
    setEdges(flowEdges)
  }, [pgNodes, setNodes, setEdges, selectedNodeIdx])

  function makeFlowNode(n: Postgres_Node, idx: number, x: number, y: number, selected: boolean): Node {
    const isMaster = n.postgres?.role === Postgres_PostgresService_Role.MASTER
    const services: string[] = []
    if (n.etcd) services.push("etcd")
    if (n.pgbouncer) services.push("pgb")
    if (n.monitoring) services.push("mon")

    return {
      id: n.name,
      position: { x, y },
      data: {
        label: (
          <div
            className="cursor-pointer select-none"
            onClick={() => { setSelectedNodeIdx(idx); setShowSettings(false) }}
          >
            <div className="flex items-center gap-1.5 mb-1">
              <Server className="w-3.5 h-3.5 text-primary" />
              <span className="font-semibold text-xs text-foreground">{n.name}</span>
            </div>
            <Badge
              variant={isMaster ? "default" : "secondary"}
              className="text-[10px] mb-1.5 font-mono"
            >
              {isMaster ? "Master" : "Replica"}
            </Badge>
            <div className="flex gap-3 text-[10px] text-muted-foreground font-mono">
              <span>{n.hardware?.cores ?? 0}C</span>
              <span>{n.hardware?.memory ?? 0}G</span>
              <span>{n.hardware?.disk ?? 0}GB</span>
            </div>
            {services.length > 0 && (
              <div className="flex gap-1 mt-1.5 flex-wrap">
                {services.map(s => (
                  <span key={s} className="text-[9px] px-1.5 py-0.5 rounded-md bg-muted text-muted-foreground font-mono">
                    {s}
                  </span>
                ))}
              </div>
            )}
          </div>
        ),
      },
      sourcePosition: Position.Bottom,
      targetPosition: Position.Top,
      className: `!rounded-xl !p-3 !border !bg-card/95 !backdrop-blur-sm ${
        selected
          ? "!border-primary !shadow-[0_0_20px_var(--color-primary)/0.15]"
          : "!border-border hover:!border-primary/50"
      }`,
    }
  }

  useEffect(() => { buildGraph() }, [buildGraph])

  const selectedNode = selectedNodeIdx !== null ? pgNodes[selectedNodeIdx] : null
  const panelOpen = selectedNodeIdx !== null || showSettings

  if (loading) {
    return <div className="flex items-center justify-center h-64"><Loader2 className="w-5 h-5 animate-spin text-muted-foreground" /></div>
  }

  return (
    <div className="-m-6 h-[calc(100vh-3.5rem)] relative overflow-hidden">
      <div className="absolute inset-0">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onPaneClick={() => { setSelectedNodeIdx(null); setShowSettings(false) }}
          fitView
          fitViewOptions={{ padding: 0.3 }}
          proOptions={{ hideAttribution: true }}
          minZoom={0.3}
          maxZoom={2}
        >
          <Background gap={20} size={1} className="opacity-30" />
          <Controls
            showInteractive={false}
            className="!bg-card/90 !backdrop-blur-sm !border-border !rounded-lg !shadow-lg"
          />
          <MiniMap
            className="!bg-card/90 !backdrop-blur-sm !border-border !rounded-lg"
            maskColor="var(--color-background)"
            nodeColor="var(--color-primary)"
          />
        </ReactFlow>
      </div>

      {/* Top toolbar */}
      <div className="absolute top-4 left-4 right-4 z-10 pointer-events-none">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 pointer-events-auto">
            <Button
              variant="outline"
              size="icon"
              className="h-8 w-8 bg-card/90 backdrop-blur-sm shadow-lg"
              onClick={() => navigate("/topologies")}
            >
              <ChevronLeft className="w-4 h-4" />
            </Button>
            <div className="bg-card/90 backdrop-blur-sm rounded-lg border shadow-lg px-3 py-1.5 flex items-center gap-2">
              <Input
                className="h-7 w-44 bg-transparent border-none text-sm font-semibold placeholder:text-muted-foreground/50 focus-visible:ring-0 px-0"
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder="Template name..."
              />
              <span className="text-muted-foreground/30">|</span>
              <Input
                className="h-7 w-40 bg-transparent border-none text-xs text-muted-foreground placeholder:text-muted-foreground/30 focus-visible:ring-0 px-0"
                value={description}
                onChange={e => setDescription(e.target.value)}
                placeholder="Description..."
              />
            </div>
          </div>

          <div className="flex items-center gap-2 pointer-events-auto">
            <Button
              variant="outline"
              size="sm"
              className="h-8 bg-card/90 backdrop-blur-sm shadow-lg text-xs"
              onClick={() => { setShowSettings(!showSettings); setSelectedNodeIdx(null) }}
            >
              <Settings2 className="w-3.5 h-3.5 mr-1.5" />
              Settings
            </Button>
            <Button
              variant="outline"
              size="icon"
              className="h-8 w-8 bg-card/90 backdrop-blur-sm shadow-lg"
              onClick={() => panelOpen
                ? (setSelectedNodeIdx(null), setShowSettings(false))
                : setShowSettings(true)
              }
            >
              {panelOpen
                ? <PanelRightClose className="w-3.5 h-3.5" />
                : <PanelRightOpen className="w-3.5 h-3.5" />
              }
            </Button>
            <Button
              onClick={handleSave}
              disabled={saving}
              size="sm"
              className="h-8 shadow-lg"
            >
              {saving ? <Loader2 className="w-3.5 h-3.5 mr-1.5 animate-spin" /> : <Save className="w-3.5 h-3.5 mr-1.5" />}
              {isEdit ? "Update" : "Save"}
            </Button>
          </div>
        </div>
      </div>

      {/* Bottom toolbar */}
      <div className="absolute bottom-6 left-1/2 -translate-x-1/2 z-10 pointer-events-auto">
        <div className="bg-card/90 backdrop-blur-sm rounded-xl border shadow-lg px-4 py-2 flex items-center gap-3">
          <span className="text-[10px] text-muted-foreground font-mono uppercase tracking-wider">Add</span>
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs"
            onClick={() => addNode(Postgres_PostgresService_Role.REPLICA)}
          >
            <Plus className="w-3 h-3 mr-1" />Replica
          </Button>
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <span className="font-mono">{pgNodes.length}</span>
            <span>node{pgNodes.length !== 1 ? "s" : ""}</span>
          </div>
        </div>
      </div>

      {/* Right panel */}
      <AnimatePresence>
        {panelOpen && (
          <motion.div
            initial={{ x: 360, opacity: 0 }}
            animate={{ x: 0, opacity: 1 }}
            exit={{ x: 360, opacity: 0 }}
            transition={{ type: "spring", damping: 30, stiffness: 350 }}
            className="absolute top-4 right-4 bottom-4 w-80 z-10 pointer-events-auto"
          >
            <div className="h-full bg-card/95 backdrop-blur-md border rounded-xl shadow-2xl flex flex-col overflow-hidden">
              {showSettings ? (
                <SettingsPanel
                  pgVersion={pgVersion}
                  setPgVersion={setPgVersion}
                  storageEngine={storageEngine}
                  setStorageEngine={setStorageEngine}
                  onClose={() => setShowSettings(false)}
                />
              ) : selectedNode ? (
                <NodePanel
                  node={selectedNode}
                  idx={selectedNodeIdx!}
                  canDelete={pgNodes.length > 2}
                  onUpdate={updateNode}
                  onDelete={removeNode}
                  onClose={() => setSelectedNodeIdx(null)}
                />
              ) : null}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

function SettingsPanel({
  pgVersion, setPgVersion, storageEngine, setStorageEngine, onClose,
}: {
  pgVersion: number
  setPgVersion: (v: number) => void
  storageEngine: number
  setStorageEngine: (v: number) => void
  onClose: () => void
}) {
  return (
    <>
      <div className="flex items-center justify-between px-4 py-3 border-b">
        <div className="flex items-center gap-2">
          <Settings2 className="w-4 h-4 text-primary" />
          <span className="font-medium text-sm">Cluster Settings</span>
        </div>
        <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClose}>
          <X className="w-3.5 h-3.5" />
        </Button>
      </div>
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        <div className="space-y-1.5">
          <Label className="text-[10px] uppercase tracking-wider text-muted-foreground font-mono">Version</Label>
          <select
            className="w-full h-8 rounded-lg border border-input bg-background text-foreground px-3 text-sm"
            value={pgVersion}
            onChange={e => setPgVersion(Number(e.target.value))}
          >
            <option value={Postgres_Settings_Version.VERSION_17}>PostgreSQL 17</option>
            <option value={Postgres_Settings_Version.VERSION_16}>PostgreSQL 16</option>
            <option value={Postgres_Settings_Version.VERSION_18}>PostgreSQL 18</option>
          </select>
        </div>
        <div className="space-y-1.5">
          <Label className="text-[10px] uppercase tracking-wider text-muted-foreground font-mono">Storage Engine</Label>
          <select
            className="w-full h-8 rounded-lg border border-input bg-background text-foreground px-3 text-sm"
            value={storageEngine}
            onChange={e => setStorageEngine(Number(e.target.value))}
          >
            <option value={Postgres_Settings_StorageEngine.HEAP}>Heap (default)</option>
            <option value={Postgres_Settings_StorageEngine.ORIOLEDB}>OrioleDB</option>
          </select>
        </div>
      </div>
    </>
  )
}

function NodePanel({
  node, idx, canDelete, onUpdate, onDelete, onClose,
}: {
  node: Postgres_Node
  idx: number
  canDelete: boolean
  onUpdate: (idx: number, updates: Partial<Postgres_Node>) => void
  onDelete: (idx: number) => void
  onClose: () => void
}) {
  const isMaster = node.postgres?.role === Postgres_PostgresService_Role.MASTER

  return (
    <>
      <div className="flex items-center justify-between px-4 py-3 border-b">
        <div className="flex items-center gap-2">
          <Server className="w-4 h-4 text-primary" />
          <span className="font-medium text-sm">{node.name}</span>
          <Badge variant={isMaster ? "default" : "secondary"} className="text-[10px] font-mono">
            {isMaster ? "Master" : "Replica"}
          </Badge>
        </div>
        <div className="flex items-center gap-1">
          {canDelete && (
            <Button variant="ghost" size="icon" className="h-7 w-7 text-destructive hover:text-destructive" onClick={() => onDelete(idx)}>
              <Trash2 className="w-3.5 h-3.5" />
            </Button>
          )}
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClose}>
            <X className="w-3.5 h-3.5" />
          </Button>
        </div>
      </div>
      <div className="flex-1 overflow-y-auto p-4 space-y-5">
        <div className="space-y-1.5">
          <Label className="text-[10px] uppercase tracking-wider text-muted-foreground font-mono">Node Name</Label>
          <Input
            className="h-8 text-sm"
            value={node.name}
            onChange={e => onUpdate(idx, { name: e.target.value })}
          />
        </div>

        <div className="space-y-3">
          <Label className="text-[10px] uppercase tracking-wider text-muted-foreground font-mono">Hardware</Label>
          <div className="grid grid-cols-3 gap-2">
            <div className="space-y-1">
              <div className="flex items-center gap-1 text-muted-foreground">
                <Cpu className="w-3 h-3" />
                <span className="text-[10px] font-mono">CPU</span>
              </div>
              <Input
                type="number"
                min={1}
                className="h-8 text-sm font-mono"
                value={node.hardware?.cores ?? 2}
                onChange={e => onUpdate(idx, {
                  hardware: { ...node.hardware!, cores: parseInt(e.target.value) || 1 },
                })}
              />
            </div>
            <div className="space-y-1">
              <div className="flex items-center gap-1 text-muted-foreground">
                <MemoryStick className="w-3 h-3" />
                <span className="text-[10px] font-mono">RAM</span>
              </div>
              <Input
                type="number"
                min={1}
                className="h-8 text-sm font-mono"
                value={node.hardware?.memory ?? 4}
                onChange={e => onUpdate(idx, {
                  hardware: { ...node.hardware!, memory: parseInt(e.target.value) || 1 },
                })}
              />
            </div>
            <div className="space-y-1">
              <div className="flex items-center gap-1 text-muted-foreground">
                <HardDrive className="w-3 h-3" />
                <span className="text-[10px] font-mono">Disk</span>
              </div>
              <Input
                type="number"
                min={1}
                className="h-8 text-sm font-mono"
                value={node.hardware?.disk ?? 50}
                onChange={e => onUpdate(idx, {
                  hardware: { ...node.hardware!, disk: parseInt(e.target.value) || 10 },
                })}
              />
            </div>
          </div>
          <div className="text-[10px] text-muted-foreground/50 font-mono">
            cores · GB · GB
          </div>
        </div>

        <div className="space-y-3">
          <Label className="text-[10px] uppercase tracking-wider text-muted-foreground font-mono">Services</Label>
          <div className="space-y-2">
            <ServiceToggle
              label="Etcd"
              description="Distributed consensus"
              checked={Boolean(node.etcd)}
              onChange={v => onUpdate(idx, { etcd: v ? {} as any : undefined })}
            />
            <ServiceToggle
              label="PgBouncer"
              description="Connection pooler"
              checked={Boolean(node.pgbouncer)}
              onChange={v => onUpdate(idx, { pgbouncer: v ? {} as any : undefined })}
            />
            <ServiceToggle
              label="Monitoring"
              description="Node & PG exporters"
              checked={Boolean(node.monitoring)}
              onChange={v => onUpdate(idx, { monitoring: v ? {} as any : undefined })}
            />
          </div>
        </div>
      </div>
    </>
  )
}

function ServiceToggle({
  label, description, checked, onChange,
}: {
  label: string
  description: string
  checked: boolean
  onChange: (v: boolean) => void
}) {
  return (
    <button
      onClick={() => onChange(!checked)}
      className={`w-full flex items-center justify-between px-3 py-2.5 rounded-lg border text-left transition-all duration-150 ${
        checked
          ? "border-primary/40 bg-primary/5"
          : "border-border hover:border-muted-foreground/30"
      }`}
    >
      <div>
        <div className="text-sm font-medium text-foreground">{label}</div>
        <div className="text-[10px] text-muted-foreground">{description}</div>
      </div>
      <div className={`w-8 h-5 rounded-full transition-colors flex items-center ${
        checked ? "bg-primary justify-end" : "bg-muted justify-start"
      }`}>
        <div className="w-3.5 h-3.5 rounded-full bg-white mx-0.5 shadow-sm" />
      </div>
    </button>
  )
}
