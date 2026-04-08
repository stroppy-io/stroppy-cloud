package types

// Provider is the infrastructure provider for machine provisioning.
type Provider string

const (
	ProviderYandex Provider = "yandex"
	ProviderDocker Provider = "docker" // local emulation via apt/rpm in containers
)

// DatabaseKind is the database to deploy and test.
type DatabaseKind string

const (
	DatabasePostgres DatabaseKind = "postgres"
	DatabaseMySQL    DatabaseKind = "mysql"
	DatabasePicodata DatabaseKind = "picodata"
)

// Phase is the DAG node type identifier for each run stage.
type Phase string

const (
	PhaseNetwork            Phase = "network"           // subnet/VPC allocation
	PhaseMachines           Phase = "machines"          // VM provisioning or Docker container creation
	PhaseInstallDB          Phase = "install_db"        // database binary installation
	PhaseConfigureDB        Phase = "configure_db"      // database cluster configuration
	PhaseInstallMonitor     Phase = "install_monitor"   // monitoring stack deployment
	PhaseConfigureMonitor   Phase = "configure_monitor" // monitoring configuration & targets
	PhaseInstallStroppy     Phase = "install_stroppy"   // stroppy deployment on a dedicated machine
	PhaseRunStroppy         Phase = "run_stroppy"       // stroppy test execution
	PhaseInstallEtcd        Phase = "install_etcd"
	PhaseConfigureEtcd      Phase = "configure_etcd"
	PhaseInstallProxy       Phase = "install_proxy" // HAProxy/ProxySQL
	PhaseConfigureProxy     Phase = "configure_proxy"
	PhaseInstallPgBouncer   Phase = "install_pgbouncer"
	PhaseConfigurePgBouncer Phase = "configure_pgbouncer"
	PhaseTeardown           Phase = "teardown" // infrastructure cleanup
)

// MachineRole distinguishes machines by purpose within a run.
type MachineRole string

const (
	RoleDatabase  MachineRole = "database"
	RoleMonitor   MachineRole = "monitor"
	RoleStroppy   MachineRole = "stroppy"
	RoleEtcd      MachineRole = "etcd"
	RoleProxy     MachineRole = "proxy" // HAProxy / ProxySQL / LB
	RolePgBouncer MachineRole = "pgbouncer"
)

// MachineSpec describes a single machine to provision.
type MachineSpec struct {
	Role     MachineRole `json:"role"`
	Count    int         `json:"count"`
	CPUs     int         `json:"cpus"`
	MemoryMB int         `json:"memory_mb"`
	DiskGB   int         `json:"disk_gb"`
}

// --- Database topologies ---

// PostgresTopology describes a Postgres cluster layout.
type PostgresTopology struct {
	Master       MachineSpec       `json:"master"`
	Replicas     []MachineSpec     `json:"replicas,omitempty"`
	HAProxy      *MachineSpec      `json:"haproxy,omitempty"`
	PgBouncer    bool              `json:"pgbouncer"` // colocated on each PG node
	Patroni      bool              `json:"patroni"`
	Etcd         bool              `json:"etcd"` // colocated on PG nodes (up to 3)
	SyncReplicas int               `json:"sync_replicas"`
	Options      map[string]string `json:"options,omitempty"`
}

// MySQLTopology describes a MySQL cluster layout.
type MySQLTopology struct {
	Primary   MachineSpec       `json:"primary"`
	Replicas  []MachineSpec     `json:"replicas,omitempty"`
	ProxySQL  *MachineSpec      `json:"proxysql,omitempty"` // dedicated ProxySQL node(s)
	GroupRepl bool              `json:"group_replication"`
	SemiSync  bool              `json:"semi_sync"` // semi-synchronous replication (when no GR)
	Options   map[string]string `json:"options,omitempty"`
}

