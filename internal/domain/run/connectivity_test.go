package run

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func networkChecked(id int64, payload string) RawEvent {
	return RawEvent{ID: id, At: time.Unix(id, 0).UTC(), Kind: "note", Subject: "network.checked", Input: json.RawMessage(payload)}
}

func TestProjectionConnectivity(t *testing.T) {
	st := NewState([]MachineSnapshot{{Name: "db-1", Role: "db"}, {Name: "runner-1", Role: "runner"}}, nil)
	o := st.Apply(StatusRunning, networkChecked(7, `{"stage":"deploy","total":2,"failed":1,"critical":1,
		"pairs":[
			{"from":"runner-1","to":"db-1","port":5432,"address":"10.0.0.5:5432","container":"postgres","critical":true,"outcome":"timeout","latency_ms":3000,"attempts":4,"error":"i/o timeout"},
			{"from":"runner-1","to":"db-1","port":9187,"critical":false,"outcome":"ok","latency_ms":0.4,"attempts":1}
		],
		"snapshots":[{"machine":"db-1","artifact":"artifact/8f1c3f2a-netdiag-probe-db-1"}]}`))
	c := st.Connectivity
	if c == nil || c.Total != 2 || c.Failed != 1 || c.Critical != 1 || len(c.Pairs) != 2 || len(c.Snapshots) != 1 {
		t.Fatalf("state %+v", c)
	}
	if !c.CheckedAt.Equal(time.Unix(7, 0)) || c.Pairs[0].Outcome != "timeout" || c.Pairs[0].Container != "postgres" {
		t.Fatalf("pair %+v", c.Pairs[0])
	}
	ev := o.Timeline
	if ev == nil || ev.Kind != "network.checked" || !strings.Contains(ev.Error, "db-1:5432 from runner-1 (timeout)") {
		t.Fatalf("timeline %+v", ev)
	}
	if pairs := ev.Payload["pairs"].([]any); len(pairs) != 1 {
		t.Fatalf("the timeline keeps the failed paths only: %v", pairs)
	}

	// A later report replaces the earlier one; an unknown outcome reads as other.
	st.Apply(StatusRunning, networkChecked(9, `{"stage":"workload","total":1,"failed":1,"critical":1,"pairs":[{"from":"runner-1","to":"db-1","port":5432,"critical":true,"outcome":"filtered"}]}`))
	if c := st.Connectivity; c.Stage != "workload" || len(c.Pairs) != 1 || c.Pairs[0].Outcome != "other" || c.Snapshots == nil {
		t.Fatalf("replaced %+v", c)
	}
}

func TestArtifactRefsIncludeNetworkSnapshots(t *testing.T) {
	r := Run{Result: json.RawMessage(`{"artifacts":["artifact/a-log","artifact/a-netdiag-probe-db-1"]}`)}
	r.State.Connectivity = &ConnectivityState{Snapshots: []ConnectivitySnapshot{
		{Machine: "db-1", Artifact: "artifact/a-netdiag-probe-db-1"},
		{Machine: "runner-1", Artifact: "artifact/a-netdiag-probe-runner-1"},
	}}
	got := artifactRefs(r)
	if strings.Join(got, ",") != "artifact/a-log,artifact/a-netdiag-probe-db-1,artifact/a-netdiag-probe-runner-1" {
		t.Fatalf("refs %v", got)
	}
	// A run without a result still serves its snapshots.
	r.Result = nil
	if got := artifactRefs(r); len(got) != 2 {
		t.Fatalf("refs %v", got)
	}
}
