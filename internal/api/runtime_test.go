package api

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

func sameJSON(t *testing.T, a, b json.RawMessage) {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(x, y) {
		t.Fatal("advanced config lost in API roundtrip")
	}
}

func TestRuntimeAPIRoundtrip(t *testing.T) {
	raw := json.RawMessage(`{"containers":[{"name":"db-1","set":{"files":[{"path":"/etc/db.conf","content":""}]}}]}`)
	d := library.DatabaseSpec{Version: "17", Params: json.RawMessage(`{}`), Runtime: raw}
	sameJSON(t, raw, databaseSpecOf(specToWire(d)).Runtime)
	machine := json.RawMessage(`{"cpu":16,"memory_gb":64,"public_ip":false,"boot_disk":{"gb":100,"type":"network-ssd"}}`)
	sizes := map[string]library.RoleSize{"db": {Size: "S", Machine: machine}}
	sameJSON(t, machine, sizesFrom(oas.NewOptRoleSizes(sizesTo(sizes)))["db"].Machine)
	req := &oas.TestWrite{Execution: oas.NewOptSchemaValue(schemaValueOf(raw)), Sizes: oas.NewOptRoleSizes(sizesTo(sizes))}
	spec, err := testSpecFrom(req)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, raw, spec.Execution)
	h := &Handler{}
	wire := h.testOf(library.Test{Spec: spec}, library.Fit{}, library.Resolved{})
	v, ok := wire.Execution.Get()
	if !ok {
		t.Fatal("execution omitted from saved test response")
	}
	sameJSON(t, raw, rawOf(v))
}
