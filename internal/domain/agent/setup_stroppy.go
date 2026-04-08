package agent

// StroppyInstallConfig is the agent payload for stroppy installation.
type StroppyInstallConfig struct {
	Version string `json:"version"`
}

// StroppyRunConfig is the agent payload for stroppy test execution.
// The agent writes ConfigJSON to a temp file and runs: stroppy run -f <file>
type StroppyRunConfig struct {
	// ConfigJSON is the full stroppy-config.json content.
	ConfigJSON string `json:"config_json"`
}
