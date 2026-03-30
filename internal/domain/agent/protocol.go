package agent

// Command is sent from server to agent to execute a setup step.
type Command struct {
	ID     string `json:"id"`
	Action Action `json:"action"`
	Config any    `json:"config"` // action-specific payload
}

// Action identifies what the agent should do.
type Action string

const (
	ActionInstallPostgres  Action = "install_postgres"
	ActionConfigPostgres   Action = "config_postgres"
	ActionInstallMySQL     Action = "install_mysql"
	ActionConfigMySQL      Action = "config_mysql"
	ActionInstallPicodata  Action = "install_picodata"
	ActionConfigPicodata   Action = "config_picodata"
	ActionInstallMonitor   Action = "install_monitor"
	ActionConfigMonitor    Action = "config_monitor"
	ActionInstallStroppy   Action = "install_stroppy"
	ActionRunStroppy       Action = "run_stroppy"
	ActionInstallEtcd      Action = "install_etcd"
	ActionConfigEtcd       Action = "config_etcd"
	ActionInstallPatroni   Action = "install_patroni"
	ActionConfigPatroni    Action = "config_patroni"
	ActionInstallPgBouncer Action = "install_pgbouncer"
	ActionConfigPgBouncer  Action = "config_pgbouncer"
	ActionInstallHAProxy   Action = "install_haproxy"
	ActionConfigHAProxy    Action = "config_haproxy"
	ActionInstallProxySQL  Action = "install_proxysql"
	ActionConfigProxySQL   Action = "config_proxysql"
	ActionShutdown         Action = "shutdown"
)

// Report is sent from agent back to server.
type Report struct {
	CommandID string       `json:"command_id"`
	Status    ReportStatus `json:"status"`
	Error     string       `json:"error,omitempty"`
	Output    string       `json:"output,omitempty"`
}

// ReportStatus indicates the outcome of a command execution.
type ReportStatus string

const (
	ReportRunning   ReportStatus = "running"
	ReportCompleted ReportStatus = "completed"
	ReportFailed    ReportStatus = "failed"
)

// LogLine is a streamed log entry from agent to server.
type LogLine struct {
	CommandID string `json:"command_id"`
	Line      string `json:"line"`
	Stream    string `json:"stream"` // "stdout" or "stderr"
}
