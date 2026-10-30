package compile

import (
	"encoding/json"
	"strings"
)

// cockroach24Dialect selects crdb24 for TPC-C on CockroachDB 24.1: the crdb
// driver picks crdb.sql, whose procedures 24.1 cannot parse. A dialect or
// SQL file the workload names itself wins.
func cockroach24Dialect(raw json.RawMessage, version string) (json.RawMessage, error) {
	if version != "24.1" && !strings.HasPrefix(version, "24.1.") {
		return raw, nil
	}
	var segment map[string]any
	if err := json.Unmarshal(raw, &segment); err != nil {
		return nil, err
	}
	workload, ok := segment["workload"].(map[string]any)
	if !ok {
		return raw, nil
	}
	if script, ok := workload["script"].(string); !ok || script != "tpcc/tx" && script != "tpcc/procs" {
		return raw, nil
	}
	if workload["sql_dialect"] != nil || workload["sql_file"] != nil {
		return raw, nil
	}
	workload["sql_dialect"] = "crdb24"
	return json.Marshal(segment)
}
