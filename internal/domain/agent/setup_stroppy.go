package agent

// StroppyInstallConfig is the agent payload for stroppy installation.
type StroppyInstallConfig struct {
	Version string `json:"version"`
}

// StroppyRunConfig is the agent payload for stroppy test execution.
type StroppyRunConfig struct {
	DBHost   string            `json:"db_host"`
	DBPort   int               `json:"db_port"`
	DBKind   string            `json:"db_kind"`
	Workload string            `json:"workload"`
	Duration string            `json:"duration"`
	Workers  int               `json:"workers"`
	Options  map[string]string `json:"options,omitempty"`
	OTLPEnv  map[string]string `json:"otlp_env,omitempty"` // K6_OTEL_* vars from server settings
}
