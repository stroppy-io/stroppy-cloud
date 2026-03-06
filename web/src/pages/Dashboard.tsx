import { useEffect } from "react"
import { useNavigate } from "react-router"
import { useStore } from "@nanostores/react"
import { motion } from "framer-motion"
import {
  FlaskConical,
  Play,
  Settings,
  ArrowRight,
  Activity,
  Clock,
} from "lucide-react"
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { $runs, $runsLoading, loadRuns, type TestRunSummary } from "@/stores/runs"
import { RunStatusBadge } from "@/components/runs/RunStatusBadge"

const quickActions = [
  {
    title: "New Test Suite",
    description: "Create and configure a database performance test",
    icon: FlaskConical,
    to: "/editor",
  },
  {
    title: "View Runs",
    description: "Monitor active and past test executions",
    icon: Play,
    to: "/runs",
  },
  {
    title: "Settings",
    description: "Configure Hatchet, Docker, and cloud connections",
    icon: Settings,
    to: "/settings",
  },
]

function formatRelativeTime(iso: string): string {
  if (!iso) return "—"
  const diff = Date.now() - new Date(iso).getTime()
  const seconds = Math.floor(diff / 1000)
  if (seconds < 60) return "just now"
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

function RecentRunRow({ run }: { run: TestRunSummary }) {
  const navigate = useNavigate()

  return (
    <button
      type="button"
      className="flex items-center gap-3 px-3 py-2 w-full text-left hover:bg-accent/30 transition-colors border-b border-border last:border-b-0"
      onClick={() => navigate(`/runs/${run.runId}`)}
    >
      <RunStatusBadge status={run.status} />
      <span className="text-[12px] flex-1 truncate">{run.testSuiteName || run.runId}</span>
      <span className="text-[11px] text-muted-foreground font-mono shrink-0">
        {formatRelativeTime(run.createdAt)}
      </span>
      <ArrowRight className="h-3 w-3 text-muted-foreground shrink-0" />
    </button>
  )
}

export function DashboardPage() {
  const navigate = useNavigate()
  const runs = useStore($runs)
  const loading = useStore($runsLoading)

  useEffect(() => {
    loadRuns(5)
  }, [])

  const item = {
    hidden: { opacity: 0, y: 8 },
    show: { opacity: 1, y: 0 },
  }

  return (
    <motion.div
      initial="hidden"
      animate="show"
      variants={{ show: { transition: { staggerChildren: 0.06 } } }}
      className="p-6 max-w-4xl mx-auto"
    >
      <motion.div variants={item} className="mb-6">
        <h1 className="text-[18px] font-semibold tracking-tight">Dashboard</h1>
        <p className="text-[12px] text-muted-foreground mt-0.5">
          Database performance testing platform
        </p>
      </motion.div>

      {/* Quick Actions */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mb-6">
        {quickActions.map((action) => {
          const Icon = action.icon
          return (
            <motion.div key={action.to} variants={item}>
              <Card
                className="cursor-pointer hover:border-primary/50 transition-colors group"
                onClick={() => navigate(action.to)}
              >
                <CardHeader>
                  <div className="flex items-center justify-between">
                    <Icon className="h-5 w-5 text-primary" />
                    <ArrowRight className="h-3.5 w-3.5 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
                  </div>
                  <CardTitle>{action.title}</CardTitle>
                  <CardDescription>{action.description}</CardDescription>
                </CardHeader>
              </Card>
            </motion.div>
          )
        })}
      </div>

      {/* Recent Runs */}
      <motion.div variants={item}>
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-1.5">
            <Clock className="h-3.5 w-3.5 text-muted-foreground" />
            <h2 className="text-[13px] font-semibold">Recent Runs</h2>
          </div>
          <Button variant="ghost" size="sm" className="text-[11px] h-6" onClick={() => navigate("/runs")}>
            View all
          </Button>
        </div>

        <Card>
          {loading && runs.length === 0 ? (
            <CardContent className="py-6">
              <div className="flex items-center justify-center gap-2 text-[12px] text-muted-foreground">
                <Activity className="h-3.5 w-3.5 animate-pulse" />
                Loading...
              </div>
            </CardContent>
          ) : runs.length === 0 ? (
            <CardContent className="py-6">
              <p className="text-[12px] text-muted-foreground text-center">
                No test runs yet. Create a test suite to get started.
              </p>
            </CardContent>
          ) : (
            <div>
              {runs.slice(0, 5).map((run) => (
                <RecentRunRow key={run.runId} run={run} />
              ))}
            </div>
          )}
        </Card>
      </motion.div>
    </motion.div>
  )
}
