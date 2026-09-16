// Package spec is the Go form of what the pipelines receive and return: the
// RunSpec (spec.run@1), the RunResult (spec.result.run@1), the SuiteSpec
// (spec.suite@1) and the service-pipeline params. The server compiles a
// library Test into a Run; the pipeline executes it and never resolves
// anything itself. Secrets are NAMES of Graphene secrets, never values.
//
// The JSON shape is fixed by the schemapb schemas in schemas/spec — the test
// bakes sample values of these types through them, so a drift between the
// two fails at test time, not on a run.
package spec

import (
	"encoding/json"
	"time"
)

// ProviderKind is the cloud a run is created in.
type ProviderKind string

// Provider kinds.
const (
	ProviderYandex ProviderKind = "yandex"
	ProviderAWS    ProviderKind = "aws"
)

// Run is the RunSpec: a fully resolved, self-contained description of one
// benchmark run.
type Run struct {
	RunID    string   `json:"run_id"`
	Tenant   string   `json:"tenant"`
	Provider Provider `json:"provider"`
	Network  Network  `json:"network"`
	// Machines are every VM of the run; the pipeline creates one agent per
	// machine.
	Machines   []Machine   `json:"machines"`
	ManagedYDB *ManagedYDB `json:"managed_ydb,omitempty"`
	// Containers are everything that runs on the machines: databases,
	// proxies, exporters.
	Containers []Container `json:"containers,omitempty"`
	// HostPrep are rare pre-deploy steps run on the machine itself.
	HostPrep []HostPrep `json:"host_prep,omitempty"`
	// Scrapes are agent-side Prometheus scrapes of exporters.
	Scrapes []Scrape `json:"scrapes,omitempty"`
	// Flows describe role relationships in the topology view; ingress openings
	// are controlled by Network.Ingress.
	Flows         []Flow        `json:"flows,omitempty"`
	Workload      Workload      `json:"workload"`
	Observability Observability `json:"observability,omitempty,omitzero"`
	// Keep keeps the stand alive after the run; 0 tears everything down.
	Keep Duration `json:"keep,omitempty"`
	// ResultExpectations are metric keys the run must report.
	ResultExpectations []string `json:"result_expectations,omitempty"`
}

// Provider is where the run is created and with which credentials.
type Provider struct {
	Kind ProviderKind `json:"kind"`
	// Settings is the baked provider.<kind>.settings value.
	Settings json.RawMessage `json:"settings"`
	// CredentialsSecret names the Graphene secret with the credentials.
	CredentialsSecret string `json:"credentials_secret"`
	// ProviderConfigName is the Crossplane ProviderConfig the managed
	// resources reference (t-<tenant>).
	ProviderConfigName string `json:"provider_config_name"`
	// RegistrySecret names the Graphene secret with a private docker
	// registry login, when images need one.
	RegistrySecret string `json:"registry_secret,omitempty"`
}

// Network is the network every machine of the run joins.
type Network struct {
	CIDR           string    `json:"cidr,omitempty"`
	AllowPublicIPs bool      `json:"allow_public_ips"`
	Ingress        []Ingress `json:"ingress,omitempty"`
}

// Ingress is one security-group opening beyond intra-network traffic.
type Ingress struct {
	Port  int    `json:"port"`
	Proto string `json:"proto,omitempty"`
	CIDR  string `json:"cidr"`
}

// Machine is one VM of the run.
type Machine struct {
	Name         string            `json:"name"`
	Role         string            `json:"role"`
	CPU          int               `json:"cpu"`
	MemoryGB     int               `json:"memory_gb"`
	BootDisk     *BootDisk         `json:"boot_disk,omitempty"`
	CoreFraction int               `json:"core_fraction,omitempty"`
	Preemptible  *bool             `json:"preemptible,omitempty"`
	PublicIP     *bool             `json:"public_ip,omitempty"`
	Disks        []Disk            `json:"disks,omitempty"`
	Image        string            `json:"image"`
	Location     string            `json:"location"`
	InstanceType string            `json:"instance_type"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// BootDisk configures the disposable OS disk. Omission uses 40 GiB SSD.
type BootDisk struct {
	BlockSize int    `json:"block_size,omitempty"`
	GB        int    `json:"gb"`
	Type      string `json:"type"`
}

// Disk is a secondary disk beyond the boot disk.
type Disk struct {
	Filesystem   string   `json:"filesystem,omitempty"`
	MountOptions []string `json:"mount_options,omitempty"`
	BlockSize    int      `json:"block_size,omitempty"`
	Name         string   `json:"name"`
	GB           int      `json:"gb"`
	Type         string   `json:"type"`
	Mount        string   `json:"mount,omitempty"`
}

// Container is one container on a machine.
type Container struct {
	Name        string            `json:"name"`
	Role        string            `json:"role"`
	Machine     string            `json:"machine"`
	Image       string            `json:"image"`
	Entrypoint  []string          `json:"entrypoint,omitempty"`
	Cmd         []string          `json:"cmd,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Ports       []Port            `json:"ports,omitempty"`
	Mounts      []Mount           `json:"mounts,omitempty"`
	Files       []File            `json:"files,omitempty"`
	Scrape      string            `json:"scrape,omitempty"`
	ScrapePort  int               `json:"scrape_port,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"`
	Healthcheck *Healthcheck      `json:"healthcheck,omitempty"`
	Restart     string            `json:"restart,omitempty"`
	Ulimits     map[string]int64  `json:"ulimits,omitempty"`
}

// Port declares a host-network port; Container and Host must be equal.
type Port struct {
	Container int `json:"container"`
	Host      int `json:"host"`
}

