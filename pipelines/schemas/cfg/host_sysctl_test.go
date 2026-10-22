package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestHostSysctl(t *testing.T) {
	schematest.Run(t, HostSysctl(), schematest.Cases{
		Valid: []map[string]any{
			{},
			{
				"vm_swappiness":                 int64(0),
				"vm_overcommit_memory":          int64(2),
				"vm_overcommit_ratio":           int64(80),
				"vm_dirty_ratio":                int64(15),
				"vm_dirty_background_ratio":     int64(5),
				"vm_max_map_count":              int64(1048576),
				"vm_nr_hugepages":               int64(1024),
				"kernel_shmmax":                 int64(68719476736),
				"kernel_shmall":                 int64(16777216),
				"kernel_sem":                    "500 64000 200 256",
				"fs_file_max":                   int64(4194304),
				"fs_aio_max_nr":                 int64(2097152),
				"net_core_somaxconn":            int64(65535),
				"net_core_rmem_max":             int64(33554432),
				"net_core_wmem_max":             int64(33554432),
				"net_ipv4_tcp_keepalive_time":   int64(120),
				"net_ipv4_tcp_keepalive_intvl":  int64(10),
				"net_ipv4_tcp_keepalive_probes": int64(3),
				"net_ipv4_ip_local_port_range":  "10000 65000",
				"transparent_hugepage":          "madvise",
				"custom": map[string]any{
					"net.ipv4.tcp_fastopen": "3",
				},
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"vm_swappiness": int64(201)}, Code: "LTE_VIOLATED", Path: "vm_swappiness"},
			{Value: map[string]any{"kernel_sem": "250 32000"}, Code: "PATTERN_MISMATCH", Path: "kernel_sem"},
			{Value: map[string]any{"transparent_hugepage": "sometimes"}, Code: "CHOICE_NOT_ALLOWED", Path: "transparent_hugepage"},
			{Value: map[string]any{"vm_dirty_ratio": int64(5), "vm_dirty_background_ratio": int64(10)}, Code: "RULE_VIOLATED"},
			{Value: map[string]any{"junk": 1}, Code: "UNKNOWN_FIELD", Path: "junk"},
		},
		Render: "conf",
		Contains: []string{
			"vm.swappiness = ",
			"kernel.sem = ",
			"net.core.somaxconn = ",
			"fs.aio-max-nr = ",
		},
	})
}