// PicodataTopology describes a Picodata cluster layout.
type PicodataTopology struct {
	Instances   []MachineSpec     `json:"instances"`
	HAProxy     *MachineSpec      `json:"haproxy,omitempty"` // LB for pgproto
	Replication int               `json:"replication_factor"`
	Shards      int               `json:"shards"`
	Tiers       []PicodataTier    `json:"tiers,omitempty"` // for scale deployments
	Options     map[string]string `json:"options,omitempty"`
}

// PicodataTier describes a tier in a multi-tier Picodata deployment.
type PicodataTier struct {
	Name        string `json:"name"`
	Replication int    `json:"replication_factor"`
	CanVote     bool   `json:"can_vote"`
	Count       int    `json:"count"`
}

// DatabaseConfig holds the database specification.
// Exactly one topology field must be set, matching Kind.
type DatabaseConfig struct {
	Kind     DatabaseKind      `json:"kind"`
	Version  string            `json:"version"`
	Postgres *PostgresTopology `json:"postgres,omitempty"`
	MySQL    *MySQLTopology    `json:"mysql,omitempty"`
	Picodata *PicodataTopology `json:"picodata,omitempty"`
}

// --- Database topology presets ---

// PostgresPreset identifies a Postgres topology preset.
type PostgresPreset string

const (
	PostgresSingle PostgresPreset = "single"
	PostgresHA     PostgresPreset = "ha"
	PostgresScale  PostgresPreset = "scale"
)

// MySQLPreset identifies a MySQL topology preset.
type MySQLPreset string

const (
	MySQLSingle  MySQLPreset = "single"
	MySQLReplica MySQLPreset = "replica"
	MySQLGroup   MySQLPreset = "group"
)

// PicodataPreset identifies a Picodata topology preset.
type PicodataPreset string

const (
	PicodataSingle  PicodataPreset = "single"
	PicodataCluster PicodataPreset = "cluster"
	PicodataScale   PicodataPreset = "scale"
)

// PostgresPresets contains all available Postgres topology presets.
var PostgresPresets = map[PostgresPreset]PostgresTopology{
	PostgresSingle: {
		Master: MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
	},
	PostgresHA: {
		Master:       MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 4, MemoryMB: 8192, DiskGB: 100},
		Replicas:     []MachineSpec{{Role: RoleDatabase, Count: 2, CPUs: 4, MemoryMB: 8192, DiskGB: 100}},
		HAProxy:      &MachineSpec{Role: RoleProxy, Count: 1, CPUs: 2, MemoryMB: 2048, DiskGB: 20},
		PgBouncer:    true, // colocated on each PG node
		Patroni:      true,
		Etcd:         true, // colocated on PG nodes (3 nodes)
		SyncReplicas: 1,
	},
	PostgresScale: {
		Master:       MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 8, MemoryMB: 16384, DiskGB: 200},
		Replicas:     []MachineSpec{{Role: RoleDatabase, Count: 4, CPUs: 8, MemoryMB: 16384, DiskGB: 200}},
		HAProxy:      &MachineSpec{Role: RoleProxy, Count: 2, CPUs: 2, MemoryMB: 2048, DiskGB: 20},
		PgBouncer:    true,
		Patroni:      true,
		Etcd:         true, // colocated on first 3 PG nodes
		SyncReplicas: 2,
	},
}

// MySQLPresets contains all available MySQL topology presets.
var MySQLPresets = map[MySQLPreset]MySQLTopology{
	MySQLSingle: {
		Primary: MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
	},
	MySQLReplica: {
		Primary:  MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 4, MemoryMB: 8192, DiskGB: 100},
		Replicas: []MachineSpec{{Role: RoleDatabase, Count: 2, CPUs: 4, MemoryMB: 8192, DiskGB: 100}},
		ProxySQL: &MachineSpec{Role: RoleProxy, Count: 1, CPUs: 2, MemoryMB: 2048, DiskGB: 20},
		SemiSync: true,
	},
	MySQLGroup: {
		Primary:   MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 8, MemoryMB: 16384, DiskGB: 200},
		Replicas:  []MachineSpec{{Role: RoleDatabase, Count: 2, CPUs: 8, MemoryMB: 16384, DiskGB: 200}},
		ProxySQL:  &MachineSpec{Role: RoleProxy, Count: 2, CPUs: 2, MemoryMB: 2048, DiskGB: 20},
		GroupRepl: true,
	},
}

