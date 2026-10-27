package compile

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
)

func TestTuneDriverSizesThePoolForTheWorkload(t *testing.T) {
	segments := []json.RawMessage{
		json.RawMessage(`{"name":"load","workload":{"script":"tpcc/tx","load_workers":300},"run":{"vus":1}}`),
		json.RawMessage(`{"name":"main","workload":{"script":"tpcc/tx","load_workers":16},"run":{"vus":256}}`),
	}
	auto := func(params string) *compilation {
		return &compilation{in: Input{
			Database: library.DatabaseSpec{Params: json.RawMessage(params)},
			Workload: library.WorkloadSpec{Segments: segments},
		}}
	}
	pool := func(driver map[string]any) map[string]any {
		p, _ := driver["pool"].(map[string]any) //nolint:errcheck // nil fails below
		return p
	}

	pg := map[string]any{}
	if dsn := auto(`{}`).tuneDriver(pg, "postgres", "postgres://h/db"); dsn != "postgres://h/db" || pool(pg)["maxConns"] != 300 || pool(pg)["minConns"] != 300 {
		t.Fatalf("postgres: %s %v", dsn, pg)
	}

	my := map[string]any{"sql": map[string]any{"maxIdleConns": 8}}
	auto(`{}`).tuneDriver(my, "mysql", "")
	if pool(my)["maxOpenConns"] != 300 || pool(my)["maxIdleConns"] != nil {
		t.Fatalf("mysql: an explicit sql key must win: %v", my)
	}

	dsn := auto(`{}`).tuneDriver(map[string]any{}, "crdb", "postgresql://root@${ip:role:db}:26257/db?sslmode=disable")
	if !strings.Contains(dsn, "pool_max_conns=300") || !strings.Contains(dsn, "${ip:role:db}") {
		t.Fatalf("crdb: %s", dsn)
	}
	if kept := poolURL("postgresql://h/db?pool_max_conns=8", 300); kept != "postgresql://h/db?pool_max_conns=8" {
		t.Fatalf("crdb: an explicit URL pool must win: %s", kept)
	}

	stock := map[string]any{}
	if auto(`{"tuning":"stock"}`).tuneDriver(stock, "postgres", ""); len(stock) != 0 {
		t.Fatalf("stock tuning keeps the driver defaults: %v", stock)
	}
}

func TestClientSetupLeavesCredentialsOut(t *testing.T) {
	c := &compilation{}
	c.out.Spec.Workload.DriverType = "postgres"
	c.out.Spec.Workload.URL = "postgres://u:secret@h/db"
	c.out.Spec.Workload.Driver = map[string]any{
		"pool": map[string]any{"maxConns": 64}, "bulkSize": 2500,
		"authUser": "u", "authPassword": "secret", "authToken": "t", "serviceAccountKeyFile": "/k", "caCert": "pem",
	}
	got := c.clientSetup().Settings
	if len(got) != 2 || got["pool"] == nil || got["bulkSize"] == nil {
		t.Fatalf("only pool and bulkSize may be reported: %v", got)
	}
}
