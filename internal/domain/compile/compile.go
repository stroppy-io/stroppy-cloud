// Package compile turns a resolved test (database plan, workload, sizes,
// provider profile) into the RunSpec the stroppy-run pipeline executes:
// machines from the size table, containers from per-kind recipes, config
// files rendered through the schemapb templates, flows for security
// groups, the stroppy workload with its driver URL. The output is baked
// through spec.run@1 by the caller; nothing here talks to a cloud.
package compile

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Renderer renders a config value through a schema template.
type Renderer interface {
	Render(ctx context.Context, schemaID, template string, value json.RawMessage) (string, error)
	Bake(ctx context.Context, schemaID string, value json.RawMessage) (json.RawMessage, error)
}

// Input is everything the compiler needs.
type Input struct {
	RunID    uuid.UUID
	Tenant   string
	Database library.DatabaseSpec
	Plan     topology.Plan
	// EffectiveConfigs: role → schema id → resolved value.
	EffectiveConfigs map[string]map[string]json.RawMessage
	Workload         library.WorkloadSpec
	WorkloadBaked    json.RawMessage
	Sizes            map[string]library.RoleSize
	Provider         catalog.Provider
	ProviderKind     string
	ProviderSettings json.RawMessage
	// CredentialsSecret is the Graphene secret of the profile.
	CredentialsSecret string
	// ProviderConfigName is the Crossplane ProviderConfig of the tenant.
	ProviderConfigName string
	StroppyImage       string
	Keep               time.Duration
	Observability      spec.Observability
	Catalog            *catalog.Catalog
	Labels             map[string]string
}

// Output is the compiled run.
type Output struct {
	Spec     spec.Run
	Machines []run.MachineSnapshot
	// ClientURL is the stroppy driver URL (with placeholders).
	ClientURL string
}

// Compile builds the RunSpec.
func Compile(ctx context.Context, r Renderer, in Input) (Output, error) {
	c := &compilation{ctx: ctx, r: r, in: in, out: Output{}}
	recipe, ok := recipes[in.Database.Kind]
	if !ok {
		return Output{}, errs.Newf(errs.CodeInvalid, "database kind %s cannot be launched yet", in.Database.Kind)
	}
	if err := c.machines(); err != nil {
		return Output{}, err
	}
	c.prepareDataDisks()
	c.network()
	c.flows()
	if err := recipe(c); err != nil {
		return Output{}, err
	}
	c.exporters()
	if err := c.workload(); err != nil {
		return Output{}, err
	}
	c.out.Spec.RunID = in.RunID.String()
	c.out.Spec.Tenant = in.Tenant
	c.out.Spec.Provider = spec.Provider{
		Kind: spec.ProviderKind(in.ProviderKind), Settings: in.ProviderSettings,
		CredentialsSecret: in.CredentialsSecret, ProviderConfigName: in.ProviderConfigName,
	}
	c.out.Spec.Observability = in.Observability
	c.out.Spec.Keep = spec.Duration(in.Keep)
	// Every measuring workload reports iterations. A generic TPS is not part
	// of Stroppy 6's output contract and must not make every run degraded.
	c.out.Spec.ResultExpectations = []string{"iterations_total"}
	sort.SliceStable(c.out.Spec.Containers, func(i, j int) bool { return c.out.Spec.Containers[i].Name < c.out.Spec.Containers[j].Name })
	return c.out, nil
}

// compilation is the in-progress build.
type compilation struct {
	ctx context.Context
	r   Renderer
	in  Input
	out Output
	// machinesByRole: role → machine names in order.
	machinesByRole map[string][]string
}

// --- machines ---------------------------------------------------------------

