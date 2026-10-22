package compile

import (
	"encoding/json"
	"testing"
)

func TestNoopSkipsValidation(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`{"workload":{"script":"tpcc/tx"}}`, `{"no_steps":["validate_population"],"workload":{"script":"tpcc/tx"}}`},
		{`{"no_steps":["analyze"],"workload":{"script":"tpch/tx"}}`, `{"no_steps":["analyze","validate_answers"],"workload":{"script":"tpch/tx"}}`},
		// Listed steps are the user's choice.
		{`{"steps":["load_data","validate_population"],"workload":{"script":"tpcc/tx"}}`, `{"steps":["load_data","validate_population"],"workload":{"script":"tpcc/tx"}}`},
		{`{"workload":{"script":"simple"}}`, `{"workload":{"script":"simple"}}`},
	} {
		got, err := noopSteps(json.RawMessage(tc.in))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tc.want {
			t.Errorf("%s → %s, want %s", tc.in, got, tc.want)
		}
	}
}
