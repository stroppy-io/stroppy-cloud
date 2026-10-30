package compile

import (
	"encoding/json"
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
	for _, tc := range []struct {
		name, params, driverType string
		driver, want             map[string]any
	}{
		{"postgres", `{}`, "postgres", map[string]any{}, map[string]any{"maxConns": 300, "minConns": 300}},
		{"picodata", `{}`, "picodata", map[string]any{}, map[string]any{"maxConns": 300, "minConns": 300}},
		{"crdb is pgx too", `{}`, "crdb", map[string]any{}, map[string]any{"maxConns": 300, "minConns": 300}},
		{"an explicit postgres key wins", `{}`, "crdb", map[string]any{"postgres": map[string]any{"maxConns": 8}}, map[string]any{"minConns": 300}},
		{"an explicit sql key wins", `{}`, "mysql", map[string]any{"sql": map[string]any{"maxIdleConns": 8}}, map[string]any{"maxOpenConns": 300}},
		{"ydb", `{}`, "ydb", map[string]any{}, map[string]any{"maxOpenConns": 300, "maxIdleConns": 300}},
		{"stock keeps the driver defaults", `{"tuning":"stock"}`, "postgres", map[string]any{}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auto(tc.params).tuneDriver(tc.driver, tc.driverType)
			got, ok := tc.driver["pool"].(map[string]any)
			if !ok {
				got = nil
			}
			if len(got) != len(tc.want) {
				t.Fatalf("pool %v, want %v", got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Fatalf("pool %v, want %v", got, tc.want)
				}
			}
		})
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
