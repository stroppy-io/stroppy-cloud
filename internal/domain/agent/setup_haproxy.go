package agent

// HAProxyInstallConfig is the agent payload for HAProxy installation.
type HAProxyInstallConfig struct{}

// HAProxyConfig is the agent payload for HAProxy configuration.
type HAProxyConfig struct {
	DBKind      string   `json:"db_kind"`      // "postgres", "mysql", "picodata"
	WritePort   int      `json:"write_port"`   // e.g. 5000 (PG), 3306 (MySQL)
	ReadPort    int      `json:"read_port"`    // e.g. 5001 (PG), 3307 (MySQL)
	Backends    []string `json:"backends"`     // host:port list
	HealthCheck string   `json:"health_check"` // "patroni" or "mysql" or "tcp"
	PatroniPort int      `json:"patroni_port"` // 8008 (Patroni REST API)
}