// PicodataPresets contains all available Picodata topology presets.
var PicodataPresets = map[PicodataPreset]PicodataTopology{
	PicodataSingle: {
		Instances:   []MachineSpec{{Role: RoleDatabase, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 50}},
		Replication: 1,
		Shards:      1,
	},
	PicodataCluster: {
		Instances:   []MachineSpec{{Role: RoleDatabase, Count: 3, CPUs: 4, MemoryMB: 8192, DiskGB: 100}},
		HAProxy:     &MachineSpec{Role: RoleProxy, Count: 1, CPUs: 2, MemoryMB: 2048, DiskGB: 20},
		Replication: 2,
		Shards:      3,
	},
	PicodataScale: {
		Instances:   []MachineSpec{{Role: RoleDatabase, Count: 6, CPUs: 8, MemoryMB: 16384, DiskGB: 200}},
		HAProxy:     &MachineSpec{Role: RoleProxy, Count: 2, CPUs: 2, MemoryMB: 2048, DiskGB: 20},
		Replication: 3,
		Shards:      6,
		Tiers: []PicodataTier{
			{Name: "compute", Replication: 1, CanVote: true, Count: 3},
			{Name: "storage", Replication: 2, CanVote: false, Count: 3},
		},
	},
}

// --- Monitoring ---

// MonitorConfig holds monitoring export targets.
// Monitoring is always deployed; this configures where to send data.
type MonitorConfig struct {
	MetricsEndpoint string `json:"metrics_endpoint,omitempty"` // Prometheus remote_write URL
	LogsEndpoint    string `json:"logs_endpoint,omitempty"`    // Loki push URL
}

// StroppyConfig holds stroppy test runner settings.
type StroppyConfig struct {
	Version     string            `json:"version"`
	Workload    string            `json:"workload"`
	Duration    string            `json:"duration"`
	VUSScale    float64           `json:"vus_scale,omitempty"`    // VU scaling factor (1 = default VUs per scenario)
	PoolSize    int               `json:"pool_size,omitempty"`    // DB connection pool size
	ScaleFactor int               `json:"scale_factor,omitempty"` // Warehouses / scale factor for TPC-C
	Workers     int               `json:"workers,omitempty"`      // Deprecated: use vus_scale
	Options     map[string]string `json:"options,omitempty"`
}

// NetworkConfig holds network/subnet allocation settings.
type NetworkConfig struct {
	CIDR string `json:"cidr"`
	Zone string `json:"zone,omitempty"`
}

// RunConfig is the full specification of a test run.
// It is used to build the execution DAG.
type RunConfig struct {
	ID       string         `json:"id"`
	Provider Provider       `json:"provider"`
	Network  NetworkConfig  `json:"network"`
	Machines []MachineSpec  `json:"machines"`
	Database DatabaseConfig `json:"database"`
	Monitor  MonitorConfig  `json:"monitor"`
	Stroppy  StroppyConfig  `json:"stroppy"`
	// PresetID references a presets row. If set and no topology is provided in Database,
	// the preset's topology is applied. Topology in the request takes priority.
	PresetID string `json:"preset_id,omitempty"`
	// PackageID references a packages row. Resolved to ResolvedPackage at run start.
	// If empty, the default built-in package for db_kind+version is used.
	PackageID string `json:"package_id,omitempty"`
	// ResolvedPackage is populated by the server before building the DAG. Not sent by clients.
	ResolvedPackage *Package `json:"-"`
}
