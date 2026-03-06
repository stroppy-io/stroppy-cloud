import { useEffect, useState, useCallback } from "react"
import { useParams, useNavigate } from "react-router"
import { useStore } from "@nanostores/react"
import { ArrowLeft, Loader2, GitBranch, BarChart3 } from "lucide-react"
import { AnimatePresence } from "framer-motion"
import { Button } from "@/components/ui/button"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { RunStatusBadge } from "@/components/runs/RunStatusBadge"
import { DagGraph } from "@/components/runs/DagGraph"
import { StepDetail } from "@/components/runs/StepDetail"
import {
  $workflowGraph,
  startStreamingGraph,
  stopStreaming,
} from "@/stores/runs"

const GRAFANA_BASE_URL = window.location.protocol + "//" + window.location.hostname + ":3000"

export function RunDetailPage() {
  const { runId } = useParams<{ runId: string }>()
  const navigate = useNavigate()
  const graph = useStore($workflowGraph)
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)

  useEffect(() => {
    if (runId) {
      startStreamingGraph(runId)
    }
    return () => {
      stopStreaming()
    }
  }, [runId])

  const handleNodeClick = useCallback((nodeId: string) => {
    setSelectedNodeId((prev) => (prev === nodeId ? null : nodeId))
  }, [])

  const selectedNode = graph?.nodes.find((n) => n.id === selectedNodeId) ?? null

  const completedCount = graph?.nodes.filter(
    (n) => n.status === "WORKFLOW_NODE_STATUS_COMPLETED"
  ).length ?? 0
  const totalCount = graph?.nodes.length ?? 0
  const progressPct = totalCount > 0 ? (completedCount / totalCount) * 100 : 0

  const grafanaUrl = `${GRAFANA_BASE_URL}/d/stroppy-overview?orgId=1&kiosk&theme=dark&var-node=All&var-run_id=${runId ?? ""}`

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center gap-2 border-b bg-secondary/30 px-3 py-1.5">
        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6"
          onClick={() => navigate("/runs")}
        >
          <ArrowLeft className="h-3.5 w-3.5" />
        </Button>
        <span className="text-[12px] font-mono text-muted-foreground truncate">
          {runId}
        </span>
        {graph && (
          <RunStatusBadge
            status={
              graph.status.replace("WORKFLOW_STATUS_", "TEST_RUN_STATUS_") as
                | "TEST_RUN_STATUS_PENDING"
                | "TEST_RUN_STATUS_RUNNING"
                | "TEST_RUN_STATUS_COMPLETED"
                | "TEST_RUN_STATUS_FAILED"
                | "TEST_RUN_STATUS_CANCELLED"
            }
          />
        )}
      </div>

      {/* Progress bar */}
      <div className="h-1 bg-secondary">
        <div
          className="h-full bg-chart-2 transition-all duration-500"
          style={{ width: `${progressPct}%` }}
        />
      </div>

      {/* Tabbed content: DAG + Monitoring */}
      <Tabs defaultValue="dag" className="flex-1 flex flex-col min-h-0">
        <TabsList>
          <TabsTrigger value="dag" className="gap-1.5 text-[12px]">
            <GitBranch className="h-3 w-3" />
            Workflow
          </TabsTrigger>
          <TabsTrigger value="monitoring" className="gap-1.5 text-[12px]">
            <BarChart3 className="h-3 w-3" />
            Monitoring
          </TabsTrigger>
        </TabsList>

        <TabsContent value="dag" className="flex-1 min-h-0">
          {!graph ? (
            <div className="flex-1 flex items-center justify-center h-full">
              <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <div className="flex h-full overflow-hidden">
              <div className="flex-1">
                <DagGraph
                  graph={graph}
                  onNodeClick={handleNodeClick}
                  selectedNodeId={selectedNodeId}
                />
              </div>
              <AnimatePresence>
                {selectedNode && (
                  <StepDetail
                    key={selectedNode.id}
                    node={selectedNode}
                    onClose={() => setSelectedNodeId(null)}
                  />
                )}
              </AnimatePresence>
            </div>
          )}
        </TabsContent>

        <TabsContent value="monitoring" className="flex-1 min-h-0">
          <iframe
            src={grafanaUrl}
            className="w-full h-full border-0"
            title="Grafana Monitoring"
            allow="fullscreen"
          />
        </TabsContent>
      </Tabs>
    </div>
  )
}
