package api

import (
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

func TestOverviewConnectivity(t *testing.T) {
	r := run.Run{Status: run.StatusFailed}
	if overviewOf(r, false).Connectivity.Set {
		t.Fatal("an unchecked run has no connectivity block")
	}
	r.State.Connectivity = &run.ConnectivityState{
		CheckedAt: time.Unix(7, 0).UTC(), Total: 3, Failed: 2, Critical: 1,
		Pairs: []run.ConnectivityPair{
			{From: "runner-1", To: "db-1", Port: 9187, Outcome: "ok", LatencyMs: 0.4, Attempts: 1},
			{From: "runner-1", To: "db-1", Port: 9100, Container: "node-exporter", Outcome: "refused", Error: "connection refused"},
			{From: "db-2", To: "db-1", Port: 5432, Address: "10.0.0.5:5432", Critical: true, Outcome: "timeout", Error: "i/o timeout"},
		},
		Snapshots: []run.ConnectivitySnapshot{{Machine: "db-1", Artifact: "artifact/8f1c3f2a-netdiag-probe-db-1"}},
	}
	c, ok := overviewOf(r, false).Connectivity.Get()
	if !ok || c.Total != 3 || c.Failed != 2 || c.Critical != 1 || !c.CheckedAt.Set {
		t.Fatalf("connectivity %+v", c)
	}
	if c.Pairs[0].Port != 5432 || !c.Pairs[0].Critical || c.Pairs[0].Outcome != oas.ConnectivityOutcomeTimeout || c.Pairs[1].Port != 9100 || c.Pairs[2].Outcome != oas.ConnectivityOutcomeOk {
		t.Fatalf("critical failures first, then warnings, then ok: %+v", c.Pairs)
	}
	if c.Pairs[1].Container.Value != "node-exporter" || c.Pairs[0].Address.Value != "10.0.0.5:5432" {
		t.Fatalf("pair fields %+v", c.Pairs)
	}
	if len(c.Snapshots) != 1 || c.Snapshots[0].ArtifactID != "8f1c3f2a-netdiag-probe-db-1" {
		t.Fatalf("snapshot ids are downloadable ids: %+v", c.Snapshots)
	}
}
