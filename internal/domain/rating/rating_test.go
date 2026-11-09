package rating

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
)

func TestScenarioMetric(t *testing.T) {
	t.Parallel()
	reg := schemas.New()
	c, err := catalog.New(context.Background(), reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{catalog: c}
	cases := []struct {
		scenario, want string
	}{
		{"tpcc/tx", "tpmc"},
		{"tpcc/procs+tpcc/tx", "tpmc"},
		{"tpch/tx", "query_geomean_ms"},
		{"tpcb/tx", "tps"},
		{"tpcc/tx+tpch/tx", ""},
		{"execute_sql", ""},
		{"no/such", ""},
	}
	for _, tc := range cases {
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()
			if got := s.scenarioMetric(tc.scenario); got != tc.want {
				t.Fatalf("%s rates by %q, want %q", tc.scenario, got, tc.want)
			}
		})
	}
}
