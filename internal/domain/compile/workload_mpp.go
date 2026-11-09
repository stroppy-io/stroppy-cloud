package compile

import (
	"encoding/json"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
)

// sqlDialectParam is the stroppy flag that selects a built-in SQL dialect.
const sqlDialectParam = "sql-dialect"

// gpDialect selects stroppy's gp dialect for the analytic scripts on a
// Greenplum-family server: its schema distributes and stores every table for
// an MPP engine, and stroppy stops a run on such a server that did not ask
// for it. A dialect or SQL file the workload names itself wins.
//
// The dialect is a parameter of the stroppy build (stroppy-io/stroppy#259):
// a build whose script does not list sql-dialect is left alone, so the
// segment keeps validating against the build the platform runs, and the
// compiler starts to ask for gp with the release that has it.
func gpDialect(raw json.RawMessage, build catalog.StroppyVersion) (json.RawMessage, error) {
	var segment map[string]any
	if err := json.Unmarshal(raw, &segment); err != nil {
		return nil, err
	}
	workload, ok := segment["workload"].(map[string]any)
	if !ok {
		return raw, nil
	}
	script, ok := workload["script"].(string)
	if !ok || (script != "tpch/tx" && script != "tpcds") || workload["sql_dialect"] != nil || workload["sql_file"] != nil {
		return raw, nil
	}
	if !stroppyHasParam(build, script, sqlDialectParam) {
		return raw, nil
	}
	workload["sql_dialect"] = "gp"
	return json.Marshal(segment)
}

// stroppyHasParam reports a typed parameter of a script of the build.
func stroppyHasParam(build catalog.StroppyVersion, script, name string) bool {
	sc, ok := build.Script(script)
	if !ok {
		return false
	}
	for _, p := range sc.Params {
		if p.Name == name {
			return true
		}
	}
	return false
}
