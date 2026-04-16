package agent

// YDBInstallConfig is the agent payload for YDB binary installation.
type YDBInstallConfig struct {
	Version string `json:"version"`
}

// YDBStaticConfig is the agent payload for starting a YDB static (storage) node.
type YDBStaticConfig struct {
	Hosts          []string          `json:"hosts"` // all static node addresses
	InstanceID     int               `json:"instance_id"`
	AdvertiseHost  string            `json:"advertise_host"`
	DiskPath       string            `json:"disk_path"` // "/ydb_data" in Docker, real disk on VM
	DiskGB         int               `json:"disk_gb"`   // allocated disk size; pdisk is sized to this
	MemoryMB       int               `json:"memory_mb"` // total machine RAM for memory limits
	CPUs           int               `json:"cpus"`      // vCPUs for actor system tuning
	FaultTolerance string            `json:"fault_tolerance"`
	Options        map[string]string `json:"options,omitempty"`
}

// YDBInitConfig is the agent payload for cluster initialization (runs on one static node).
type YDBInitConfig struct {
	StaticEndpoint string `json:"static_endpoint"` // grpc://<host>:2136
	DatabasePath   string `json:"database_path"`   // /Root/testdb
	ConfigPath     string `json:"config_path"`     // /opt/ydb/cfg/config.yaml
}

// YDBDatabaseConfig is the agent payload for starting a YDB dynamic (database) node.
type YDBDatabaseConfig struct {
	StaticEndpoints []string          `json:"static_endpoints"` // node-broker addresses
	AdvertiseHost   string            `json:"advertise_host"`
	DatabasePath    string            `json:"database_path"` // /Root/testdb
	MemoryMB        int               `json:"memory_mb"`     // total machine RAM for memory limits
	CPUs            int               `json:"cpus"`          // vCPUs for actor system tuning
	Options         map[string]string `json:"options,omitempty"`
}
