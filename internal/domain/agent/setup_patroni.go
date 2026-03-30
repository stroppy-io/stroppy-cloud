package agent

// PatroniInstallConfig is the agent payload for Patroni installation.
type PatroniInstallConfig struct{}

// PatroniClusterConfig is the agent payload for Patroni configuration.
type PatroniClusterConfig struct {
	Name        string            `json:"name"`         // patroni cluster name
	NodeName    string            `json:"node_name"`    // this node's name
	PGVersion   string            `json:"pg_version"`   // e.g. "16"
	ConnectAddr string            `json:"connect_addr"` // this node's advertised address
	EtcdHosts   string            `json:"etcd_hosts"`   // comma-separated etcd client endpoints
	SyncMode    bool              `json:"sync_mode"`    // synchronous_mode
	SyncCount   int               `json:"sync_count"`   // synchronous_node_count
	PGOptions   map[string]string `json:"pg_options,omitempty"`
}
