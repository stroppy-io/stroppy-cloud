package compile

import (
	"encoding/json"
	"fmt"
	"strings"
)

// stroppyAtLeast compares a catalog Stroppy version ("6.2.1") with major.minor.
func stroppyAtLeast(version string, major, minor int) bool {
	var ma, mi int
	if n, err := fmt.Sscanf(version, "%d.%d", &ma, &mi); err != nil || n != 2 {
		return true // a dev or custom build: assume current
	}
	return ma > major || ma == major && mi >= minor
}

// cockroachSQL selects the embedded dialect without replacing user SQL
// (Stroppy before 6.2, which resolves --sql-file against its assets).
func cockroachSQL(raw json.RawMessage, version string) (json.RawMessage, error) {
	var segment map[string]any
	if err := json.Unmarshal(raw, &segment); err != nil {
		return nil, err
	}
	workload, ok := segment["workload"].(map[string]any)
	if !ok {
		return raw, nil
	}
	script, ok := workload["script"].(string)
	if !ok {
		return raw, nil
	}
	var file string
	switch script {
	case "tpcb/tx", "tpcb/procs":
		file = "crdb.sql"
	case "tpcc/tx", "tpcc/procs":
		file = "crdb.sql"
		if version == "24.1" || strings.HasPrefix(version, "24.1.") {
			file = "crdb24.sql"
		}
	default:
		return raw, nil
	}
	if existing, ok := workload["sql_file"].(string); !ok || existing == "" {
		workload["sql_file"] = file
	}
	return json.Marshal(segment)
}
