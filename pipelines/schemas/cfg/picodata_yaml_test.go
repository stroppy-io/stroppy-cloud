package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func picodataFull() map[string]any {
	return map[string]any{
		"cluster_name":               "bench",
		"default_replication_factor": int64(2),
		"default_bucket_count":       int64(30000),
		"shredding":                  false,
		"tiers": []any{
			map[string]any{"name": "default", "replication_factor": int64(2), "can_vote": true, "bucket_count": int64(30000)},
			map[string]any{"name": "storage", "replication_factor": int64(3), "can_vote": false},
		},
		"instance_name":        "default_1_1",
		"replicaset_name":      "default_1",
		"tier":                 "default",
		"peer":                 []any{"10.0.0.11:3301", "10.0.0.12:3301"},
		"failure_domain":       map[string]any{"zone": "ru-central1-a"},
		"instance_dir":         "/var/lib/picodata",
		"iproto_listen":        "0.0.0.0:3301",
		"iproto_advertise":     "10.0.0.11:3301",
		"http_listen":          "0.0.0.0:8081",
		"pg_listen":            "0.0.0.0:4327",
		"pg_advertise":         "10.0.0.11:4327",
		"pg_ssl":               false,
		"admin_socket":         "/var/run/picodata/admin.sock",
		"boot_timeout":         int64(600),
		"audit":                "file:/var/log/picodata/audit.log",
		"memtx_memory":         "8G",
		"memtx_max_tuple_size": "4M",
		"vinyl_memory":         "512M",
		"vinyl_cache":          "512M",
		"vinyl_read_threads":   int64(4),
		"vinyl_write_threads":  int64(8),
		"log_level":            "info",
		"log_format":           "json",
		"log_destination":      "file:/var/log/picodata/picodata.log",
		"custom":               map[string]any{"share_dir": "/usr/share/picodata"},
	}
}

func picodataInvalid() []schematest.Invalid {
	return []schematest.Invalid{
		{Value: map[string]any{"memtx_memory": "8 gigs"}, Code: "PATTERN_MISMATCH", Path: "memtx_memory"},
		{Value: map[string]any{"log_level": "trace"}, Code: "CHOICE_NOT_ALLOWED", Path: "log_level"},
		{Value: map[string]any{"tiers": []any{map[string]any{"name": "storage"}}}, Code: "RULE_VIOLATED"},
		{Value: map[string]any{"boot_timeout": int64(0)}, Code: "GTE_VIOLATED", Path: "boot_timeout"},
		{Value: map[string]any{"junk": 1}, Code: "UNKNOWN_FIELD", Path: "junk"},
	}
}

func TestPicodata25(t *testing.T) {
	schematest.Run(t, Picodata25(), schematest.Cases{
		Valid:   []map[string]any{{}, picodataFull()},
		Invalid: picodataInvalid(),
		Render:  "conf",
		Contains: []string{
			"cluster:\n  name: ",
			"  tier:\n",
			"instance:\n",
			"  memtx:\n    memory: ",
			// 25.3 spells the network keys flat and calls the SQL section pg.
			"  iproto_listen: ",
			"  pg:\n    listen: ",
		},
	})

	out := renderDefaults(t, Picodata25())
	dontWantLines(t, out, "  iproto:\n", "pgproto", "system_memory", "wal_dir", "replication_mode")
}

func TestPicodata261(t *testing.T) {
	schematest.Run(t, Picodata261(), schematest.Cases{
		Valid: []map[string]any{{}, picodataFull()},
		Invalid: append(picodataInvalid(), schematest.Invalid{
			Value: map[string]any{"tiers": []any{map[string]any{"name": "default", "replication_mode": "sync"}}},
			Code: "UNKNOWN_FIELD",
		}),
		Render: "conf", Contains: []string{"  iproto:\n", "  pgproto:\n", "    system_memory:"},
	})
	dontWantLines(t, renderDefaults(t, Picodata261()), "replication_mode:", "wal_mode:")
}

func TestPicodata26(t *testing.T) {
	full := picodataFull()
	full["memtx_system_memory"] = "512M"
	full["wal_dir"] = "/var/lib/picodata/wal"
	full["backup_dir"] = "/var/lib/picodata/backup"
	full["tiers"] = []any{
		map[string]any{
			"name": "default", "replication_factor": int64(2), "can_vote": true,
			"bucket_count": int64(30000), "replication_mode": "sync", "wal_mode": "fsync",
		},
	}

	schematest.Run(t, Picodata26(), schematest.Cases{
		Valid:   []map[string]any{{}, full},
		Invalid: picodataInvalid(),
		Render:  "conf",
		Contains: []string{
			"cluster:\n  name: ",
			"  tier:\n",
			"instance:\n",
			"  memtx:\n    memory: ",
			// 26.x nests the network sections and renamed pg -> pgproto.
			"  iproto:\n    listen: ",
			"  http:\n    listen: ",
			"  pgproto:\n    listen: ",
			"    tls:\n      enabled: ",
		},
	})

	out := renderDefaults(t, Picodata26())
	wantLines(t, out,
		"  wal_dir: /var/lib/picodata",
		"  backup_dir: /var/lib/picodata/backup",
		"    system_memory: 256M",
		"      replication_mode: async",
		"      wal_mode: write",
	)
	dontWantLines(t, out, "iproto_listen:", "  pg:\n", "    ssl:")
}
