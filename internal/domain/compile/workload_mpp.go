package compile

import (
	"encoding/json"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
)

// sqlDialectParam is the stroppy flag that selects a built-in SQL dialect.
const sqlDialectParam = "sql-dialect"

// engineDialect is the analytic SQL dialect a database kind needs for a
// script, "" when the driver's own one is right:
//   - gp for TPC-H and TPC-DS on a Greenplum-family server: its schema
//     distributes and stores every table for an MPP engine, and stroppy stops
//     a run on such a server that did not ask for it (stroppy-io/stroppy#259);
//   - mariadb for TPC-DS on MariaDB: the mysql queries roll up together with
//     ORDER BY, which MariaDB refuses (stroppy-io/stroppy#271).
//
// required says a build without the dialect cannot measure the kind at all:
// the pg schema on an MPP engine leaves every table's placement to the
// server, so the numbers would not be comparable.
func engineDialect(kind catalog.DatabaseKind, script string) (dialect string, required bool) {
	switch {
	case dbkind.IsMPP(kind) && analyticScript(script):
		return "gp", true
	case kind == catalog.MariaDB && script == "tpcds":
		return "mariadb", false
	default:
		return "", false
	}
}

// kindDialect sets the dialect the database kind needs on an analytic
// segment. A dialect or SQL file the workload names itself wins.
func kindDialect(raw json.RawMessage, build catalog.StroppyVersion, kind catalog.DatabaseKind) (json.RawMessage, error) {
	var shape struct {
		Workload struct {
			Script     string          `json:"script"`
			SQLDialect json.RawMessage `json:"sql_dialect"`
			SQLFile    json.RawMessage `json:"sql_file"`
		} `json:"workload"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		return nil, err
	}
	w := shape.Workload
	dialect, required := engineDialect(kind, w.Script)
	if dialect == "" || set(w.SQLDialect) || set(w.SQLFile) {
		return raw, nil
	}
	if !stroppyHasParam(build, w.Script, sqlDialectParam) {
		if required {
			return nil, errs.Newf(errs.CodeInvalid, "stroppy %s has no Greenplum dialect for %s", build.Version, w.Script).Because("mpp_dialect_unavailable", "script", w.Script, "stroppy_version", build.Version)
		}
		return raw, nil
	}
	// Re-encode through raw maps so every other key keeps its exact value.
	var segment, workload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &segment); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(segment["workload"], &workload); err != nil {
		return nil, err
	}
	encodedDialect, err := json.Marshal(dialect)
	if err != nil {
		return nil, err
	}
	workload["sql_dialect"] = encodedDialect
	encoded, err := json.Marshal(workload)
	if err != nil {
		return nil, err
	}
	segment["workload"] = encoded
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
