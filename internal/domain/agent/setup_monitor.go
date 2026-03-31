package agent

// MonitorInstallConfig is the agent payload for monitoring stack installation.
type MonitorInstallConfig struct{}

// MonitorSetupConfig is the agent payload for monitoring configuration.
type MonitorSetupConfig struct {
	MetricsEndpoint string   `json:"metrics_endpoint"`
	LogsEndpoint    string   `json:"logs_endpoint"`
	ScrapeTargets   []string `json:"scrape_targets"` // host:port of all monitored nodes
}
