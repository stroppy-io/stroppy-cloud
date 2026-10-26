package compile

import "encoding/json"

// rowStore keeps a segment on row tables: serverless YDB has no column
// store ("OLAP schema operations are not supported"), and column is the
// schema's default for TPC-H and TPC-DS.
func rowStore(raw json.RawMessage) (json.RawMessage, error) {
	var segment map[string]any
	if err := json.Unmarshal(raw, &segment); err != nil {
		return nil, err
	}
	wl, ok := segment["workload"].(map[string]any)
	if !ok || wl["ydb_store_mode"] != "column" {
		return raw, nil
	}
	wl["ydb_store_mode"] = "row"
	return json.Marshal(segment)
}
