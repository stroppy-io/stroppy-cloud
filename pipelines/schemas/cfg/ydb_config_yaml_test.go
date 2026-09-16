package cfg

import (
	"testing"

	schemapb "github.com/gopherex/schemapb/go/schemapb"
	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func ydbFull() map[string]any {
	hosts := make([]any, 0, 9)
	for i := 1; i <= 9; i++ {
		hosts = append(hosts, map[string]any{
			"host":           "ydb-storage-" + string(rune('0'+i)),
			"node_id":        int64(i),
			"host_config_id": int64(1),
			"port":           int64(19001),
			"data_center":    string(rune('0' + (i-1)/3 + 1)),
			"rack":           string(rune('0' + (i-1)%3 + 1)),
		})
	}

	return map[string]any{
		"static_erasure": "mirror-3-dc",
		"host_config_id": int64(1),
		"drives": []any{
			map[string]any{"path": "/dev/disk/by-partlabel/ydb_disk_ssd_01", "type": "nvme"},
			map[string]any{"path": "/dev/disk/by-partlabel/ydb_disk_ssd_02", "type": "nvme"},
			map[string]any{"path": "/dev/disk/by-partlabel/ydb_disk_ssd_03", "type": "nvme"},
		},
		"hosts":  hosts,
		"domain": "Root",
		"storage_pool_types": []any{
			map[string]any{
				"kind":            "ssd",
				"erasure_species": "mirror-3-dc",
				"pdisk_type":      "NVME",
				"vdisk_kind":      "Default",
				"box_id":          int64(1),
			},
		},
		"state_storage_nodes":            []any{int64(1), int64(2), int64(3), int64(4), int64(5), int64(6), int64(7), int64(8), int64(9)},
		"state_storage_nto_select":       int64(9),
		"enforce_user_token_requirement": false,
		"blob_storage_service_set": map[string]any{
			"groups": []any{map[string]any{"erasure_species": "mirror-3-dc"}},
		},
		"use_auto_config":               true,
		"node_type":                     "STORAGE",
		"cpu_count":                     int64(16),
		"grpc_port":                     int64(2136),
		"grpcs":                         true,
		"grpc_ssl_port":                 int64(2135),
		"interconnect_port":             int64(19001),
		"monitoring_port":               int64(8765),
		"enable_query_service_spilling": true,
		"spilling_root":                 "/var/lib/ydb/spilling",
		"spilling_max_total_size":       int64(21474836480),
		"log_default_level":             int64(5),
		"log_syslog":                    false,
		"log_format":                    "full",
		"feature_flags":                 map[string]any{"enable_views": true},
		"custom":                        map[string]any{"cms_config": "{sentinel_config: {}}"},
	}
}

func ydbCases(extra map[string]any) schematest.Cases {
	full := ydbFull()
	for k, v := range extra {
		full[k] = v
	}

	return schematest.Cases{
		Valid: []map[string]any{{}, full},
		Invalid: []schematest.Invalid{
			{
				Value: map[string]any{"drives": []any{map[string]any{"path": "/dev/nvme0n1", "type": "SSD"}}},
				Code:  "CHOICE_NOT_ALLOWED", Path: "drives[0].type",
			},
			{Value: map[string]any{"state_storage_nto_select": int64(4)}, Code: "RULE_VIOLATED"},
			{Value: map[string]any{"log_default_level": int64(9)}, Code: "LTE_VIOLATED", Path: "log_default_level"},
			{Value: map[string]any{
				"hosts":              []any{map[string]any{"host": "a", "node_id": int64(1)}},
				"storage_pool_types": []any{map[string]any{"erasure_species": "mirror-3-dc"}},
			}, Code: "RULE_VIOLATED"},
			{Value: map[string]any{"junk": 1}, Code: "UNKNOWN_FIELD", Path: "junk"},
		},
		Render: "conf",
		Contains: []string{
			"host_configs:\n",
			"domains_config:\n",
			"blob_storage_config:\n",
			"actor_system_config:\n",
			"log_config:\n",
		},
	}
}

// ydbMemoryCases is ydbCases plus the memory_controller_config section, which
// both live majors carry.
func ydbMemoryCases() schematest.Cases {
	c := ydbCases(map[string]any{
		"memory_hard_limit_bytes":           int64(68719476736),
		"memory_soft_limit_percent":         int64(75),
		"memory_target_utilization_percent": int64(50),
		"memory_shared_cache_min_percent":   int64(20),
		"memory_shared_cache_max_percent":   int64(60),
	})
	c.Contains = append(c.Contains, "memory_controller_config:\n")

	return c
}

func TestYdb25(t *testing.T) {
	schematest.Run(t, Ydb25(), ydbMemoryCases())
}

// TestYdb26 pins the decision that 26.x renders the same configuration V1
// document as 25.x: V2 (the metadata/config envelope) is still experimental
// and 26.x neither requires it nor drops V1.
func TestYdb26(t *testing.T) {
	c := ydbMemoryCases()
	c.Contains = append(c.Contains, "cfg.ydb.config.yaml@26")
	schematest.Run(t, Ydb26(), c)

	out := renderDefaults(t, Ydb26())
	dontWantLines(t, out, "metadata:", "kind: MainConfig")
}

func TestYdbDriveMediaRendering(t *testing.T) {
	schematest.Run(t, Ydb26(), schematest.Cases{
		Valid: []map[string]any{ydbFull()}, Render: "conf",
		Contains: []string{"path: /dev/disk/by-partlabel/ydb_disk_ssd_01\n    type: NVME"},
	})
}

// Profile zero is mandatory for system tablet startup in configuration V1.
func TestYdbSystemTabletChannels(t *testing.T) {
	for _, s := range []*schemapb.Schema{Ydb25(), Ydb26()} {
		for _, tc := range []struct {
			media    string
			category int
		}{{"ROT", 0}, {"SSD", 1}, {"NVME", 2}} {
			input := ydbFull()
			input["storage_pool_types"].([]any)[0].(map[string]any)["pdisk_type"] = tc.media
			vals, result, err := s.Resolve(input)
			if err != nil || result.Blocking() {
				t.Fatalf("resolve: %v, %v", err, result)
			}
			out, err := s.Render("conf", vals)
			if err != nil {
				t.Fatal(err)
			}
			var conf struct {
				StaticErasure string `yaml:"static_erasure"`
				Channels      struct {
					Profiles []struct {
						ID       int `yaml:"profile_id"`
						Channels []struct {
							Erasure  string `yaml:"erasure_species"`
							Category int    `yaml:"pdisk_category"`
							Kind     string `yaml:"storage_pool_kind"`
						} `yaml:"channel"`
					} `yaml:"profile"`
				} `yaml:"channel_profile_config"`
			}
			if err := yaml.Unmarshal([]byte(out), &conf); err != nil {
				t.Fatal(err)
			}
			if conf.StaticErasure != "mirror-3-dc" || len(conf.Channels.Profiles) != 1 || conf.Channels.Profiles[0].ID != 0 || len(conf.Channels.Profiles[0].Channels) != 3 {
				t.Fatalf("invalid system channel profile: %+v", conf)
			}
			for _, c := range conf.Channels.Profiles[0].Channels {
				if c.Erasure != "mirror-3-dc" || c.Category != tc.category || c.Kind != "ssd" {
					t.Fatalf("channel does not match storage pool: %+v", c)
				}
			}
		}
	}
}
