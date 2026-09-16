package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRuntimePreservesExplicitValues(t *testing.T) {
	patch, err := NormalizeRuntime(json.RawMessage(`{"machines":{"db-1":{"cpu":8,"memory_gb":32,"preemptible":false,"boot_disk":{"gb":80,"type":"network-ssd"},"disks":[{"name":"data","gb":200,"type":"network-ssd","mount":"/data"}]}},"containers":[{"name":"postgres","set":{"env":{"A":"new"},"files":[{"path":"/etc/db.conf","content":""}],"cmd":[]}}],"network":{"allow_public_ips":false}}`))
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	r := Run{Machines: []Machine{{Name: "db-1", Role: "db", CPU: 2, MemoryGB: 4, Preemptible: &yes}}, Network: Network{AllowPublicIPs: true}, Containers: []Container{{Name: "postgres", Role: "db", Machine: "db-1", Image: "postgres:17", Cmd: []string{"old"}, Env: map[string]string{"A": "old", "B": "keep"}, Restart: "always", Files: []File{{Path: "/etc/db.conf", Content: "generated"}, {Path: "/etc/other", Content: "keep"}}}}}
	if err := ApplyRuntime(&r, patch); err != nil {
		t.Fatal(err)
	}
	m, c := r.Machines[0], r.Containers[0]
	if m.CPU != 8 || m.MemoryGB != 32 || m.Preemptible == nil || *m.Preemptible || m.BootDisk.GB != 80 || m.Disks[0].GB != 200 {
		t.Fatalf("machine overlay lost values: %+v", m)
	}
	if r.Network.AllowPublicIPs || c.Env["A"] != "new" || c.Env["B"] != "keep" || len(c.Cmd) != 0 || c.Restart != "always" || c.Files[0].Content != "" || len(c.Files) != 2 {
		t.Fatal("container/network overlay lost explicit values or reset omitted fields")
	}
}

func TestRuntimeRejectsUnknownAndUnmatchedFields(t *testing.T) {
	for _, raw := range []string{`{"ignored":true}`, `{"machines":{"db-1":{"typo":2}}}`, `{"containers":[{"set":{"image":"x"}}]}`, `{"containers":[{"name":"db","set":{"files":[{"path":"/etc/x"}]}}]}`} {
		if _, err := NormalizeRuntime(json.RawMessage(raw)); err == nil {
			t.Errorf("accepted invalid runtime %s", raw)
		}
	}
	r := Run{Machines: []Machine{{Name: "db-1"}}}
	if err := ApplyRuntime(&r, json.RawMessage(`{"machines":{"typo":{"cpu":4}}}`)); err == nil {
		t.Fatal("unknown machine silently ignored")
	}
	if err := ApplyRuntime(&r, json.RawMessage(`{"containers":[{"name":"typo","set":{"image":"x"}}]}`)); err == nil {
		t.Fatal("unknown container silently ignored")
	}
}

func TestMachineOverrideValidatesNestedStorage(t *testing.T) {
	for _, raw := range []string{`{"cpu":0}`, `{"memory_gb":-1}`, `{"boot_disk":{"gb":80}}`, `{"disks":[{"name":"data","gb":20}]}`} {
		if _, err := NormalizeMachineOverride(json.RawMessage(raw)); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	raw, err := NormalizeMachineOverride(json.RawMessage(`{"cpu":4,"public_ip":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "memory_gb") {
		t.Fatal("invented absent memory override")
	}
}

func TestRuntimeWorkloadPreservesUint64AndEmptyObjects(t *testing.T) {
	raw, err := NormalizeRuntime(json.RawMessage(`{"workload":{"url":"postgres://db:5433/app","segments":[{"name":"exact","seed":18446744073709551615,"workload":{"script":"simple"},"run":{"executor":"shared-iterations","iterations":1}}]},"containers":[{"name":"db","set":{"env":{},"healthcheck":null}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	r := Run{Workload: Workload{URL: "postgres://old", DriverType: "postgres"}, Containers: []Container{{Name: "db", Env: map[string]string{"REMOVE": "yes"}, Healthcheck: &Healthcheck{Cmd: []string{"true"}}}}}
	if err := ApplyRuntime(&r, raw); err != nil {
		t.Fatal(err)
	}
	segments, err := DecodeSegments(r.Workload.Segments)
	if err != nil {
		t.Fatal(err)
	}
	if segments[0].Seed == nil || *segments[0].Seed != ^uint64(0) || r.Workload.URL != "postgres://db:5433/app" {
		t.Fatal("workload override lost precision or endpoint")
	}
	if len(r.Containers[0].Env) > 0 || r.Containers[0].Healthcheck != nil {
		t.Fatal("explicit empty/null did not clear defaults")
	}
}