func (c *compilation) machines() error {
	c.machinesByRole = map[string][]string{}
	settings := map[string]any{}
	_ = json.Unmarshal(c.in.ProviderSettings, &settings) //nolint:errcheck // baked upstream
	zones, err := c.placementZones(settings)
	if err != nil {
		return err
	}
	for _, node := range c.in.Plan.Nodes {
		if node.ColocatedWith != "" {
			continue
		}
		rs, ok := c.in.Sizes[node.Role]
		if !ok {
			return errs.Newf(errs.CodeInvalid, "sizes.%s: no size chosen", node.Role)
		}
		table, ok := c.in.Provider.Sizes[topology.Family(node.Role)]
		if !ok {
			table = c.in.Provider.Sizes[topology.RoleProxy]
		}
		cell, ok := table[rs.Size]
		if !ok {
			return errs.Newf(errs.CodeInvalid, "sizes.%s: %s has no %s size for %s", node.Role, c.in.Provider.Kind, rs.Size, topology.Family(node.Role))
		}
		disk := cell.DefaultDiskGB
		if rs.DiskGB > 0 {
			disk = rs.DiskGB
		}
		if c.in.Database.Kind == catalog.YDB && node.Role == topology.RoleDB {
			minimum := intParam(c.params(), "pdisks_per_node", 1)*ydbPdiskGB + 20
			if disk < minimum {
				if rs.DiskGB > 0 {
					return errs.Newf(errs.CodeInvalid, "sizes.%s.disk_gb: need at least %d GB for YDB pdisks and filesystem headroom", node.Role, minimum)
				}
				disk = minimum
			}
		}
		diskType := cell.DiskType
		if rs.DiskType != "" {
			diskType = rs.DiskType
		}
		for i := 1; i <= node.Count; i++ {
			name := fmt.Sprintf("%s-%d", node.Role, i)
			m := spec.Machine{
				Name: name, Role: node.Role, CPU: cell.CPU, MemoryGB: cell.MemoryGB, InstanceType: cell.InstanceType,
				Image: c.image(settings), Location: c.location(settings),
				Labels: map[string]string{"stroppy.io/role": node.Role, "stroppy.io/engine": node.Engine},
			}
			if len(zones) > 1 && (node.Role == topology.RoleDB || node.Role == topology.RoleDBCompute) {
				m.Location = zones[(i-1)%len(zones)]
			}
			if node.Role != topology.RoleRunner && node.Role != topology.RoleProxy {
				m.Disks = []spec.Disk{{Name: "data", GB: disk, Type: diskType, Mount: "/data"}}
			}
			c.out.Spec.Machines = append(c.out.Spec.Machines, m)
			c.machinesByRole[node.Role] = append(c.machinesByRole[node.Role], name)
			snap := run.MachineSnapshot{Name: name, Role: node.Role, Size: rs.Size, CPU: cell.CPU, MemoryGB: cell.MemoryGB, InstanceType: cell.InstanceType, Location: m.Location}
			if len(m.Disks) > 0 {
				snap.DiskGB, snap.DiskType = disk, diskType
			}
			c.out.Machines = append(c.out.Machines, snap)
		}
	}
	return nil
}

func (c *compilation) image(settings map[string]any) string {
	for _, key := range []string{"image_family", "ami_family"} {
		if v, ok := settings[key].(string); ok && v != "" {
			return v
		}
	}
	if len(c.in.Provider.Images) > 0 {
		return c.in.Provider.Images[0].ID
	}
	return "ubuntu-2404-lts"
}

func (c *compilation) location(settings map[string]any) string {
	for _, key := range []string{"zone", "availability_zone", "region"} {
		if v, ok := settings[key].(string); ok && v != "" {
			return v
		}
	}
	if len(c.in.Provider.Locations) > 0 {
		return c.in.Provider.Locations[0].ID
	}
	return ""
}

// --- network ----------------------------------------------------------------

func (c *compilation) network() {
	settings := map[string]any{}
	_ = json.Unmarshal(c.in.ProviderSettings, &settings) //nolint:errcheck // baked upstream
	n := spec.Network{CIDR: "10.130.0.0/24"}
	for _, key := range []string{"subnet_cidr", "cidr"} {
		if cidr, ok := settings[key].(string); ok && cidr != "" {
			n.CIDR = cidr
		}
	}
	if v, ok := settings["public_ips"].(bool); ok {
		n.AllowPublicIPs = v
	}
	c.out.Spec.Network = n
}

