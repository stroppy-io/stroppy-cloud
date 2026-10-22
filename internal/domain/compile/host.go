package compile

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// hostSysctlSchema renders the kernel drop-in of a database machine.
const hostSysctlSchema = "cfg.host.sysctl@1"

// dbNofile is the open-files limit of a database container: connections,
// tables, WAL segments and sockets of a cluster (Cockroach asks for at
// least 15000, MySQL derives open_files_limit from it).
//
// doc: docs.cockroachlabs.com/docs/stable/recommended-production-settings#file-descriptors-limit
const dbNofile = 1048576

// hostTuning is what a recipe asks of one database machine's kernel on top
// of cfg.host.sysctl@1 (zero fields keep the schema's values).
type hostTuning struct {
	// THP is transparent_hugepage: never (the schema default) or madvise.
	THP string
	// Overcommit is vm.overcommit_memory (nil = schema).
	Overcommit *int
	// HugePages is vm.nr_hugepages (2 MB pages).
	HugePages int
}

// tuneHost records a recipe's kernel wishes for a machine.
func (c *compilation) tuneHost(machine string, t hostTuning) {
	if c.hosts == nil {
		c.hosts = map[string]hostTuning{}
	}
	c.hosts[machine] = t
}

// prepareDatabaseHosts gives every machine that runs a database container
// the kernel drop-in and every database container its file limit, in the
// auto tuning only (stock leaves the machine as the image boots it).
func (c *compilation) prepareDatabaseHosts() error {
	if c.tuning() != tuningAuto {
		return nil
	}
	machines := map[string]string{} // machine → role
	for i := range c.out.Spec.Containers {
		ct := &c.out.Spec.Containers[i]
		if ct.Kind != spec.ContainerKindDatabase {
			continue
		}
		if _, ok := ct.Ulimits["nofile"]; !ok {
			if ct.Ulimits == nil {
				ct.Ulimits = map[string]int64{}
			}
			ct.Ulimits["nofile"] = dbNofile
		}
		machines[ct.Machine] = ct.Role
	}
	names := make([]string, 0, len(machines))
	for m := range machines {
		names = append(names, m)
	}
	sort.Strings(names)
	for _, m := range names {
		role := machines[m]
		t := c.hosts[m]
		value := map[string]any{}
		if v, ok := c.in.EffectiveConfigs[role][hostSysctlSchema]; ok {
			_ = json.Unmarshal(v, &value) //nolint:errcheck // baked upstream
		}
		explicit := c.explicitConfig(role, hostSysctlSchema)
		if t.THP != "" && !explicit["transparent_hugepage"] {
			value["transparent_hugepage"] = t.THP
		}
		if t.Overcommit != nil && !explicit["vm_overcommit_memory"] {
			value["vm_overcommit_memory"] = *t.Overcommit
		}
		if t.HugePages > 0 && !explicit["vm_nr_hugepages"] {
			value["vm_nr_hugepages"] = t.HugePages
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		conf, err := c.r.Render(c.ctx, hostSysctlSchema, "conf", raw)
		if err != nil {
			return err
		}
		thp, _ := value["transparent_hugepage"].(string) //nolint:errcheck // absent = schema default
		if thp == "" {
			thp = "never"
		}
		c.out.Spec.HostPrep = append(c.out.Spec.HostPrep,
			spec.HostPrep{Role: role, Machine: m, Kind: spec.HostPrepSysctl, Content: conf},
			spec.HostPrep{Role: role, Machine: m, Kind: spec.HostPrepScript, Content: thpScript(thp)},
		)
	}
	return nil
}

// thpScript sets transparent hugepages (a sysfs knob, not a sysctl) for
// the running kernel; defrag follows enabled so madvise regions are not
// compacted synchronously on a page fault.
//
// doc: docs.kernel.org/admin-guide/mm/transhuge.html — "sysfs"
func thpScript(mode string) string {
	defrag := mode
	if mode == "madvise" {
		defrag = "defer+madvise"
	}
	return fmt.Sprintf(`thp=/sys/kernel/mm/transparent_hugepage
if [ -w "$thp/enabled" ]; then
  echo %s > "$thp/enabled"
  echo %s > "$thp/defrag"
fi
`, mode, defrag)
}
