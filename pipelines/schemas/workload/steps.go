package workload

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Steps lists actual bench step names, not SQL section names. All workloads
// use the executor's "workload" gate for measurement.
// doc: Stroppy workloads/* Setup and pkg/bench/throughput.go.
func Steps(script string) []string {
	switch script {
	case "simple", "baseline":
		return []string{"drop_schema", "create_schema", "load_data", "workload"}
	case "execute_sql":
		return []string{"workload"}
	case "tpcb/tx", "tpcb/procs":
		s := []string{"drop_schema", "create_schema"}
		if script == "tpcb/procs" {
			s = append(s, "create_procedures")
		}
		return append(s, "load_data", "create_indexes", "create_foreign_keys", "analyze", "workload")
	case "tpcc/tx", "tpcc/procs":
		s := []string{"drop_schema", "create_schema"}
		if script == "tpcc/procs" {
			s = append(s, "create_procedures")
		}
		return append(s, "set_unlogged", "load_data", "create_indexes", "set_logged", "create_foreign_keys", "analyze", "validate_population", "workload")
	case "tpch/tx", "tpcds":
		return []string{"drop_schema", "create_schema", "set_unlogged", "load_data", "create_indexes", "set_logged", "analyze", "validate_answers", "workload"}
	default:
		return nil
	}
}

// stepFilterRule refuses SQL section names and typos that Stroppy would silently
// skip, using the same step inventory published in the catalog.
func stepFilterRule() string {
	var clauses []string
	for _, script := range []string{"simple", "baseline", "execute_sql", "tpcb/tx", "tpcb/procs", "tpcc/tx", "tpcc/procs", "tpch/tx", "tpcds"} {
		raw, err := json.Marshal(Steps(script))
		if err != nil {
			panic(err)
		}
		clauses = append(clauses, fmt.Sprintf(`this.workload.script != %q || (!("steps" in this) || this.steps.all(step, step in %s)) && (!("no_steps" in this) || this.no_steps.all(step, step in %s))`, script, raw, raw))
	}
	return "(" + strings.Join(clauses, ") && (") + ")"
}
