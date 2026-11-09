package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestMppGucs1(t *testing.T) {
	schematest.Run(t, MppGucs1(), schematest.Cases{
		Valid: []map[string]any{
			{},
			{"optimizer": "off", "gp_vmem_protect_limit": int64(7372), "max_statement_mem": int64(6635), "statement_mem": int64(5529)},
			{
				"gp_vmem_protect_limit": int64(7372), "max_statement_mem": int64(6635), "statement_mem": int64(5529),
				"work_mem": int64(1024), "shared_buffers": int64(128), "effective_cache_size": int64(4096),
				"runaway_detector_activation_percent": int64(100), "gp_resource_manager": "queue", "optimizer": "on",
				"gp_autostats_mode": "none", "autovacuum": "off", "gp_interconnect_type": "udpifc", "gp_max_packet_size": int64(1420),
				"gp_interconnect_queue_depth": int64(16), "gp_interconnect_snd_queue_depth": int64(16), "max_connections": int64(250),
				"gp_fts_probe_timeout": int64(600), "gp_fts_probe_interval": int64(600), "gp_segment_connect_timeout": int64(1800),
				"max_wal_size": int64(4096),
				"custom":       map[string]any{"gp_workfile_compression": "on"},
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"gp_vmem_protect_limit": int64(0)}, Code: "GTE_VIOLATED", Path: "gp_vmem_protect_limit"},
			{Value: map[string]any{"optimizer": "maybe"}, Code: "CHOICE_NOT_ALLOWED", Path: "optimizer"},
			{Value: map[string]any{"gp_max_packet_size": int64(100)}, Code: "GTE_VIOLATED", Path: "gp_max_packet_size"},
			{Value: map[string]any{"custom": map[string]any{"Bad Key": "1"}}, Code: "RULE_VIOLATED", Path: "custom-keys"},
			{Value: map[string]any{"mirrors": 1}, Code: "UNKNOWN_FIELD", Path: "mirrors"},
		},
		Render: "conf",
		Contains: []string{
			"# GUCs for gpconfig",
		},
	})
}