// Mount binds a host path into the container.
type Mount struct {
	Source string `json:"source"`
	Target string `json:"target"`
	RO     bool   `json:"ro,omitempty"`
}

// File is a rendered config injected into the container at start.
type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    string `json:"mode,omitempty"`
}

// Healthcheck is how the pipeline knows a container is up.
type Healthcheck struct {
	Cmd      []string `json:"cmd"`
	Interval Duration `json:"interval,omitempty"`
	Retries  int      `json:"retries,omitempty"`
}

// HostPrepKind is what a host_prep step does.
type HostPrepKind string

// Host prep kinds.
const (
	HostPrepSysctl HostPrepKind = "sysctl"
	HostPrepDisks  HostPrepKind = "disks"
	HostPrepScript HostPrepKind = "script"
)

// HostPrep is one pre-deploy step on every machine of a role.
type HostPrep struct {
	Role    string       `json:"role"`
	Machine string       `json:"machine,omitempty"`
	Kind    HostPrepKind `json:"kind"`
	Content string       `json:"content"`
}

// Scrape is an agent-side Prometheus scrape.
type Scrape struct {
	Role string `json:"role"`
	URL  string `json:"url"`
	Job  string `json:"job"`
}

// Flow is one allowed traffic edge.
type Flow struct {
	FromRole string `json:"from_role"`
	ToRole   string `json:"to_role,omitempty"`
	External string `json:"external,omitempty"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	Label    string `json:"label,omitempty"`
}

// Workload is what stroppy runs and where — the compiled form of
// workload.stroppy@1. The server resolves the version to an image and
// renders the connection URL and the stroppy driver options; the pipeline
// writes them into stroppy-config.json.
type Workload struct {
	RunnerRole   string `json:"runner_role"`
	StroppyImage string `json:"stroppy_image"`
	// DriverType is stroppy's driverType (postgres, mysql, picodata, ydb, noop).
	DriverType string `json:"driver_type"`
	// URL is drivers.0.url; may carry ${ip:...} placeholders.
	URL string `json:"url"`
	// Driver holds the remaining drivers.0 keys in stroppy's lowerCamel form
	// (bulkSize, pool, insertProgress, authToken, …).
	Driver map[string]any `json:"driver,omitempty"`
	// Segments are baked workload.segment@1 values in order.
	Segments []json.RawMessage `json:"segments"`
	// Baseline is the baked workload.stroppy@1 baseline object.
	Baseline *Baseline `json:"baseline,omitempty"`
	// CACert is a PEM the pipeline writes next to the config (caCertFile).
	CACert string `json:"ca_cert,omitempty"`
	// YDBIAMCredentialsSecret names provider.yandex.credentials; resolved only on the runner.
	YDBIAMCredentialsSecret string `json:"ydb_iam_credentials_secret,omitempty"`
}

// Baseline asks for `stroppy baseline` on the runner before the segments.
//
// doc: `stroppy baseline --help`.
type Baseline struct {
	Enabled  bool     `json:"enabled"`
	Tiers    []string `json:"tiers,omitempty"`
	Quick    bool     `json:"quick,omitempty"`
	VUs      int64    `json:"vus,omitempty"`
	Rows     int64    `json:"rows,omitempty"`
	Duration Duration `json:"duration,omitempty"`
}

// Observability is where run telemetry goes.
type Observability struct {
	OTLPEndpoint string `json:"otlp_endpoint,omitempty"`
	// OTLPHeaders is the comma-separated key=value list stroppy sends with
	// every export (tenant auth of the collector).
	OTLPHeaders string            `json:"otlp_headers,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// Duration is time.Duration with the schemapb wire form ("5m") in JSON.
type Duration time.Duration

// MarshalJSON renders the Go duration string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON accepts a duration string or a number of nanoseconds.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		v, err := time.ParseDuration(s)
		if err != nil {
			return err
		}
		*d = Duration(v)
		return nil
	}
	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*d = Duration(n)
	return nil
}

// Std is the time.Duration.
func (d Duration) Std() time.Duration { return time.Duration(d) }

// MachineByName indexes the machines.
func (r Run) MachineByName() map[string]Machine {
	out := make(map[string]Machine, len(r.Machines))
	for _, m := range r.Machines {
		out[m.Name] = m
	}
	return out
}

// MachinesByRole groups machines by role, keeping the spec order.
func (r Run) MachinesByRole() map[string][]Machine {
	out := map[string][]Machine{}
	for _, m := range r.Machines {
		out[m.Role] = append(out[m.Role], m)
	}
	return out
}

// ContainersOn lists the containers placed on one machine, in spec order.
func (r Run) ContainersOn(machine string) []Container {
	var out []Container
	for _, c := range r.Containers {
		if c.Machine == machine {
			out = append(out, c)
		}
	}
	return out
}

// ManagedYDB is a database owned by this run, alongside its runner VMs.
// Deletion protection is deliberately unavailable for disposable run resources.
type ManagedYDB struct {
	Zones               []string `json:"zones,omitempty"`
	Type                string   `json:"type"`
	LocationID          string   `json:"location_id"`
	ResourcePresetID    string   `json:"resource_preset_id,omitempty"`
	NodeCount           int      `json:"node_count,omitempty"`
	StorageGroups       int      `json:"storage_groups,omitempty"`
	StorageType         string   `json:"storage_type,omitempty"`
	AssignPublicIPs     bool     `json:"assign_public_ips,omitempty"`
	ThrottlingRCULimit  int      `json:"throttling_rcu_limit,omitempty"`
	ProvisionedRCULimit int      `json:"provisioned_rcu_limit,omitempty"`
	StorageSizeLimitGB  int      `json:"storage_size_limit_gb,omitempty"`
}
