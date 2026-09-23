package catalog

// DatabaseKind is a database kind id.
type DatabaseKind string

// Kinds.
const (
	Postgres   DatabaseKind = "postgres"
	MySQL      DatabaseKind = "mysql"
	MariaDB    DatabaseKind = "mariadb"
	Picodata   DatabaseKind = "picodata"
	YDB        DatabaseKind = "ydb"
	YDBManaged DatabaseKind = "ydb_managed"
	Cockroach  DatabaseKind = "cockroach"
	OrioleDB   DatabaseKind = "orioledb"
	External   DatabaseKind = "external"
	Noop       DatabaseKind = "noop"
	PgNoop     DatabaseKind = "pg_noop"
)

// Protocol is a stroppy driver protocol.
type Protocol string

// Protocols.
const (
	ProtoPg        Protocol = "pg"
	ProtoMySQL     Protocol = "mysql"
	ProtoPicodata  Protocol = "picodata"
	ProtoYDBGrpc   Protocol = "ydb_grpc"
	ProtoYDBGrpcs  Protocol = "ydb_grpcs"
	ProtoCockroach Protocol = "cockroach"
	ProtoNoop      Protocol = "noop"
)

// Database is one kind.
type Database struct {
	Kind         DatabaseKind
	Title        string
	Description  string
	Versions     []Version
	Roles        []Role
	Topologies   []Topology
	ParamsSchema string
	Protocols    []Protocol
	// Deployable is false for kinds the platform only connects to.
	Deployable bool
}

// Version of a database.
type Version struct {
	Version    string
	Image      string
	Default    bool
	Deprecated bool
}

// Role is a machine role of a topology.
type Role struct {
	Role          string
	Title         string
	Engine        string
	ConfigSchemas []string
	// ConfigSeeds are the recipe's own values per config schema — what the
	// server fills before the user's deviations (preloaded extensions,
	// data directories). They are part of the effective config the user
	// sees and may override.
	ConfigSeeds map[string]map[string]any
}

// Topology is a template: a name and a params value.
type Topology struct {
	ID          string
	Title       string
	Description string
	Params      map[string]any
}

// Provider is one cloud.
type Provider struct {
	Kind              ProviderKind
	Title             string
	SettingsSchema    string
	CredentialsSchema string
	Locations         []Named
	Platforms         []Named
	DiskTypes         []DiskType
	// Sizes: role family → size → machine.
	Sizes  map[string]map[string]SizeSpec
	Images []Image
}

// SizeTable is the size table of a role family. A family without its own
// table (etcd, …) sizes like the small service machines: proxy.
func (p Provider) SizeTable(family string) map[string]SizeSpec {
	if t, ok := p.Sizes[family]; ok {
		return t
	}
	return p.Sizes["proxy"]
}

// ProviderKind is a cloud id.
type ProviderKind string

// Provider kinds.
const (
	Yandex ProviderKind = "yandex"
	AWS    ProviderKind = "aws"
)

// Named is an id with a title.
type Named struct {
	ID    string
	Title string
}

// DiskType is a provider disk type.
type DiskType struct {
	ID     string
	Title  string
	MinGB  int
	StepGB int
}

// Image is a boot image.
type Image struct {
	ID string
	OS string
}

// SizeSpec is what a T-shirt size means for a role family. JSON tags are
// the system.sizes@1 field names.
type SizeSpec struct {
	CPU           int    `json:"cpu"`
	MemoryGB      int    `json:"memory_gb"`
	InstanceType  string `json:"instance_type"`
	DefaultDiskGB int    `json:"default_disk_gb"`
	DiskType      string `json:"disk_type"`
}

// Sizes in order.
var Sizes = []string{"XS", "S", "M", "L", "XL"}

// StroppyCatalog is system.stroppy_catalog@1.
type StroppyCatalog struct {
	Source   string           `json:"source"`
	Versions []StroppyVersion `json:"versions"`
}

// StroppyVersion is one build.
type StroppyVersion struct {
	Version    string          `json:"version"`
	Image      string          `json:"image"`
	Default    bool            `json:"default"`
	Deprecated bool            `json:"deprecated"`
	Baseline   bool            `json:"baseline"`
	Protocols  []Protocol      `json:"protocols"`
	Scripts    []StroppyScript `json:"scripts"`
}

// StroppyScript is one workload script.
type StroppyScript struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Protocols   []Protocol     `json:"protocols,omitempty"`
	Steps       []StroppyStep  `json:"steps"`
	Params      []StroppyParam `json:"params,omitempty"`
}

// StroppyStep is one step of a script.
type StroppyStep struct {
	ID          string `json:"id"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Phase       string `json:"phase,omitempty"`
}

// StroppyParam is one typed flag of a script.
type StroppyParam struct {
	Name               string `json:"name"`
	Config             string `json:"config"`
	Scope              string `json:"scope,omitempty"`
	Type               string `json:"type"`
	Description        string `json:"description,omitempty"`
	Default            any    `json:"default,omitempty"`
	DefaultDescription string `json:"default_description,omitempty"`
	Env                string `json:"env,omitempty"`
}

// Metric is one cataloged metric.
type Metric struct {
	Key            string
	Title          string
	Description    string
	Unit           string
	HigherIsBetter bool
	Group          string
	Scope          string // result | db | host
	DBKinds        []DatabaseKind
	RatingEligible bool
	// Expr is the MetricsQL of the metric's time series: `$run` stands for
	// the matchers of Graphene's attributes (scraped component series),
	// `$native` for Stroppy's own spelling (workload series). Empty: the
	// metric is a number of the result only, with no series.
	Expr string
}