// flows: the plan's edges as security-group rules. The plan's protocol
// names (pg, mysql-replication, …) become the flow label; the wire
// protocol is tcp for every database port.
func (c *compilation) flows() {
	for _, f := range c.in.Plan.Flows {
		if f.Port <= 0 {
			continue
		}
		proto := "tcp"
		if strings.HasPrefix(f.Protocol, "ydb_grpc") {
			proto = "grpc"
		}
		c.out.Spec.Flows = append(c.out.Spec.Flows, spec.Flow{FromRole: f.From, ToRole: f.To, Protocol: proto, Port: f.Port, Label: f.Protocol})
	}
}

// --- workload ---------------------------------------------------------------

// workload renders the stroppy side: image, driver type, URL with the
// client placeholder, driver options in lowerCamel, segments, baseline.
func (c *compilation) workload() error {
	ver, ok := c.in.Catalog.StroppyVersion(c.in.Workload.StroppyVersion)
	if !ok {
		return errs.Newf(errs.CodeInvalid, "unknown stroppy version %q", c.in.Workload.StroppyVersion)
	}
	image := c.in.StroppyImage
	if image == "" {
		image = ver.Image
	}
	var baked struct {
		Segments   []json.RawMessage `json:"segments"`
		Driver     map[string]any    `json:"driver"`
		Connection map[string]any    `json:"connection"`
		Baseline   map[string]any    `json:"baseline"`
	}
	if err := json.Unmarshal(c.in.WorkloadBaked, &baked); err != nil {
		return errs.Wrap(errs.CodeInvalid, "workload value", err)
	}
	if c.in.Workload.Protocol == catalog.ProtoCockroach {
		for i, raw := range baked.Segments {
			segment, err := cockroachSQL(raw, c.in.Database.Version)
			if err != nil {
				return errs.Wrap(errs.CodeInvalid, "cockroach workload segment", err)
			}
			baked.Segments[i] = segment
		}
	}
	url, driverType := c.clientURL(baked.Connection)
	driver := map[string]any{}
	if c.in.Workload.Protocol == catalog.ProtoPicodata {
		driver["postgres"] = map[string]any{"defaultQueryExecMode": "exec"}
	}
	for k, v := range baked.Driver {
		if k == "pool" || k == "insert_progress" {
			driver[camel(k)] = camelMap(v)
			continue
		}
		driver[camel(k)] = v
	}
	for k, v := range baked.Connection {
		if k == "kind" {
			continue
		}
		if k == "query_exec_mode" {
			driver["postgres"] = map[string]any{"defaultQueryExecMode": v}
			continue
		}
		driver[camel(k)] = v
	}
	w := spec.Workload{RunnerRole: topology.RoleRunner, StroppyImage: image, DriverType: driverType, URL: url, Driver: driver, Segments: baked.Segments}
	if ca, ok := baked.Connection["ca_cert"].(string); ok && ca != "" {
		w.CACert = ca
		delete(driver, "caCert")
	}
	if enabled, _ := baked.Baseline["enabled"].(bool); enabled { //nolint:errcheck // absent = off
		b := &spec.Baseline{Enabled: true}
		if tiers, ok := baked.Baseline["tiers"].([]any); ok {
			for _, t := range tiers {
				b.Tiers = append(b.Tiers, fmt.Sprint(t))
			}
		}
		b.Quick, _ = baked.Baseline["quick"].(bool) //nolint:errcheck // absent = false
		b.VUs = int64(numberOf(baked.Baseline["vus"]))
		b.Rows = int64(numberOf(baked.Baseline["rows"]))
		if d, ok := baked.Baseline["duration"].(string); ok {
			if dd, err := time.ParseDuration(d); err == nil {
				b.Duration = spec.Duration(dd)
			}
		}
		w.Baseline = b
	}
	if c.out.Spec.ManagedYDB != nil {
		w.YDBIAMCredentialsSecret = c.in.CredentialsSecret
		w.URL = "grpcs://managed-ydb-pending:2135/?database=/pending"
		url = w.URL
	}
	c.out.Spec.Workload = w
	c.out.ClientURL = url
	return nil
}

