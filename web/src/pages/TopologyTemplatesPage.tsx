import { useEffect, useState, useCallback } from "react"
import { api } from "@/lib/api"
import { Card, CardContent } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Plus, Trash2, Pencil, Server, Layers, MoreVertical } from "lucide-react"
import { Link } from "react-router-dom"
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger
} from "@/components/ui/dropdown-menu"
import type { TopologyTemplate } from "@/proto/api/api_pb.ts"

const dbTypeLabel = (t: number) => {
  switch (t) {
    case 1: return "PostgreSQL"
    case 2: return "Picodata"
    default: return "Unknown"
  }
}

export function TopologyTemplatesPage() {
  const [templates, setTemplates] = useState<TopologyTemplate[]>([])
  const [total, setTotal] = useState(0)

  const load = useCallback(async () => {
    try {
      const res = await api.listTopologyTemplates({ limit: BigInt(50) })
      setTemplates(res.topologyTemplates)
      setTotal(Number(res.total))
    } catch { /* ignore */ }
  }, [])

  useEffect(() => { load() }, [load])

  const handleDelete = async (id: string) => {
    if (!confirm("Delete this template?")) return
    try {
      await api.deleteTopologyTemplate({ templateId: id })
      load()
    } catch { /* ignore */ }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Topology Templates</h1>
          <p className="text-muted-foreground text-sm mt-0.5">
            <span className="font-mono text-xs">{total}</span> template(s)
          </p>
        </div>
        <Button size="sm" asChild>
          <Link to="/topologies/new"><Plus className="w-3.5 h-3.5 mr-1.5" />New Template</Link>
        </Button>
      </div>

      {templates.length === 0 ? (
        <Card>
          <CardContent className="py-16 flex flex-col items-center">
            <div className="w-12 h-12 rounded-2xl bg-muted flex items-center justify-center mb-3">
              <Layers className="w-6 h-6 text-muted-foreground" />
            </div>
            <p className="text-sm font-medium mb-1">No templates yet</p>
            <p className="text-xs text-muted-foreground mb-4">Design your first database topology</p>
            <Button size="sm" asChild>
              <Link to="/topologies/new"><Plus className="w-3.5 h-3.5 mr-1.5" />Create Template</Link>
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {templates.map(t => {
            const tmpl = t.template
            const templateCase = tmpl?.template.case
            const typeLabel = templateCase?.includes("Cluster") ? "Cluster" :
                             templateCase?.includes("Instance") ? "Instance" : "-"

            // Count nodes for cluster templates
            let nodeCount = 0
            if (templateCase === "postgresCluster" && tmpl?.template.value) {
              nodeCount = (tmpl.template.value as any).nodes?.length ?? 0
            }

            return (
              <Card key={t.id} className="group relative hover:border-primary/30 transition-colors">
                <CardContent className="pt-5 pb-4">
                  {/* Header */}
                  <div className="flex items-start justify-between mb-3">
                    <div className="flex items-center gap-2.5">
                      <div className="w-9 h-9 rounded-lg bg-chart-3/10 flex items-center justify-center shrink-0">
                        <Server className="w-4 h-4 text-chart-3" />
                      </div>
                      <div className="min-w-0">
                        <div className="font-medium text-sm truncate">{t.name}</div>
                        <div className="flex items-center gap-1.5 mt-0.5">
                          <Badge variant="default" className="font-mono text-[9px]">
                            {dbTypeLabel(t.databaseType)}
                          </Badge>
                          {t.builtin && <Badge variant="outline" className="text-[9px] font-mono">builtin</Badge>}
                        </div>
                      </div>
                    </div>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-7 w-7 opacity-0 group-hover:opacity-100 transition-opacity">
                          <MoreVertical className="w-3.5 h-3.5" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem asChild>
                          <Link to={`/topologies/${t.id}/edit`}>
                            <Pencil className="w-3.5 h-3.5 mr-2" />Edit
                          </Link>
                        </DropdownMenuItem>
                        {!t.builtin && (
                          <DropdownMenuItem onClick={() => handleDelete(t.id)} className="text-destructive focus:text-destructive">
                            <Trash2 className="w-3.5 h-3.5 mr-2" />Delete
                          </DropdownMenuItem>
                        )}
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>

                  {/* Description */}
                  {t.description && (
                    <p className="text-[11px] text-muted-foreground mb-3 line-clamp-2">{t.description}</p>
                  )}

                  {/* Visual topology preview */}
                  <div className="rounded-lg bg-muted/30 border border-border/40 p-3 mb-3">
                    <div className="flex items-center justify-center gap-2">
                      {nodeCount > 0 ? (
                        Array.from({ length: Math.min(nodeCount, 5) }).map((_, i) => (
                          <div key={i} className="flex flex-col items-center gap-1">
                            <div className={`w-6 h-6 rounded-md border flex items-center justify-center text-[8px] font-mono ${
                              i === 0 ? "bg-primary/15 border-primary/30 text-primary" : "bg-muted border-border text-muted-foreground"
                            }`}>
                              {i === 0 ? "M" : "R"}
                            </div>
                            {i === 0 && nodeCount > 1 && (
                              <div className="w-px h-2 bg-primary/30" />
                            )}
                          </div>
                        ))
                      ) : (
                        <span className="text-[10px] text-muted-foreground font-mono">{typeLabel}</span>
                      )}
                      {nodeCount > 5 && (
                        <span className="text-[10px] text-muted-foreground font-mono">+{nodeCount - 5}</span>
                      )}
                    </div>
                  </div>

                  {/* Footer */}
                  <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                    <span>{typeLabel} {nodeCount > 0 && `\u00b7 ${nodeCount} nodes`}</span>
                    <span className="font-mono">
                      {t.createdAt ? new Date(Number(t.createdAt.seconds) * 1000).toLocaleDateString() : "-"}
                    </span>
                  </div>
                </CardContent>
              </Card>
            )
          })}
        </div>
      )}
    </div>
  )
}
