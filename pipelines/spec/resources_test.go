package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func capacityRun() Run {
	return Run{Provider: Provider{Kind: ProviderYandex}, Machines: []Machine{{Name: "db-1", Role: "db", CPU: 4, MemoryGB: 8, InstanceType: "standard-v3", Disks: []Disk{{Name: "data", GB: 10, Type: "network-ssd", Mount: "/data"}}}, {Name: "runner-1", Role: "runner", CPU: 2, MemoryGB: 4, InstanceType: "standard-v3", Disks: []Disk{{Name: "unrelated", GB: 100000, Type: "network-ssd", Mount: "/cache"}}}}, Containers: []Container{{Name: "postgres", Role: "db", Machine: "db-1", Image: "postgres:17", Mounts: []Mount{{Source: "/data/pg", Target: "/var/lib/postgresql/data"}}}}, Workload: Workload{DriverType: "postgres", Segments: []json.RawMessage{json.RawMessage(`{"name":"huge","workload":{"script":"tpcc/tx","scale_factor":100000},"run":{"vus":1,"duration":"1m"}}`)}}}
}

func TestHugeDatasetRejectsUnrelatedDiskCapacity(t *testing.T) {
	r := capacityRun()
	err := ResourceErrors(CheckResources(r))
	if err == nil || !strings.Contains(err.Error(), "storage budget") {
		t.Fatalf("huge workload accepted on 10 GiB: %v", err)
	}
	r.Workload.DriverType = "noop"
	if err := ResourceErrors(CheckResources(r)); err != nil {
		t.Fatalf("noop should not need database storage: %v", err)
	}
}

// A dataset that does not fit one machine fits the four segment hosts of a
// Greenplum-family cluster, which distribute every table: their storage is
// counted together, as for the other distributed engines.
func TestDistributedStorageOfTheGreenplumFamily(t *testing.T) {
	for _, tc := range []struct {
		image  string
		accept bool
	}{
		{"docker.stroppy.io/stroppy-io/greengage:7.5.1", true},
		{"docker.stroppy.io/stroppy-io/warehousepg:7.6.1-whpg", true},
		{"docker.stroppy.io/stroppy-io/cloudberry:2.1.0-incubating", true},
		{"docker.stroppy.io/stroppy-io/greenplum:6.27.1", true},
		{"docker.stroppy.io/stroppy-io/opengpdb:6.30.2", true},
		{"postgres:17", false},
	} {
		t.Run(tc.image, func(t *testing.T) {
			r := capacityRun()
			r.Workload.Segments = []json.RawMessage{json.RawMessage(`{"name":"h","workload":{"script":"tpch/tx","scale_factor":20},"run":{"vus":1,"duration":"1m"}}`)}
			r.Containers = nil
			for _, name := range []string{"db-1", "db-2", "db-3", "db-4"} {
				if name != "db-1" {
					r.Machines = append(r.Machines, Machine{Name: name, Role: "db", CPU: 4, MemoryGB: 8, InstanceType: "standard-v3", Disks: []Disk{{Name: "data", GB: 10, Type: "network-ssd", Mount: "/data"}}})
				}
				r.Containers = append(r.Containers, Container{Name: name + "-db", Role: "db", Machine: name, Image: tc.image, Mounts: []Mount{{Source: "/data/mpp", Target: "/data/mpp"}}})
			}
			failed := false
			for _, issue := range CheckResources(r) {
				failed = failed || (issue.Code == "capacity_insufficient" && issue.Severity == "ERROR")
			}
			if failed == tc.accept {
				t.Errorf("capacity error = %v for %s", failed, tc.image)
			}
		})
	}
}

func TestResourcesRejectImpossibleLayouts(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Run)
		code string
	}{
		{"mount collision", func(r *Run) {
			r.Machines[0].Disks = append(r.Machines[0].Disks, Disk{Name: "wal", GB: 20, Mount: "/data"})
		}, "duplicate"},
		{"OS mount", func(r *Run) { r.Machines[0].Disks[0].Mount = "/etc" }, "unsafe_mount"},
		{"port collision", func(r *Run) {
			r.Containers[0].Ports = []Port{{Host: 5432, Container: 5432}}
			r.Containers = append(r.Containers, Container{Name: "other", Role: "db", Machine: "db-1", Ports: []Port{{Host: 5432, Container: 5432}}})
		}, "port_conflict"},
		{"fixed buffer", func(r *Run) {
			r.Containers[0].Files = []File{{Path: "/etc/postgresql.conf", Content: "shared_buffers = '16GB'\n"}}
		}, "memory_overcommit"},
		{"mysql buffer", func(r *Run) {
			r.Containers[0].Files = []File{{Path: "/etc/my.cnf", Content: "innodb_buffer_pool_size = 16384M\n"}}
		}, "memory_overcommit"},
		{"platform combination", func(r *Run) { r.Machines[0].CPU = 3 }, "yc_machine_shape"},
		{"unknown host target", func(r *Run) {
			r.HostPrep = []HostPrep{{Role: "db", Machine: "runner-1", Kind: HostPrepScript, Content: "true"}}
		}, "selector"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := capacityRun()
			r.Workload.DriverType = "noop"
			tc.edit(&r)
			found := false
			for _, i := range CheckResources(r) {
				if i.Code == tc.code && i.Severity == "ERROR" {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s", tc.code)
			}
		})
	}
}

func TestCapacityCoexistingFamiliesAndStaticDimensions(t *testing.T) {
	e := EstimateDataset([]json.RawMessage{json.RawMessage(`{"name":"a","workload":{"script":"tpch/tx","scale_factor":10}}`), json.RawMessage(`{"name":"b","workload":{"script":"tpch/tx","scale_factor":1}}`), json.RawMessage(`{"name":"c","workload":{"script":"tpcds","scale_factor":0.01}}`)})
	if e.DataGB != 10.25 {
		t.Fatalf("coexisting estimate %+v", e)
	}
}
