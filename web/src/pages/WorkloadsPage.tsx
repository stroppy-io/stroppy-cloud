import { useEffect, useState, useCallback } from "react"
import { api } from "@/lib/api"
import { Card, CardContent } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Plus, Upload, Trash2, FileCode, Database, MoreVertical } from "lucide-react"
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import type { Workload } from "@/proto/api/api_pb.ts"

export function WorkloadsPage() {
  const [workloads, setWorkloads] = useState<Workload[]>([])
  const [total, setTotal] = useState(0)
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [scriptFile, setScriptFile] = useState<File | null>(null)
  const [sqlFile, setSqlFile] = useState<File | null>(null)
  const [uploading, setUploading] = useState(false)

  const load = useCallback(async () => {
    try {
      const res = await api.listWorkloads({ limit: BigInt(50) })
      setWorkloads(res.workloads)
      setTotal(Number(res.total))
    } catch { /* ignore */ }
  }, [])

  useEffect(() => { load() }, [load])

  const handleUpload = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!scriptFile) return
    setUploading(true)
    try {
      const script = new Uint8Array(await scriptFile.arrayBuffer())
      let sql: Uint8Array | undefined
      if (sqlFile) sql = new Uint8Array(await sqlFile.arrayBuffer())
      await api.registerWorkload({ name, description: description || undefined, script, sql })
      setOpen(false)
      setName("")
      setDescription("")
      setScriptFile(null)
      setSqlFile(null)
      load()
    } catch (err) {
      alert(err instanceof Error ? err.message : "Upload failed")
    }
    setUploading(false)
  }

  const handleDelete = async (id: string) => {
    if (!confirm("Delete this workload?")) return
    try {
      await api.deleteWorkload({ workloadId: id })
      load()
    } catch { /* ignore */ }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Workloads</h1>
          <p className="text-muted-foreground text-sm mt-0.5">
            <span className="font-mono text-xs">{total}</span> registered workload(s)
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button size="sm"><Plus className="w-3.5 h-3.5 mr-1.5" />Register</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Register Workload</DialogTitle>
            </DialogHeader>
            <form onSubmit={handleUpload} className="space-y-4">
              <div className="space-y-1.5">
                <Label className="text-xs">Name</Label>
                <Input value={name} onChange={e => setName(e.target.value)} required placeholder="my-benchmark" />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Description</Label>
                <Input value={description} onChange={e => setDescription(e.target.value)} placeholder="Optional description" />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Script (.ts/.js)</Label>
                <Input type="file" accept=".ts,.js" onChange={e => setScriptFile(e.target.files?.[0] ?? null)} required />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">SQL (optional)</Label>
                <Input type="file" accept=".sql" onChange={e => setSqlFile(e.target.files?.[0] ?? null)} />
              </div>
              <Button type="submit" className="w-full" disabled={uploading}>
                <Upload className="w-3.5 h-3.5 mr-1.5" />{uploading ? "Uploading..." : "Upload & Probe"}
              </Button>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      {workloads.length === 0 ? (
        <Card>
          <CardContent className="py-16 flex flex-col items-center">
            <div className="w-12 h-12 rounded-2xl bg-muted flex items-center justify-center mb-3">
              <Database className="w-6 h-6 text-muted-foreground" />
            </div>
            <p className="text-sm font-medium mb-1">No workloads yet</p>
            <p className="text-xs text-muted-foreground mb-4">Register your first benchmark workload</p>
            <Button size="sm" onClick={() => setOpen(true)}>
              <Plus className="w-3.5 h-3.5 mr-1.5" />Register Workload
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {workloads.map(w => (
            <Card key={w.id} className="group relative hover:border-primary/30 transition-colors">
              <CardContent className="pt-5 pb-4">
                {/* Header */}
                <div className="flex items-start justify-between mb-3">
                  <div className="flex items-center gap-2.5">
                    <div className="w-9 h-9 rounded-lg bg-chart-2/10 flex items-center justify-center shrink-0">
                      <FileCode className="w-4 h-4 text-chart-2" />
                    </div>
                    <div className="min-w-0">
                      <div className="font-medium text-sm truncate">{w.name}</div>
                      <div className="flex items-center gap-1.5 mt-0.5">
                        <Badge variant="secondary" className="font-mono text-[9px]">
                          {w.probe?.driverConfig ? `type-${w.probe.driverConfig.driverType}` : "unknown"}
                        </Badge>
                        {w.builtin && <Badge variant="outline" className="text-[9px] font-mono">builtin</Badge>}
                      </div>
                    </div>
                  </div>
                  {!w.builtin && (
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-7 w-7 opacity-0 group-hover:opacity-100 transition-opacity">
                          <MoreVertical className="w-3.5 h-3.5" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => handleDelete(w.id)} className="text-destructive focus:text-destructive">
                          <Trash2 className="w-3.5 h-3.5 mr-2" />Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  )}
                </div>

                {/* Description */}
                {w.description && (
                  <p className="text-[11px] text-muted-foreground mb-3 line-clamp-2">{w.description}</p>
                )}

                {/* Stats row */}
                <div className="flex items-center gap-4 pt-3 border-t border-border/60">
                  <div className="text-center flex-1">
                    <div className="text-lg font-semibold font-mono">{w.probe?.steps.length ?? 0}</div>
                    <div className="text-[10px] text-muted-foreground uppercase tracking-wider">Steps</div>
                  </div>
                  <div className="w-px h-8 bg-border/60" />
                  <div className="text-center flex-1">
                    <div className="text-lg font-semibold font-mono">{w.probe?.envParams.length ?? 0}</div>
                    <div className="text-[10px] text-muted-foreground uppercase tracking-wider">Params</div>
                  </div>
                </div>

                {/* Steps tags */}
                {(w.probe?.steps.length ?? 0) > 0 && (
                  <div className="flex gap-1.5 mt-3 flex-wrap">
                    {w.probe?.steps.map((s, i) => (
                      <Badge key={i} variant="outline" className="text-[9px] font-mono">{s}</Badge>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
