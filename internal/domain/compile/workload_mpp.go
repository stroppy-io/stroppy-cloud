package compile

import (
	"encoding/json"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
)

// sqlDialectParam is the stroppy flag that selects a built-in SQL dialect.
const sqlDialectParam = "sql-dialect"

// gpDialect selects stroppy's gp dialect for the analytic scripts on a
// Greenplum-family server: its schema distributes and stores every table for
// an MPP engine, and stroppy stops a run on such a server that did not ask
// for it. A dialect or SQL file the workload names itself wins.
//
// The dialect is a parameter of the stroppy build (stroppy-io/stroppy#259).
// A build without it would load the pg schema, which leaves every table's
// placement to the server: the numbers would not be comparable, so such a
// run is refused instead of measured.
func gpDialect(raw json.RawMessage, build catalog.StroppyVersion) (json.RawMessage, error) {
	var segment map[string]json.RawMessage
	if err := json.Unmarshal(raw, &segment); err != nil {
		return nil, err
	}
	var workload map[string]json.RawMessage
	if len(segment["workload"]) == 0 || json.Unmarshal(segment["workload"], &workload) != nil {
		return raw, nil
	}
	var script string
	if json.Unmarshal(workload["script"], &script) != nil || (script != "tpch/tx" && script != "tpcds") {
		return raw, nil
	}
	if set(workload["sql_dialect"]) || set(workload["sql_file"]) {
		return raw, nil
	}
	if !stroppyHasParam(build, script, sqlDialectParam) {
		return nil, errs.Newf(errs.CodeInvalid, "stroppy %s has no Greenplum dialect for %s", build.Version, script).Because("mpp_dialect_unavailable", "script", script, "stroppy_version", build.Version)
	}
	workload["sql_dialect"] = json.RawMessage(`"gp"`)
	w, err := json.Marshal(workload)
	if err != nil {
		return nil, err
	}
	segment["workload"] = w
	return json.Marshal(segment)
}

// set reports a JSON value that is present and not null.
func set(v json.RawMessage) bool { return len(v) > 0 && string(v) != "null" }

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
