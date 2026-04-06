package agent

// MonitorInstallConfig is the agent payload for monitoring stack installation.
// DatabaseKind tells the agent whether to install a DB-specific exporter
// (e.g. postgres_exporter). Empty means "no DB exporter needed".
type MonitorInstallConfig struct {
	DatabaseKind string `json:"database_kind,omitempty"`
}

// MonitorSetupConfig is the agent payload for monitoring configuration.
type MonitorSetupConfig struct {
	MetricsEndpoint string   `json:"metrics_endpoint"` // VictoriaMetrics remote_write URL
	LogsEndpoint    string   `json:"logs_endpoint"`
	ScrapeTargets   []string `json:"scrape_targets"` // internal hostnames of all monitored nodes
	RunID           string   `json:"run_id"`         // added as external label to all metrics
	DatabaseKind    string   `json:"database_kind,omitempty"`
	BearerToken     string   `json:"bearer_token,omitempty"` // auth token for vmauth
}
