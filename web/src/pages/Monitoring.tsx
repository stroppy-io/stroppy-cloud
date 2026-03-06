import { useState } from "react"
import { motion } from "framer-motion"

const GRAFANA_BASE_URL = window.location.protocol + "//" + window.location.hostname + ":3000"

const dashboards = [
  { uid: "stroppy-overview", label: "Overview" },
  { uid: "stroppy-database", label: "Database" },
  { uid: "stroppy-load", label: "Load Test" },
] as const

export function MonitoringPage() {
  const [active, setActive] = useState<string>(dashboards[0].uid)
  const grafanaUrl = `${GRAFANA_BASE_URL}/d/${active}?orgId=1&kiosk&theme=dark&var-node=All`

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.2 }}
      className="flex flex-col h-full"
    >
      <div className="flex items-center gap-3 border-b border-border bg-secondary/20 px-4 py-1.5">
        <h1 className="text-[14px] font-semibold">Monitoring</h1>
        <div className="flex items-center gap-0.5 ml-2">
          {dashboards.map((d) => (
            <button
              key={d.uid}
              onClick={() => setActive(d.uid)}
              className={`px-2 py-0.5 text-[12px] rounded transition-colors ${
                active === d.uid
                  ? "bg-primary/10 text-primary font-medium"
                  : "text-muted-foreground hover:text-foreground hover:bg-secondary"
              }`}
            >
              {d.label}
            </button>
          ))}
        </div>
        <a
          href={GRAFANA_BASE_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="ml-auto text-[11px] text-primary hover:underline"
        >
          Open Grafana
        </a>
      </div>
      <iframe
        key={active}
        src={grafanaUrl}
        className="flex-1 w-full border-0"
        title="Grafana Monitoring"
        allow="fullscreen"
      />
    </motion.div>
  )
}
