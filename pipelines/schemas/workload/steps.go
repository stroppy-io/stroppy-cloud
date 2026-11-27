package workload

import (
	"encoding/json"
	"fmt"
	"strings"
)

// A script's step graph depends on one of its parameters: the TPC-C terminal
// model and the TPC-H/TPC-DS execution mode. The first mode of each list is
// Stroppy's default.
// doc: `stroppy probe <script> --resolved -o json` → workloads[].steps
// (stroppycfg/testdata/stroppy-probe-modes.json).
var (
	tpccModes  = []string{"stress", "strict"}
	tpchModes  = []string{"legacy", "qualification", "power", "power-query-only", "throughput", "throughput-query-only", "full"}
	tpcdsModes = []string{"legacy", "queries", "benchmark"}
)

// Modes lists the step-graph modes of a script; one empty mode for a script
// whose steps never change.
func Modes(script string) []string {
	switch script {
	case "tpcc/tx", "tpcc/procs":
		return tpccModes
	case "tpch/tx":
		return tpchModes
	case "tpcds":
		return tpcdsModes
	default:
		return []string{""}
	}
}

// ModeParam is the workload parameter that selects a script's mode; empty for
// a script without modes.
func ModeParam(script string) string {
	switch script {
	case "tpcc/tx", "tpcc/procs":
		return "terminal_model"
	case "tpch/tx", "tpcds":
		return "mode"
	default:
		return ""
	}
}

// Mode is the step-graph mode a segment's workload parameters select. TPC-DS
// benchmark=true is the benchmark mode under its earlier name.
func Mode(script string, params map[string]any) string {
	modes := Modes(script)
	mode := modes[0]
	if key := ModeParam(script); key != "" {
		for _, m := range modes {
			if params[key] == m {
				mode = m
			}
		}
	}
	if script == "tpcds" && params["benchmark"] == true {
		return "benchmark"
	}
	return mode
}

// Steps lists actual bench step names, not SQL section names, in the order
// Stroppy runs them in the given mode.
// doc: `stroppy probe -o json` → workloads[].steps (stroppycfg/testdata/stroppy-probe.json).
func Steps(script, mode string) []string {
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
		return tpccSteps(script == "tpcc/procs", mode == "strict")
	case "tpch/tx":
		return tpchSteps(mode)
	case "tpcds":
		if mode == "benchmark" {
			return []string{
				"generate_maintenance", "drop_schema", "create_schema", "load_data", "create_indexes", "analyze",
				"prepare_maintenance", "power", "throughput_1", "maintenance_1", "throughput_2", "maintenance_2",
			}
		}
		return []string{"drop_schema", "create_schema", "load_data", "create_indexes", "analyze", "validate_answers", "workload"}
	case "banking":
		return []string{"drop_schema", "create_schema", "load_data", "transfer", "verify"}
	case "banking/inject":
		return []string{"setup_injection", "drop_schema", "create_schema", "load_data", "transfer", "verify"}
	default:
		return nil
	}
}

// doc: stroppy 6.5.0 workloads/tpcc/README.md "Strict terminal model".
func tpccSteps(procs, strict bool) []string {
	s := []string{"drop_schema", "create_schema"}
	if procs {
		s = append(s, "create_procedures")
		if strict {
			s = append(s, "create_terminal_procedures")
		}
	}
	s = append(s, "load_data", "create_indexes", "create_foreign_keys", "analyze", "validate_population")
	if strict {
		return append(s, "initialize_terminals", "workload", "delivery_results")
	}
	return append(s, "workload")
}

// doc: stroppy 6.5.0 workloads/tpch/README.md "Named execution modes": a
// named mode prefixes its graph steps with the mode, the full schedule runs
// the power and throughput phases twice.
func tpchSteps(mode string) []string {
	s := []string{"drop_schema", "create_schema", "load_data", "create_indexes", "analyze"}
	var phase []string
	runs := 1
	switch mode {
	case "legacy":
		return append(s, "validate_answers", "workload")
	case "qualification":
		return append(s, "validate_answers")
	case "power":
		phase = []string{"power_queries", "power_rf1", "power_rf2"}
	case "power-query-only":
		phase = []string{"power_queries"}
	case "throughput":
		phase = []string{"throughput_queries", "throughput_refresh"}
	case "throughput-query-only":
		phase = []string{"throughput_queries"}
	case "full":
		phase = []string{"power_queries", "power_rf1", "power_rf2", "throughput_queries", "throughput_refresh"}
		runs = 2
	default:
		return nil
	}
	s = append(s, mode+"/prepare_streams")
	for run := 1; run <= runs; run++ {
		for _, p := range phase {
			s = append(s, fmt.Sprintf("%s/run_%d/%s", mode, run, p))
		}
	}
	return s
}

// Measured tells the steps of a script whose iterations are measured.
// doc: `stroppy probe -o json` → workloads[].steps[].measured
func Measured(script, mode, step string) bool {
	switch {
	case script == "banking" || script == "banking/inject":
		return step == "transfer"
	case script == "tpch/tx" && mode != "legacy":
		return strings.Contains(step, "/run_")
	case script == "tpcds" && mode == "benchmark":
		return step == "power" || strings.HasPrefix(step, "throughput_")
	default:
		return step == "workload"
	}
}

// modeCEL is the CEL value of a segment's mode, as Mode reads it.
func modeCEL(script string) string {
	key := ModeParam(script)
	if key == "" {
		return `""`
	}
	value := fmt.Sprintf(`(%q in this.workload && this.workload.%s != null ? this.workload.%s : %q)`, key, key, key, Modes(script)[0])
	if script == "tpcds" {
		return `("benchmark" in this.workload && this.workload.benchmark == true ? "benchmark" : ` + value + `)`
	}
	return value
}

// stepFilterRule refuses SQL section names and typos that Stroppy would silently
// skip, using the same step inventory published in the catalog.
func stepFilterRule() string {
	var clauses []string
	for _, script := range Scripts() {
		for _, mode := range Modes(string(script)) {
			raw, err := json.Marshal(Steps(string(script), mode))
			if err != nil {
				panic(err)
			}
			clauses = append(clauses, fmt.Sprintf(`this.workload.script != %q || %s != %q || (!("steps" in this) || this.steps.all(step, step in %s)) && (!("no_steps" in this) || this.no_steps.all(step, step in %s))`,
				script, modeCEL(string(script)), mode, raw, raw))
		}
	}
	return "(" + strings.Join(clauses, ") && (") + ")"
}
