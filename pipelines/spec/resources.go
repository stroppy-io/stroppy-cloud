package spec

import (
	"encoding/json"
	"fmt"
	"math"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ResourceIssue is a pre-provision finding, reusable by library previews and CLI.
type ResourceIssue struct {
	Scope    string `json:"scope,omitempty"`
	Path     string `json:"path"`
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// DatasetEstimate is a planning estimate, not a physical-size guarantee. It
// excludes workload-dependent growth and compression. Separate workload families
// can coexist; repeated loads of the same family use its largest population.
type DatasetEstimate struct {
	DataGB        float64  `json:"data_gb"`
	RecommendedGB int      `json:"recommended_gb"`
	Unknown       []string `json:"unknown,omitempty"`
}

// EstimateDataset follows built-in Stroppy populations (TPC-C warehouses,
// TPC-B 100k accounts/branch, TPC-H/DS GB scale). Values are deliberately planning
// budgets; custom SQL and indexes can change the physical footprint substantially.
func EstimateDataset(raw []json.RawMessage) DatasetEstimate {
	e := DatasetEstimate{}
	families := map[string]float64{}
	for _, r := range raw {
		var s Segment
		if json.Unmarshal(r, &s) != nil {
			e.Unknown = append(e.Unknown, "invalid segment")
			continue
		}
		family := strings.Split(s.Workload.Script, "/")[0]
		sf := paramNumber(s.Workload.Params, "scale_factor", 1)
		var gb float64
		switch family {
		case "tpcc":
			gb = sf * 0.1
		case "tpcb":
			gb = sf * 0.015
		case "tpch":
			gb = sf
		case "tpcds":
			gb = math.Max(sf, 0.25) // fixed dimensions survive fractional scale
		case "baseline":
			gb = paramNumber(s.Workload.Params, "rows", 250000) * 256 / (1 << 30)
		default:
			e.Unknown = append(e.Unknown, s.Name)
			continue
		}
		if custom, ok := s.Workload.Params["schema_file"].(string); ok && custom != "" {
			e.Unknown = append(e.Unknown, s.Name+": custom schema")
		}
		if custom, ok := s.Workload.Params["sql_file"].(string); ok && custom != "" {
			e.Unknown = append(e.Unknown, s.Name+": custom SQL")
		}
		families[family] = math.Max(families[family], gb)
	}
	keys := make([]string, 0, len(families))
	for k := range families {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		e.DataGB += families[k]
	}
	if e.DataGB > 0 {
		e.RecommendedGB = int(math.Ceil(e.DataGB*3 + 2))
	}
	return e
}

func paramNumber(p map[string]any, key string, fallback float64) float64 {
	switch n := p[key].(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case uint64:
		return float64(n)
	case int:
		return float64(n)
	}
	return fallback
}

// ResourceErrors formats only paths/reasons, never raw configuration or secrets.
func ResourceErrors(issues []ResourceIssue) error {
	for _, i := range issues {
		if i.Severity == "ERROR" {
			return &ValidationError{Issues: slices.Clone(issues)}
		}
	}
	return nil
}

// CheckResources runs after recipe and raw-file overlays, before cloud resources.
func CheckResources(r Run) []ResourceIssue {
	var issues []ResourceIssue
	add := func(p, code, severity, msg string) {
		issues = append(issues, ResourceIssue{Scope: "run_spec", Path: p, Code: code, Severity: severity, Message: msg})
	}
	machines := r.MachineByName()
	for i, m := range r.Machines {
		p := fmt.Sprintf("machines[%d]", i)
		if r.Provider.Kind == ProviderYandex {
			checkYandexMachine(m, p, add)
		}
		if m.CPU < 1 || m.MemoryGB < 1 {
			add(p, "hardware", "ERROR", "CPU and memory must be positive")
		}
		if r.Provider.Kind == ProviderAWS && m.CoreFraction != 0 {
			add(p+".core_fraction", "provider_option", "ERROR", "core_fraction is only supported by YC")
		}
		if m.BootDisk != nil {
			checkDisk(r.Provider.Kind, Disk{GB: m.BootDisk.GB, Type: m.BootDisk.Type, BlockSize: m.BootDisk.BlockSize}, p+".boot_disk", add)
		}
		names, mounts := map[string]bool{}, map[string]bool{}
		for j, d := range m.Disks {
			dp := fmt.Sprintf("%s.disks[%d]", p, j)
			checkDisk(r.Provider.Kind, d, dp, add)
			if (d.Filesystem != "" || len(d.MountOptions) > 0) && d.Mount == "" {
				add(dp, "filesystem_mount", "ERROR", "filesystem and mount options require a mount point; omit them for a raw disk")
			}
			if r.Provider.Kind == ProviderAWS && (d.Filesystem != "" || len(d.MountOptions) > 0) {
				add(dp, "provider_option", "ERROR", "automatic filesystem configuration currently requires YC; AWS needs explicit host preparation")
			}
			if names[d.Name] {
				add(dp+".name", "duplicate", "ERROR", "disk names must be unique on a machine")
			}
			names[d.Name] = true
			if d.GB < 1 {
				add(dp+".gb", "disk_size", "ERROR", "disk size must be positive")
			}
			if d.Mount != "" {
				if !safeDataMount(d.Mount) {
					add(dp+".mount", "unsafe_mount", "ERROR", "mount must be a canonical absolute data path outside OS directories")
				}
				if mounts[d.Mount] {
					add(dp+".mount", "duplicate", "ERROR", "disk mount points must be unique")
				}
				mounts[d.Mount] = true
			}
		}
		for a := range mounts {
			for b := range mounts {
				if a != b && strings.HasPrefix(b, a+"/") {
					add(p+".disks", "overlapping_mounts", "ERROR", "nested disk mounts require an explicit host preparation layout; automatic mounts must not overlap")
				}
			}
		}
	}
	for i, h := range r.HostPrep {
		if h.Machine != "" {
			m, ok := machines[h.Machine]
			if !ok || m.Role != h.Role {
				add(fmt.Sprintf("host_prep[%d]", i), "selector", "ERROR", "machine must exist and belong to the selected role")
			}
		}
	}
	portOwners := map[string]string{}
	for i, c := range r.Containers {
		p := fmt.Sprintf("containers[%d]", i)
		seen := map[string]bool{}
		for _, f := range c.Files {
			if !canonicalAbsolute(f.Path) || f.Path == "/" {
				add(p+".files", "file_path", "ERROR", "file paths must be canonical absolute paths")
			}
			if seen[f.Path] {
				add(p+".files", "duplicate", "ERROR", "file paths must be unique per container")
			}
			seen[f.Path] = true
		}
		targets := map[string]bool{}
		for _, m := range c.Mounts {
			if !canonicalAbsolute(m.Source) || !canonicalAbsolute(m.Target) {
				add(p+".mounts", "mount_path", "ERROR", "mount paths must be canonical absolute paths")
			}
			if targets[m.Target] {
				add(p+".mounts", "duplicate", "ERROR", "mount targets must be unique per container")
			}
			targets[m.Target] = true
		}
		for _, port := range c.Ports {
			key := fmt.Sprintf("%s:%d", c.Machine, port.Host)
			if owner, ok := portOwners[key]; ok {
				add(p+".ports", "port_conflict", "ERROR", "host port already declared by "+owner)
			}
			portOwners[key] = c.Name
		}
		checkMemory(c, machines[c.Machine], p, add)
	}
	if r.Workload.DriverType != "noop" {
		e := EstimateDataset(r.Workload.Segments)
		if len(e.Unknown) > 0 {
			add("workload.segments", "capacity_unknown", "WARNING", "custom SQL/simple or schema overrides prevent a complete storage estimate; built-in population estimates do not cover arbitrary writes")
		}
		dbs := databaseStorage(r)
		if r.ManagedYDB != nil && r.ManagedYDB.Type == "serverless" {
			dbs = map[string]float64{"managed_ydb": float64(r.ManagedYDB.StorageSizeLimitGB)}
		}
		if len(dbs) == 0 && e.DataGB > 0 {
			add("workload.segments", "capacity_external", "WARNING", "database storage capacity is not available locally; verify external/managed storage limits")
		}
		distributed := false
		for _, c := range r.Containers {
			image := strings.ToLower(c.Image)
			if strings.Contains(image, "cockroach") || strings.Contains(image, "picodata") || strings.Contains(image, "ydb") {
				distributed = true
			}
		}
		available := 0.0
		names := make([]string, 0, len(dbs))
		for name := range dbs {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			gb := dbs[name]
			if distributed {
				available += gb
				continue
			}
			storagePath := "machines"
			for index, m := range r.Machines {
				if m.Name == name {
					storagePath = fmt.Sprintf("machines[%d]", index)
				}
			}
			if name == "managed_ydb" {
				storagePath = "managed_ydb.storage_size_limit_gb"
			}
			checkStorage(storagePath, gb, e, add)
		}
		if distributed && len(dbs) > 0 {
			checkStorage("machines", available, e, add)
			add("machines", "replication_budget", "WARNING", "distributed storage estimate uses aggregate capacity; replication, shard placement and per-node imbalance need additional headroom")
		}
	}
	slices.SortFunc(issues, func(a, b ResourceIssue) int {
		return strings.Compare(a.Path+"/"+a.Code+"/"+a.Message, b.Path+"/"+b.Code+"/"+b.Message)
	})
	return issues
}

func checkStorage(field string, gb float64, e DatasetEstimate, add func(string, string, string, string)) {
	if e.DataGB > gb*0.8 {
		add(field, "capacity_insufficient", "ERROR", fmt.Sprintf("built-in data estimate %.2f GiB exceeds the 80%% storage budget of %.2f GiB; provision at least %d GiB including index/WAL/temp headroom", e.DataGB, gb, e.RecommendedGB))
	} else if float64(e.RecommendedGB) > gb {
		add(field, "capacity_headroom", "WARNING", fmt.Sprintf("%.2f GiB storage is below the recommended %d GiB; indexes, WAL and temporary data can fill it", gb, e.RecommendedGB))
	}
}
func canonicalAbsolute(p string) bool { return strings.HasPrefix(p, "/") && path.Clean(p) == p }
func safeDataMount(p string) bool {
	if !canonicalAbsolute(p) || p == "/" {
		return false
	}
	for _, root := range []string{"/boot", "/dev", "/proc", "/sys", "/etc", "/usr", "/bin", "/sbin", "/lib", "/lib64", "/run"} {
		if p == root || strings.HasPrefix(p, root+"/") {
			return false
		}
	}
	return p != "/var" && p != "/home" && p != "/root" && p != "/tmp"
}

// Match each writable DB bind to the deepest configured disk mount; absent bind
// or a path outside data disks consumes boot storage with 10 GiB reserved for OS.
func databaseStorage(r Run) map[string]float64 {
	out := map[string]float64{}
	for _, c := range r.Containers {
		if strings.Contains(c.Name, "exporter") || (c.Role != "db" && !strings.HasPrefix(c.Role, "db-")) {
			continue
		}
		m, ok := r.MachineByName()[c.Machine]
		if !ok {
			continue
		}
		boot := 40
		if m.BootDisk != nil {
			boot = m.BootDisk.GB
		}
		capacity := float64(max(0, boot-10))
		for _, mount := range c.Mounts {
			if mount.RO {
				continue
			}
			longest := 0
			for _, d := range m.Disks {
				if d.Mount != "" && (mount.Source == d.Mount || strings.HasPrefix(mount.Source, d.Mount+"/")) && len(d.Mount) > longest {
					capacity = float64(d.GB)
					longest = len(d.Mount)
				}
			}
		}
		if old, ok := out[m.Name]; !ok || capacity < old {
			out[m.Name] = capacity
		}
	}
	return out
}

var memorySetting = regexp.MustCompile(`(?mi)^\s*(shared_buffers|innodb_buffer_pool_size|memtx_memory)\s*[=:]\s*['"]?([0-9]+(?:\.[0-9]+)?)\s*([kKmMgGtT](?:[iI]?[bB])?)?`)

func checkMemory(c Container, m Machine, p string, add func(string, string, string, string)) {
	values := map[string]float64{}
	for _, f := range c.Files {
		for _, hit := range memorySetting.FindAllStringSubmatch(f.Content, -1) {
			n, err := strconv.ParseFloat(hit[2], 64)
			if err != nil {
				continue
			}
			unit := strings.ToLower(hit[3])
			factor := 1.0
			if unit != "" {
				switch unit[0] {
				case 'k':
					factor = 1 << 10
				case 'm':
					factor = 1 << 20
				case 'g':
					factor = 1 << 30
				case 't':
					factor = 1 << 40
				}
			} else if hit[1] == "shared_buffers" {
				factor = 8192
			}
			values[hit[1]] = n * factor
		}
	}
	total := 0.0
	for _, key := range []string{"shared_buffers", "innodb_buffer_pool_size", "memtx_memory"} {
		total += values[key]
	}
	if total > float64(m.MemoryGB)*(1<<30) {
		add(p+".files", "memory_overcommit", "ERROR", "configured fixed database buffers exceed machine RAM")
	}
	if total > float64(m.MemoryGB)*(1<<30)*0.8 && total <= float64(m.MemoryGB)*(1<<30) {
		add(p+".files", "memory_headroom", "WARNING", "fixed buffers consume over 80% of machine RAM; connections, processes and OS also need memory")
	}
}

// NeedsPrivateEgress reports whether at least one VM needs a shared NAT gateway.
func NeedsPrivateEgress(r Run) bool {
	for _, m := range r.Machines {
		public := r.Network.AllowPublicIPs
		if m.PublicIP != nil {
			public = *m.PublicIP
		}
		if !public {
			return true
		}
	}
	return false
}
