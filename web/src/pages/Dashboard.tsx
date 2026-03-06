import { useNavigate } from "react-router"
import { FlaskConical, Play, Settings, ArrowRight } from "lucide-react"
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card"
import { Button } from "@/components/ui/button"

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

export function DashboardPage() {
  const navigate = useNavigate()

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="mb-8">
        <h1 className="text-[20px] font-semibold mb-1">Dashboard</h1>
        <p className="text-[13px] text-muted-foreground">
          Database performance testing sandbox
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
        {quickActions.map((action) => {
          const Icon = action.icon
          return (
            <Card
              key={action.to}
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
          )
        })}
      </div>

      <div className="mt-8">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-[14px] font-semibold">Recent Runs</h2>
          <Button variant="ghost" size="sm" onClick={() => navigate("/runs")}>
            View all
          </Button>
        </div>
        <Card>
          <CardContent className="p-6">
            <p className="text-[13px] text-muted-foreground text-center">
              No test runs yet. Create a test suite to get started.
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