// clientURL is the driver URL per protocol; the host is a placeholder the
// pipeline resolves after provisioning.
func (c *compilation) clientURL(conn map[string]any) (url, driverType string) {
	if c.in.Database.Kind == catalog.External && c.in.Database.ExternalDSN != "" {
		return c.in.Database.ExternalDSN, driverTypeOf(c.in.Workload.Protocol)
	}
	ep := c.in.Plan.Client
	host := "127.0.0.1"
	if ep.Role != "" {
		host = "${ip:role:" + ep.Role + "}"
	}
	proto := c.in.Workload.Protocol
	switch proto { //nolint:exhaustive // pg is the default branch
	case catalog.ProtoNoop:
		return "noop://localhost", "noop"
	case catalog.ProtoMySQL:
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", mysqlUser, mysqlPassword, host, ep.Port, mysqlDatabase), "mysql"
	case catalog.ProtoPicodata:
		return fmt.Sprintf("postgres://%s:%s@%s:%d?sslmode=disable", picodataUser, picodataPassword, host, ep.Port), "picodata"
	case catalog.ProtoYDBGrpc:
		return fmt.Sprintf("grpc://%s:%d%s", host, ep.Port, strParam(c.params(), "database_path", ydbDatabasePath)), "ydb"
	case catalog.ProtoYDBGrpcs:
		return fmt.Sprintf("grpcs://%s:%d%s", host, ep.Port, strParam(c.params(), "database_path", ydbDatabasePath)), "ydb"
	case catalog.ProtoCockroach:
		return fmt.Sprintf("postgresql://root@%s:%d/%s?sslmode=disable", host, ep.Port, cockroachDatabase), "postgres"
	default: // catalog.ProtoPg and anything external
		sslmode := "disable"
		if v, ok := conn["sslmode"].(string); ok && v != "" {
			sslmode = v
		}
		if c.in.Database.Kind == catalog.PgNoop {
			return fmt.Sprintf("postgresql://%s@%s:%d/postgres?sslmode=%s", pgUser, host, ep.Port, sslmode), "postgres"
		}
		return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=%s", pgUser, pgPassword, host, ep.Port, pgDatabase, sslmode), "postgres"
	}
}

func driverTypeOf(p catalog.Protocol) string {
	switch p {
	case catalog.ProtoMySQL:
		return "mysql"
	case catalog.ProtoPicodata:
		return "picodata"
	case catalog.ProtoYDBGrpc, catalog.ProtoYDBGrpcs:
		return "ydb"
	case catalog.ProtoNoop:
		return "noop"
	case catalog.ProtoPg, catalog.ProtoCockroach:
		return "postgres"
	default:
		return "postgres"
	}
}

// --- helpers ----------------------------------------------------------------

// Well-known credentials and names of the deployed databases (the
// benchmark stand is throwaway; nothing here is a secret).
//
//nolint:gosec // G101: fixed benchmark-stand passwords, not credentials
const (
	pgUser, pgPassword, pgDatabase = "postgres", "stroppy_postgres", "postgres"
	replUser, replPassword         = "replicator", "stroppy_replication"
	mysqlUser, mysqlPassword       = "root", "stroppy_mysql"
	mysqlDatabase                  = "stroppy"
	picodataUser, picodataPassword = "admin", "T0psecret"
	ydbDatabasePath                = "/Root/testdb"
	cockroachDatabase              = "defaultdb"
	dataMount                      = "/data"
	pgDataDir                      = dataMount + "/pg"
	confDir                        = "/etc/stroppy"

	nodeExporterPort, postgresExporterPort = 9100, 9187
	mysqldExporterPort                     = 9104
)

