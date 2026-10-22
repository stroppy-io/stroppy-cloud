package cfg

import (
	"strings"
	"testing"
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

// wantContains is wantLines without the trailing newline: the cockroach "conf"
// template is a single flag line, not a file of lines.
func wantContains(t *testing.T, text string, subs ...string) {
	t.Helper()

	for _, want := range subs {
		if !strings.Contains(text, want) {
			t.Errorf("rendered flags lack %q:\n%s", want, text)
		}
	}
}

func cockroachFull() map[string]any {
	return map[string]any{
		"store_path":      "/var/lib/cockroach/data",
		"store_size":      "80%",
		"ballast_size":    "1GiB",
		"cache":           "35%",
		"max_sql_memory":  "25%",
		"max_tsdb_memory": "128MiB",
		"locality":        "region=ru-central1,zone=ru-central1-a",
		"join":            []any{"10.0.0.11:26257", "10.0.0.12:26257", "10.0.0.13:26257"},
		"advertise_addr":  "10.0.0.11:26257",
		"listen_addr":     "0.0.0.0:26257",
		"http_addr":       "0.0.0.0:8080",
		"sql_addr":        "0.0.0.0:26258",
		"insecure":        true,
		"max_offset":      500 * time.Millisecond,
		"cluster_name":    "stroppy",
		"gc_ttl_seconds":  int64(600),
		"cluster_settings": map[string]any{
			"kv.snapshot_rebalance.max_rate":      "256MiB",
			"kv.range_split.load_qps_threshold":   "5000",
			"kv.closed_timestamp.target_duration": "3s",
			"sql.defaults.vectorize":              "on",
		},
	}
}

func cockroachCases(full map[string]any) schematest.Cases {
	return schematest.Cases{
		Valid: []map[string]any{{}, full},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"store_size": "lots"}, Code: "PATTERN_MISMATCH", Path: "store_size"},
			{Value: map[string]any{"locality": "Region=Foo"}, Code: "PATTERN_MISMATCH", Path: "locality"},
			{Value: map[string]any{"max_offset": 30 * time.Second}, Code: "LTE_VIOLATED", Path: "max_offset"},
			{Value: map[string]any{"cluster_settings": map[string]any{"NotASetting": "1"}}, Code: "RULE_VIOLATED", Path: "cluster_settings"},
			{Value: map[string]any{"junk": 1}, Code: "UNKNOWN_FIELD", Path: "junk"},
			// no certificates are provisioned, so secure mode is refused
			{Value: map[string]any{"insecure": false}, Code: "RULE_VIOLATED", Path: "insecure-only"},
			{Value: map[string]any{"gc_ttl_seconds": int64(10)}, Code: "GTE_VIOLATED", Path: "gc_ttl_seconds"},
		},
		Render: "conf",
		Contains: []string{
			"--store=path=",
			"--cache=",
			"--max-sql-memory=",
			"--insecure",
		},
	}
}

// runCockroach exercises both templates of one major and returns the rendered
// flag line of the defaults.
func runCockroach(t *testing.T, s *schemapb.Schema, full map[string]any) string {
	t.Helper()

	schematest.Run(t, s, cockroachCases(full))

	schematest.Run(t, s, schematest.Cases{
		Valid:    []map[string]any{full},
		Render:   "settings",
		Contains: []string{"SET CLUSTER SETTING ", ";", "ALTER RANGE default CONFIGURE ZONE USING gc.ttlseconds = 600;"},
	})

	return renderDefaults(t, s)
}

func TestCockroach24(t *testing.T) {
	full := cockroachFull()
	full["wal_failover"] = "among-stores"
	full["max_disk_temp_storage"] = "30GiB"
	full["locality_advertise_addr"] = "region=ru-central1@10.0.0.11:26257"

	out := runCockroach(t, Cockroach24(), full)

	// --wal-failover, --max-disk-temp-storage and --locality-advertise-addr
	// are on the v24.1 page already.
	wantContains(t, out, "--max-disk-temp-storage=32GiB")
	dontWantLines(t, out, "--wal-failover", "--locality-advertise-addr")
}

func TestCockroach25(t *testing.T) {
	full := cockroachFull()
	full["wal_failover"] = "among-stores"
	full["max_disk_temp_storage"] = "64GiB"
	full["locality_advertise_addr"] = "region=ru-central1@10.0.0.11:26257"

	out := runCockroach(t, Cockroach25(), full)

	// --wal-failover is omitted while it is disabled, which is the default.
	wantContains(t, out, "--max-disk-temp-storage=32GiB")
	dontWantLines(t, out, "--wal-failover", "--locality-advertise-addr")
}

func TestCockroach26(t *testing.T) {
	full := cockroachFull()
	full["wal_failover"] = "among-stores"
	full["max_disk_temp_storage"] = "64GiB"

	out := runCockroach(t, Cockroach26(), full)

	wantContains(t, out, "--max-disk-temp-storage=32GiB")
	dontWantLines(t, out, "--wal-failover")
}
