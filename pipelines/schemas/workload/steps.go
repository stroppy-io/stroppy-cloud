package workload

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Steps lists actual bench step names, not SQL section names, in the order
// Stroppy runs them.
// doc: `stroppy probe -o json` → workloads[].steps (stroppycfg/testdata/stroppy-probe.json).
func Steps(script string) []string {
	switch script {
	case "simple", "baseline":
		return []string{"drop_schema", "create_schema", "load_data", "workload", "cleanup"}
	case "execute_sql":
		return []string{"validate_source", "workload"}
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
		return append(s, "load_data", "create_indexes", "create_foreign_keys", "analyze", "validate_population", "workload")
	case "tpch/tx", "tpcds":
		return []string{"drop_schema", "create_schema", "load_data", "create_indexes", "analyze", "validate_answers", "workload"}
	case "banking":
		return []string{"drop_schema", "create_schema", "load_data", "transfer", "verify"}
	case "banking/inject":
		return []string{"setup_injection", "drop_schema", "create_schema", "load_data", "transfer", "verify"}
	default:
		return nil
	}
}

// MeasuredStep is the one step of a script whose iterations are measured.
// doc: `stroppy probe -o json` → workloads[].steps[].measured
func MeasuredStep(script string) string {
	if script == "banking" || script == "banking/inject" {
		return "transfer"
	}
	return "workload"
}

// stepFilterRule refuses SQL section names and typos that Stroppy would silently
// skip, using the same step inventory published in the catalog.
func stepFilterRule() string {
	var clauses []string
	for _, script := range Scripts() {
		raw, err := json.Marshal(Steps(string(script)))
		if err != nil {
			panic(err)
		}
		clauses = append(clauses, fmt.Sprintf(`this.workload.script != %q || (!("steps" in this) || this.steps.all(step, step in %s)) && (!("no_steps" in this) || this.no_steps.all(step, step in %s))`, script, raw, raw))
	}
	return "(" + strings.Join(clauses, ") && (") + ")"
}