// ip is the placeholder of a machine's private address.
func ip(machine string) string { return "${ip:" + machine + "}" }

// render bakes the effective config of a role/schema with the cluster
// overlay and renders its template.
func (c *compilation) render(role, schemaID, template string, overlay map[string]any) (string, error) {
	base := map[string]any{}
	if rc, ok := c.in.EffectiveConfigs[role]; ok {
		if v, ok := rc[schemaID]; ok && len(v) > 0 {
			_ = json.Unmarshal(v, &base) //nolint:errcheck // baked upstream
		}
	}
	for k, v := range overlay {
		base[k] = v
	}
	raw, err := json.Marshal(base)
	if err != nil {
		return "", err
	}
	out, err := c.r.Render(c.ctx, schemaID, template, raw)
	if err != nil {
		if e, ok := errs.AsValidation(err); ok {
			e.Detail = fmt.Sprintf("configs.%s.%s: %s", role, schemaID, e.Detail)
		}
		return "", err
	}
	return out, nil
}

// effective returns the resolved config value of a role/schema as a map.
func (c *compilation) effective(role, schemaID string) map[string]any {
	out := map[string]any{}
	if rc, ok := c.in.EffectiveConfigs[role]; ok {
		if v, ok := rc[schemaID]; ok {
			_ = json.Unmarshal(v, &out) //nolint:errcheck // baked upstream
		}
	}
	return out
}

// schemaFor finds the config schema id of a role by prefix (versioned ids).
func (c *compilation) schemaFor(role, prefix string) string {
	kind, ok := c.in.Catalog.Database(c.in.Database.Kind)
	if !ok {
		return ""
	}
	for _, r := range kind.Roles {
		if r.Role != role {
			continue
		}
		for _, id := range r.ConfigSchemas {
			if strings.HasPrefix(id, prefix) {
				return kind.ConfigSchema(c.in.Database.Version, id)
			}
		}
	}
	return ""
}

func (c *compilation) params() map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal(c.in.Database.Params, &out) //nolint:errcheck // baked upstream
	return out
}

func (c *compilation) add(containers ...spec.Container) {
	c.out.Spec.Containers = append(c.out.Spec.Containers, containers...)
}

// exporters adds node_exporter on every machine (the database exporters
// live in the recipes).
func (c *compilation) exporters() {
	for _, m := range c.out.Spec.Machines {
		name := m.Name + "-node-exporter"
		c.add(spec.Container{
			Name: name, Role: m.Role, Machine: m.Name, Image: imageNodeExporter,
			Cmd: []string{
				"--path.rootfs=/host", "--path.procfs=/host/proc", "--path.sysfs=/host/sys",
				"--path.udev.data=/host/run/udev/data", fmt.Sprintf("--web.listen-address=:%d", nodeExporterPort),
			},
			Mounts:  []spec.Mount{{Source: "/", Target: "/host", RO: true}},
			Ports:   []spec.Port{{Container: nodeExporterPort, Host: nodeExporterPort}},
			Scrape:  "/metrics",
			Restart: "always",
		})
	}
}

func camel(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

func camelMap(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := map[string]any{}
	for k, x := range m {
		out[camel(k)] = x
	}
	return out
}

func numberOf(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	}
	return 0
}

func intParam(p map[string]any, key string, def int) int {
	if v := numberOf(p[key]); v != 0 {
		return int(v)
	}
	if _, ok := p[key]; ok {
		return int(numberOf(p[key]))
	}
	return def
}

func strParam(p map[string]any, key, def string) string {
	if v, ok := p[key].(string); ok && v != "" {
		return v
	}
	return def
}

func boolParam(p map[string]any, key string) bool {
	v, _ := p[key].(bool) //nolint:errcheck // absent = false
	return v
}
