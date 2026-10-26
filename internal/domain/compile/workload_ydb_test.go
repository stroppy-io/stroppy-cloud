package compile

import (
	"encoding/json"
	"testing"
)

func TestRowStore(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`{"workload":{"script":"tpch/tx","ydb_store_mode":"column"}}`, `{"workload":{"script":"tpch/tx","ydb_store_mode":"row"}}`},
		{`{"workload":{"script":"tpch/tx","ydb_store_mode":"row"}}`, `{"workload":{"script":"tpch/tx","ydb_store_mode":"row"}}`},
		{`{"workload":{"script":"tpcb/tx"}}`, `{"workload":{"script":"tpcb/tx"}}`},
	} {
		got, err := rowStore(json.RawMessage(tc.in))
		if err != nil {
			t.Fatal(err)
		}
		var a, b any
		_ = json.Unmarshal(got, &a)
		_ = json.Unmarshal([]byte(tc.want), &b)
		if ga, _ := json.Marshal(a); string(ga) != func() string { gb, _ := json.Marshal(b); return string(gb) }() {
			t.Errorf("rowStore(%s) = %s, want %s", tc.in, got, tc.want)
		}
	}
}
