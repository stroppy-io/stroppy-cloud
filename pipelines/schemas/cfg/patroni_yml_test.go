package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestPatroniYml3(t *testing.T) {
	full := map[string]any{
		"scope":                      "stroppy-pg",
		"name":                       "pg-1",
		"dcs_namespace":              "/stroppy/",
		"restapi_listen":             "0.0.0.0:8008",
		"restapi_connect_address":    "10.0.0.11:8008",
		"etcd3_hosts":                []any{"10.0.0.21:2379", "10.0.0.22:2379", "10.0.0.23:2379"},
		"ttl":                        60,
		"loop_wait":                  10,
		"retry_timeout":              10,
		"maximum_lag_on_failover":    int64(8388608),
		"synchronous_mode":           "quorum",
		"synchronous_mode_strict":    "true",
		"synchronous_node_count":     2,
		"failsafe_mode":              "true",
		"use_pg_rewind":              "true",
		"use_slots":                  "true",
		"postgresql_listen":          "0.0.0.0:5432",
		"postgresql_connect_address": "10.0.0.11:5432",
		"data_dir":                   "/var/lib/postgresql/17/main",
		"bin_dir":                    "/usr/lib/postgresql/17/bin",
		"pg_hba_path":                "/etc/postgresql/pg_hba.conf",
		"superuser_username":         "postgres",
		"superuser_password":         "s3cret",
		"replication_username":       "replicator",
		"replication_password":       "r3pl",
		"rewind_username":            "rewind",
		"rewind_password":            "rw",
		"tag_nofailover":             "false",
		"tag_noloadbalance":          "false",
		"tag_clonefrom":              "true",
		"tag_nosync":                 "false",
		"watchdog_mode":              "off",
		"watchdog_device":            "/dev/watchdog",
		"watchdog_safety_margin":     5,
		"postgresql_parameters":      map[string]any{"max_connections": "500"},
	}

	schematest.Run(t, PatroniYml3(), schematest.Cases{
		Valid: []map[string]any{{}, full},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"ttl": int64(10)}, Code: "GTE_VIOLATED", Path: "ttl"},
			{Value: map[string]any{"synchronous_mode": "sync"}, Code: "CHOICE_NOT_ALLOWED", Path: "synchronous_mode"},
			{Value: map[string]any{"restapi_listen": "nope"}, Code: "PATTERN_MISMATCH", Path: "restapi_listen"},
			{
				Value: map[string]any{"ttl": int64(20), "loop_wait": int64(10), "retry_timeout": int64(10)},
				Code:  "RULE_VIOLATED", Path: "ttl-vs-loop",
			},
			{
				Value: map[string]any{"synchronous_mode_strict": "true"},
				Code:  "RULE_VIOLATED", Path: "strict-needs-sync",
			},
			{Value: map[string]any{"junk": 1}, Code: "UNKNOWN_FIELD", Path: "junk"},
		},
		Render: "conf",
		Contains: []string{
			"scope: ",
			"restapi:",
			"bootstrap:",
			"  dcs:",
			"    ttl: ",
			"    synchronous_mode: ",
			"    failsafe_mode: ",
			"      use_pg_rewind: ",
			"postgresql:",
			"  data_dir: ",
			"  authentication:",
			"tags:",
			"watchdog:",
		},
	})

	vals, _, err := PatroniYml3().Resolve(full)
	if err != nil {
		t.Fatal(err)
	}

	out, err := PatroniYml3().Render("conf", vals)
	if err != nil {
		t.Fatal(err)
	}

	wantLines(t, out,
		"scope: stroppy-pg",
		"name: pg-1",
		"  listen: 0.0.0.0:8008",
		"  connect_address: 10.0.0.11:8008",
		"etcd3:",
		"  hosts:",
		"      - 10.0.0.21:2379",
		"      - 10.0.0.23:2379",
		"    ttl: 60",
		"    synchronous_mode: quorum",
		"    synchronous_node_count: 2",
		"    failsafe_mode: true",
		"      use_pg_rewind: true",
		"      parameters:",
		`        max_connections: "500"`,
		"  data_dir: /var/lib/postgresql/17/main",
		"  pg_hba: /etc/postgresql/pg_hba.conf",
		"      username: replicator",
		`      password: "r3pl"`,
		"  clonefrom: true",
		"  mode: off",
		"  safety_margin: 5",
	)
}

func TestPatroniYml4(t *testing.T) {
	schematest.Run(t, PatroniYml4(), schematest.Cases{
		Valid:   []map[string]any{{}, {"synchronous_mode": "on", "synchronous_mode_strict": "true", "synchronous_node_count": 1}},
		Invalid: []schematest.Invalid{{Value: map[string]any{"ttl": 10}, Code: "GTE_VIOLATED", Path: "ttl"}},
		Render:  "conf", Contains: []string{"Patroni 4.x", "postgresql:"},
	})
}
