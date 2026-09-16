package spec

import (
	"encoding/json"
	"testing"
)

func TestNormalizePreservesExplicitValues(t *testing.T) {
	r := sampleRun()
	r.Network.AllowPublicIPs = false
	r.Workload.Segments = []json.RawMessage{json.RawMessage(`{"name":"seeded","seed":18446744073709551615,"workload":{"script":"simple"},"run":{"executor":"shared-iterations","iterations":1},"thresholds":{"error_rate":0}}`)}
	out, err := NormalizeRun(r)
	if err != nil {
		t.Fatal(err)
	}
	segments, err := DecodeSegments(out.Workload.Segments)
	if err != nil {
		t.Fatal(err)
	}
	if out.Network.AllowPublicIPs {
		t.Fatal("explicit false lost")
	}
	if segments[0].Seed == nil || *segments[0].Seed != ^uint64(0) {
		t.Fatal("seed lost precision")
	}
	if segments[0].Thresholds.ErrorRate == nil || *segments[0].Thresholds.ErrorRate != 0 {
		t.Fatal("explicit zero lost")
	}
	suite, err := NormalizeSuite(Suite{SuiteRunID: r.RunID, Tenant: r.Tenant, Cells: []SuiteCell{{ID: "one", RunSpec: r}}, Concurrency: 1, Defaults: SuiteDefaults{ContinueOnFailure: false}})
	if err != nil {
		t.Fatal(err)
	}
	if suite.Defaults.ContinueOnFailure {
		t.Fatal("suite explicit false lost")
	}
}

func TestPipelineJSONDoesNotDiscardUnknownFields(t *testing.T) {
	for _, raw := range []string{`{"unknown":true}`, `{"network":{"allow_public_ips":false,"ignored":true}}`} {
		var r Run
		if json.Unmarshal([]byte(raw), &r) == nil {
			t.Fatal("unknown input accepted:", raw)
		}
	}
	var r Run
	if err := json.Unmarshal([]byte(`{"network":{}}`), &r); err != nil {
		t.Fatal(err)
	}
	if !r.Network.AllowPublicIPs {
		t.Fatal("schema default true was not applied")
	}
	var s Suite
	if err := json.Unmarshal([]byte(`{}`), &s); err != nil {
		t.Fatal(err)
	}
	if !s.Defaults.ContinueOnFailure {
		t.Fatal("schema default true was not applied")
	}
}
