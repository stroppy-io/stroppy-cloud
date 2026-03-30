package agent

// PgBouncerInstallConfig is the agent payload for PgBouncer installation.
type PgBouncerInstallConfig struct{}

// PgBouncerConfig is the agent payload for PgBouncer configuration.
type PgBouncerConfig struct {
	ListenPort      int    `json:"listen_port"`       // default 6432
	PoolMode        string `json:"pool_mode"`         // "transaction" (recommended)
	MaxClientConn   int    `json:"max_client_conn"`   // default 1000
	DefaultPoolSize int    `json:"default_pool_size"` // default 25
	PGHost          string `json:"pg_host"`           // localhost (colocated)
	PGPort          int    `json:"pg_port"`           // 5432
	AuthType        string `json:"auth_type"`         // "trust" for testing
}
